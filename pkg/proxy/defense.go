package proxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// 限速相关常量
const (
	// 默认限流周期（秒）
	DefaultRateLimitWindow = 60
	// 默认周期内允许的最大请求数
	DefaultMaxRequestsPerWindow = 30
	// IP黑名单超时时间（分钟）
	DefaultBlacklistTimeout = 30
	// 默认日志抑制时间（秒）
	DefaultLogSuppressionWindow = 60
	// 相同IP错误日志数量阈值
	DefaultLogSuppressionThreshold = 5
)

// 添加已知攻击者的域名
var knownAttackerDomains = []string{
	"hksjz.net",             // 已知攻击者域名
	"btp3.app",              // 已知攻击者域名
	"btbtptptpie.crxo5.com", // 新发现的攻击者域名
	"crxo5.com",             // 攻击者域名的主域名
}

// 防御系统接口
type DefenseSystem interface {
	// 检查请求是否允许通过，返回是否通过和拒绝原因（如有）
	CheckRequest(clientIP, hostname string) (bool, string)
	// 安全关闭防御系统
	Shutdown() error
}

// 默认防御系统实现
type DefaultDefenseSystem struct {
	// 速率限制器
	rateLimiter *RateLimiter
	// 是否启用速率限制
	enableRateLimit bool

	// 域名黑名单
	domainBlocker *DomainBlocker
	// 是否启用域名黑名单
	enableDomainBlock bool

	// 日志抑制器（减少重复日志）
	logSuppressor *LogSuppressor
	// 是否启用日志抑制
	enableLogSuppression bool

	// 日志记录器
	logger *log.Logger

	// 清理任务的上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc
}

// LogSuppressor 用于减少重复日志
type LogSuppressor struct {
	// 存储IP最后一次记录日志的时间
	lastLogTime map[string]time.Time
	// 存储IP在窗口内的错误日志数量
	errorCount map[string]int
	// 互斥锁保护并发访问
	mu sync.Mutex
	// 日志抑制时间窗口（秒）
	window int
	// 触发抑制的阈值
	threshold int
}

// NewLogSuppressor 创建一个新的日志抑制器
func NewLogSuppressor(window, threshold int) *LogSuppressor {
	if window <= 0 {
		window = DefaultLogSuppressionWindow
	}
	if threshold <= 0 {
		threshold = DefaultLogSuppressionThreshold
	}

	return &LogSuppressor{
		lastLogTime: make(map[string]time.Time),
		errorCount:  make(map[string]int),
		window:      window,
		threshold:   threshold,
	}
}

// ShouldLog 判断是否应该记录日志
func (ls *LogSuppressor) ShouldLog(ip string, isError bool) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	now := time.Now()

	// 如果不是错误日志，始终记录
	if !isError {
		ls.lastLogTime[ip] = now
		return true
	}

	// 对于错误日志，检查频率
	lastTime, exists := ls.lastLogTime[ip]

	// 如果IP之前没有日志，或者已经过了抑制窗口，重置计数
	if !exists || now.Sub(lastTime).Seconds() > float64(ls.window) {
		ls.errorCount[ip] = 1
		ls.lastLogTime[ip] = now
		return true
	}

	// 增加错误计数
	ls.errorCount[ip]++

	// 如果超过阈值，并且在上次记录后不久，则抑制日志
	if ls.errorCount[ip] > ls.threshold {
		// 每隔一段时间仍然记录一次，用于监控
		if now.Sub(lastTime).Seconds() > float64(ls.window/4) {
			ls.lastLogTime[ip] = now
			return true
		}
		return false
	}

	// 未超过阈值，记录日志
	ls.lastLogTime[ip] = now
	return true
}

// ClearExpiredRecords 清理过期的记录
func (ls *LogSuppressor) ClearExpiredRecords() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	now := time.Now()

	// 清理过期的记录
	for ip, lastTime := range ls.lastLogTime {
		if now.Sub(lastTime).Seconds() > float64(ls.window*2) {
			delete(ls.lastLogTime, ip)
			delete(ls.errorCount, ip)
		}
	}
}

// 创建新的防御系统
func NewDefenseSystem(ctx context.Context, logger *log.Logger) *DefaultDefenseSystem {
	// 创建上下文和取消函数用于清理任务
	ctx, cancel := context.WithCancel(ctx)

	// 创建限速器，默认每分钟30个请求，超过后封禁30分钟
	rateLimiter := NewRateLimiter(DefaultRateLimitWindow, DefaultMaxRequestsPerWindow, DefaultBlacklistTimeout)

	// 启动清理任务
	go rateLimiter.StartCleanupTask(ctx)

	// 创建日志抑制器
	logSuppressor := NewLogSuppressor(DefaultLogSuppressionWindow, DefaultLogSuppressionThreshold)

	// 启动定期清理任务
	go func() {
		ticker := time.NewTicker(time.Duration(DefaultLogSuppressionWindow) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				logSuppressor.ClearExpiredRecords()
			case <-ctx.Done():
				return
			}
		}
	}()

	// 创建域名黑名单，预设部分恶意域名
	domainBlocker := NewDomainBlocker(knownAttackerDomains)

	if logger != nil {
		logger.Printf("已预设 %d 个域名黑名单", len(knownAttackerDomains))
	}

	return &DefaultDefenseSystem{
		rateLimiter:          rateLimiter,
		enableRateLimit:      true, // 默认启用限速
		domainBlocker:        domainBlocker,
		enableDomainBlock:    true, // 默认启用域名黑名单
		logSuppressor:        logSuppressor,
		enableLogSuppression: true, // 默认启用日志抑制
		logger:               logger,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// 检查请求是否允许通过
func (d *DefaultDefenseSystem) CheckRequest(clientIP, hostname string) (bool, string) {
	// 检查域名黑名单
	if d.enableDomainBlock && hostname != "" && d.domainBlocker.IsBlocked(hostname) {
		if d.logger != nil && d.shouldLog(clientIP, true) {
			d.logger.Printf("拒绝访问黑名单域名: %s 来自 %s", hostname, clientIP)
		}
		return false, fmt.Sprintf("域名 %s 已被禁止访问", hostname)
	}

	// 检查速率限制
	if d.enableRateLimit {
		allowed, message := d.rateLimiter.CheckAndUpdateLimit(clientIP)
		if !allowed {
			if d.logger != nil && d.shouldLog(clientIP, true) {
				d.logger.Printf("拒绝来自 %s 的请求: %s", clientIP, message)
			}
			return false, message
		}
	}

	return true, ""
}

// shouldLog 判断是否应该记录日志
func (d *DefaultDefenseSystem) shouldLog(ip string, isError bool) bool {
	if !d.enableLogSuppression {
		return true
	}
	return d.logSuppressor.ShouldLog(ip, isError)
}

// 启用或禁用日志抑制
func (d *DefaultDefenseSystem) EnableLogSuppression(enable bool) {
	d.enableLogSuppression = enable
	if d.logger != nil {
		d.logger.Printf("日志抑制功能已%s", ifThenElse(enable, "启用", "禁用"))
	}
}

// 设置日志抑制配置
func (d *DefaultDefenseSystem) SetLogSuppressionConfig(window, threshold int) {
	d.logSuppressor = NewLogSuppressor(window, threshold)
	if d.logger != nil {
		d.logger.Printf("已更新日志抑制配置: 窗口=%d秒, 阈值=%d次", window, threshold)
	}
}

// 安全关闭防御系统
func (d *DefaultDefenseSystem) Shutdown() error {
	if d.cancel != nil {
		d.cancel()
	}
	if d.logger != nil {
		d.logger.Println("防御系统已安全关闭")
	}
	return nil
}

// 启用或禁用速率限制
func (d *DefaultDefenseSystem) EnableRateLimit(enable bool) {
	d.enableRateLimit = enable
	if d.logger != nil {
		d.logger.Printf("速率限制功能已%s", ifThenElse(enable, "启用", "禁用"))
	}
}

// 设置速率限制配置
func (d *DefaultDefenseSystem) SetRateLimitConfig(window, maxRequests, blacklistTimeout int) {
	d.rateLimiter = NewRateLimiter(window, maxRequests, blacklistTimeout)

	// 重新启动清理任务
	if d.cancel != nil {
		d.cancel()
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	go d.rateLimiter.StartCleanupTask(d.ctx)

	if d.logger != nil {
		d.logger.Printf("已更新速率限制配置: 窗口=%d秒, 最大请求=%d, 黑名单超时=%d分钟",
			window, maxRequests, blacklistTimeout)
	}
}

// 启用或禁用域名黑名单
func (d *DefaultDefenseSystem) EnableDomainBlock(enable bool) {
	d.enableDomainBlock = enable
	if d.logger != nil {
		d.logger.Printf("域名黑名单功能已%s", ifThenElse(enable, "启用", "禁用"))
	}
}

// 将域名添加到黑名单
func (d *DefaultDefenseSystem) BlockDomain(domain string) {
	d.domainBlocker.BlockDomain(domain)
	if d.logger != nil {
		d.logger.Printf("已将域名 %s 添加到黑名单", domain)
	}
}

// 将域名从黑名单中移除
func (d *DefaultDefenseSystem) UnblockDomain(domain string) {
	d.domainBlocker.UnblockDomain(domain)
	if d.logger != nil {
		d.logger.Printf("已将域名 %s 从黑名单中移除", domain)
	}
}

// 获取黑名单域名列表
func (d *DefaultDefenseSystem) ListBlockedDomains() []string {
	return d.domainBlocker.ListBlockedDomains()
}

// 打印防御系统状态
func (d *DefaultDefenseSystem) PrintStatus() {
	if d.logger == nil {
		return
	}

	if d.enableRateLimit {
		d.logger.Printf("已启用速率限制: 每%d秒最多%d个请求, 超限封禁%d分钟",
			d.rateLimiter.window, d.rateLimiter.maxRequests, d.rateLimiter.blacklistTimeout)
	} else {
		d.logger.Println("速率限制: 已禁用")
	}

	if d.enableDomainBlock {
		blockedDomains := d.domainBlocker.ListBlockedDomains()
		d.logger.Printf("已启用域名黑名单，当前包含 %d 个域名", len(blockedDomains))
		if len(blockedDomains) > 0 {
			d.logger.Printf("黑名单域名: %s", strings.Join(blockedDomains, ", "))
		}
	} else {
		d.logger.Println("域名黑名单: 已禁用")
	}
}

// RateLimiter 实现IP限速和黑名单功能
type RateLimiter struct {
	// 存储每个IP的请求计数和时间戳
	ipRequests map[string]*IPRequestInfo
	// 存储黑名单IP及解除时间
	blacklist map[string]time.Time
	// 互斥锁保护并发访问
	mu sync.Mutex
	// 时间窗口（秒）
	window int
	// 窗口内最大请求数
	maxRequests int
	// 黑名单超时时间（分钟）
	blacklistTimeout int
}

// IPRequestInfo 存储IP请求信息
type IPRequestInfo struct {
	// 当前窗口内的请求数
	Count int
	// 窗口开始时间
	WindowStart time.Time
}

// NewRateLimiter 创建一个新的限速器
func NewRateLimiter(window, maxRequests, blacklistTimeout int) *RateLimiter {
	if window <= 0 {
		window = DefaultRateLimitWindow
	}
	if maxRequests <= 0 {
		maxRequests = DefaultMaxRequestsPerWindow
	}
	if blacklistTimeout <= 0 {
		blacklistTimeout = DefaultBlacklistTimeout
	}

	return &RateLimiter{
		ipRequests:       make(map[string]*IPRequestInfo),
		blacklist:        make(map[string]time.Time),
		window:           window,
		maxRequests:      maxRequests,
		blacklistTimeout: blacklistTimeout,
	}
}

// CheckAndUpdateLimit 检查IP是否可以请求并更新计数
// 返回是否允许请求和可选的错误消息
func (rl *RateLimiter) CheckAndUpdateLimit(ipAddr string) (bool, string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 检查IP是否在黑名单中
	if banUntil, banned := rl.blacklist[ipAddr]; banned {
		if now.Before(banUntil) {
			// 计算剩余封禁时间
			remaining := banUntil.Sub(now).Round(time.Minute)
			return false, fmt.Sprintf("IP已被临时封禁，剩余时间：约%d分钟", int(remaining.Minutes()))
		}
		// 已过封禁期，从黑名单移除
		delete(rl.blacklist, ipAddr)
	}

	// 获取IP的请求信息，如不存在则创建
	info, exists := rl.ipRequests[ipAddr]
	if !exists || now.Sub(info.WindowStart).Seconds() > float64(rl.window) {
		// 创建新窗口
		rl.ipRequests[ipAddr] = &IPRequestInfo{
			Count:       1,
			WindowStart: now,
		}
		return true, ""
	}

	// 增加计数
	info.Count++

	// 检查是否超过限制
	if info.Count > rl.maxRequests {
		// 将IP加入黑名单
		rl.blacklist[ipAddr] = now.Add(time.Duration(rl.blacklistTimeout) * time.Minute)
		return false, fmt.Sprintf("请求频率过高，IP已被临时封禁%d分钟", rl.blacklistTimeout)
	}

	return true, ""
}

// ClearExpiredRecords 清理过期的记录
func (rl *RateLimiter) ClearExpiredRecords() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 清理过期的IP请求记录
	for ip, info := range rl.ipRequests {
		if now.Sub(info.WindowStart).Seconds() > float64(rl.window) {
			delete(rl.ipRequests, ip)
		}
	}

	// 清理过期的黑名单
	for ip, until := range rl.blacklist {
		if now.After(until) {
			delete(rl.blacklist, ip)
		}
	}
}

// StartCleanupTask 启动定期清理任务
func (rl *RateLimiter) StartCleanupTask(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(rl.window) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.ClearExpiredRecords()
		case <-ctx.Done():
			return
		}
	}
}

// DomainBlocker 实现域名黑名单功能
type DomainBlocker struct {
	// 存储被禁止的域名
	blockedDomains map[string]bool
	// 互斥锁保护并发访问
	mu sync.Mutex
}

// NewDomainBlocker 创建一个新的域名黑名单
func NewDomainBlocker(initialBlacklist []string) *DomainBlocker {
	blocker := &DomainBlocker{
		blockedDomains: make(map[string]bool),
	}

	// 添加初始黑名单
	for _, domain := range initialBlacklist {
		if domain != "" {
			blocker.blockedDomains[strings.ToLower(domain)] = true
		}
	}

	return blocker
}

// IsBlocked 检查域名是否在黑名单中
func (db *DomainBlocker) IsBlocked(domain string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 转换为小写进行比较
	return db.blockedDomains[strings.ToLower(domain)]
}

// BlockDomain 将域名添加到黑名单
func (db *DomainBlocker) BlockDomain(domain string) {
	if domain == "" {
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// 转换为小写存储
	db.blockedDomains[strings.ToLower(domain)] = true
}

// UnblockDomain 将域名从黑名单中移除
func (db *DomainBlocker) UnblockDomain(domain string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 转换为小写后删除
	delete(db.blockedDomains, strings.ToLower(domain))
}

// ListBlockedDomains 列出所有黑名单域名
func (db *DomainBlocker) ListBlockedDomains() []string {
	db.mu.Lock()
	defer db.mu.Unlock()

	domains := make([]string, 0, len(db.blockedDomains))
	for domain := range db.blockedDomains {
		domains = append(domains, domain)
	}
	return domains
}

// 获取客户端真实IP地址
func GetClientIP(r interface{}) string {
	// 检查是否是http.Request类型
	if req, ok := r.(interface {
		Header(string) string
		RemoteAddr() string
	}); ok {
		// 尝试从各种可能的Header获取真实IP
		if ip := req.Header("X-Real-IP"); ip != "" {
			return ip
		}
		if ip := req.Header("X-Forwarded-For"); ip != "" {
			// X-Forwarded-For可能包含多个IP，取第一个
			ips := strings.Split(ip, ",")
			if len(ips) > 0 && ips[0] != "" {
				return strings.TrimSpace(ips[0])
			}
		}

		// 从RemoteAddr获取IP
		remoteAddr := req.RemoteAddr()
		ip, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			// 如果解析失败，直接返回原始地址
			return remoteAddr
		}
		return ip
	}

	// 如果不是期望的类型，返回空字符串
	return ""
}

// ifThenElse 是一个简单的三元操作符模拟
func ifThenElse(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}
