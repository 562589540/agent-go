package proxy

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 全局 CA 操作锁
var caLock sync.Mutex

// 加载或生成的 CA 证书的内存存储
var caCert *tls.Certificate

// CA 文件常量
// CA 证书文件
const caCertFile = "goproxy_ca_cert.pem"

// CA 私钥文件
const caKeyFile = "goproxy_ca_key.pem"

// CA 存储目录（当前工作目录的子文件夹）
const caDir = ".goproxy-ca"

// ProxyServer 是一个具备 MITM 功能的 HTTP 代理服务器
type ProxyServer struct {
	listenAddr       string
	apiKeys          []string
	upstreamProxyURL string
	logger           *log.Logger
	server           *http.Server
	httpClient       *http.Client
	authenticator    Authenticator

	// 用于 API 密钥轮换
	keyIndex int
	keyMutex sync.Mutex
}

// NewProxy 创建一个新的 HTTP 代理服务器实例
func NewProxy(listenAddr string, apiKeys []string, upstreamProxyURL string, authenticator Authenticator) (*ProxyServer, error) {
	logger := log.New(log.Writer(), "[Proxy] ", log.LstdFlags) // 使用 [Proxy] 作为日志前缀

	// 检查 API 密钥池是否为空
	if len(apiKeys) == 0 {
		return nil, errors.New("API 密钥池不能为空")
	}
	logger.Printf("已加载 %d 个 API 密钥到池中", len(apiKeys))

	// 确保 authenticator 不为 nil，如果为 nil，则使用 NilAuthenticator
	if authenticator == nil {
		logger.Println("警告: 未提供认证器，将允许所有请求。")
		authenticator = &NilAuthenticator{}
	}

	// 加载或生成 CA 证书
	_, err := loadOrGenerateCA(logger) // 结果存储在全局变量 caCert 中
	if err != nil {
		return nil, fmt.Errorf("无法加载或生成 CA 证书: %w", err)
	}

	// 配置 http Client 以支持上游代理
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	// 如果配置了上游代理, 设置 Transport 的 Proxy 函数
	if upstreamProxyURL != "" {
		proxyURL, err := url.Parse(upstreamProxyURL)
		if err != nil {
			logger.Printf("警告: 解析上游代理URL失败 '%s': %v, 将直接连接", upstreamProxyURL, err)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
			logger.Printf("已配置 HTTP Client 使用上游代理: %s", upstreamProxyURL)
		}
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Minute,
	}

	return &ProxyServer{
		listenAddr:       listenAddr,
		apiKeys:          apiKeys,
		upstreamProxyURL: upstreamProxyURL,
		logger:           logger,
		httpClient:       httpClient,
		authenticator:    authenticator,
		keyIndex:         0,
	}, nil
}

// getNextAPIKey 以轮询方式获取下一个 API 密钥 (线程安全)
func (p *ProxyServer) getNextAPIKey() string {
	p.keyMutex.Lock()
	defer p.keyMutex.Unlock()

	key := p.apiKeys[p.keyIndex]
	p.keyIndex = (p.keyIndex + 1) % len(p.apiKeys) // 循环索引
	p.logger.Printf("使用 API 密钥池中的密钥 (索引 %d): %s", (p.keyIndex-1+len(p.apiKeys))%len(p.apiKeys), maskKey(key))
	return key
}

// Start 启动代理服务器
func (p *ProxyServer) Start() error {
	p.logger.Printf("启动 HTTP 代理服务器在 %s", p.listenAddr) // 更新日志消息
	if p.upstreamProxyURL != "" {
		p.logger.Printf("使用上游代理: %s", p.upstreamProxyURL)
	}

	// 创建HTTP服务器
	p.server = &http.Server{
		Addr:         p.listenAddr,
		Handler:      http.HandlerFunc(p.handleRequest),
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 1 * time.Minute,
	}

	return p.server.ListenAndServe()
}

// Stop 停止代理服务器
func (p *ProxyServer) Stop() error {
	p.logger.Println("停止代理服务器...")
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

// handleRequest 处理所有代理请求
func (p *ProxyServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	p.logger.Printf("收到请求: %s %s from %s", r.Method, r.URL, r.RemoteAddr)
	//不在这里认证
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

// handleConnect 处理HTTPS隧道 (CONNECT请求)
func (p *ProxyServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	// 目标主机，例如 "generativelanguage.googleapis.com:443"
	targetHost := r.Host
	hostname, _, err := net.SplitHostPort(targetHost)
	if err != nil {
		// 如果分割失败，假定没有端口
		hostname = targetHost
	}
	p.logger.Printf("处理CONNECT请求: %s (主机名: %s)", targetHost, hostname)
	p.logger.Printf("请求头: %v", r.Header)

	// === Google API 的 MITM (中间人攻击) 逻辑 ===
	if hostname == "generativelanguage.googleapis.com" {
		p.logger.Printf("检测到 Google API 请求，尝试 MITM 拦截...")

		// 1. 在发送 200 OK 之前劫持客户端连接
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			p.logger.Printf("错误: 不支持连接劫持(Hijacking)")
			http.Error(w, "不支持连接劫持(Hijacking)", http.StatusInternalServerError)
			return
		}
		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			p.logger.Printf("劫持连接失败: %v", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		// 此处不关闭 clientConn，它将由 MITM 的 goroutine 管理

		// 2. 为目标主机生成证书
		hostCert, err := signHost(hostname)
		if err != nil {
			p.logger.Printf("为 '%s' 生成证书失败: %v", hostname, err)
			http.Error(w, "生成服务器证书失败", http.StatusInternalServerError)
			clientConn.Close()
			return
		}

		// 3. *现在* 向客户端发送 200 OK，然后开始 TLS 握手
		// 必须在开始 TLS 握手之前写入 200 OK 响应
		_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		if err != nil {
			p.logger.Printf("向客户端发送 200 OK 失败: %v", err)
			clientConn.Close()
			return
		}
		p.logger.Printf("已向客户端发送 200 OK (用于 MITM)")

		// 4. 与客户端执行 TLS 握手 (假装是 Google)
		tlsConfigServer := &tls.Config{
			Certificates: []tls.Certificate{*hostCert},
			MinVersion:   tls.VersionTLS12,
		}
		clientTlsConn := tls.Server(clientConn, tlsConfigServer)
		if err := clientTlsConn.Handshake(); err != nil {
			p.logger.Printf("与客户端 TLS 握手失败: %v", err)
			clientTlsConn.Close() // 这也会关闭底层的 clientConn
			return
		}
		defer clientTlsConn.Close()
		p.logger.Printf("与客户端 TLS 握手成功")

		// 5. 连接到实际的目标服务器 (如果配置了上游代理，则通过上游代理连接)
		var destConn net.Conn
		if p.upstreamProxyURL != "" {
			p.logger.Printf("通过上游代理 %s 连接到目标: %s", p.upstreamProxyURL, targetHost)
			dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
			destConn, err = dialViaProxy(r.Context(), dialer, p.upstreamProxyURL, targetHost) // 使用 dialViaProxy 替代 dialViaProxySimple
		} else {
			p.logger.Printf("直接连接到目标: %s", targetHost)
			destConn, err = net.DialTimeout("tcp", targetHost, 10*time.Second)
		}
		if err != nil {
			p.logger.Printf("连接真实目标服务器失败: %v", err)
			// 通知客户端？也许直接关闭。
			return // clientTlsConn 的 defer 会关闭客户端侧
		}
		// 此处不关闭 destConn，它将由下面的 TLS 连接管理

		// 6. 与目标服务器执行 TLS 握手
		// 如果我们信任通过上游代理的网络路径，可能不需要严格验证 Google 的证书
		tlsConfigClient := &tls.Config{
			ServerName: hostname, // 对 SNI 很重要
			MinVersion: tls.VersionTLS12,
			// InsecureSkipVerify: true, // 谨慎使用，跳过验证 Google 证书
		}
		serverTlsConn := tls.Client(destConn, tlsConfigClient)
		if err := serverTlsConn.Handshake(); err != nil {
			p.logger.Printf("与目标服务器 TLS 握手失败: %v", err)
			serverTlsConn.Close() // 关闭底层的 destConn
			return                // clientTlsConn 的 defer 会关闭客户端侧
		}
		defer serverTlsConn.Close()
		p.logger.Printf("与目标服务器 TLS 握手成功")

		// 7. 开始 MITM 代理 (读取客户端请求，修改，发送到服务器等)
		p.logger.Printf("开始 MITM 代理循环: %s <-> %s", clientTlsConn.RemoteAddr(), targetHost)
		p.mitmProxyLoop(clientTlsConn, serverTlsConn, targetHost) // 使用新函数处理
		p.logger.Printf("MITM 代理循环结束 for %s", targetHost)
		return // MITM 处理结束
	}

	// === 标准 CONNECT 隧道 (非 Google API) ===
	p.logger.Printf("非 Google API 请求，执行标准 CONNECT 隧道")
	var destConn net.Conn // 为此作用域重新声明
	if p.upstreamProxyURL != "" {
		p.logger.Printf("通过上游代理 %s 连接到目标: %s", p.upstreamProxyURL, targetHost)
		dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
		destConn, err = dialViaProxy(r.Context(), dialer, p.upstreamProxyURL, targetHost) // 使用 dialViaProxy 替代 dialViaProxySimple
	} else {
		p.logger.Printf("直接连接到目标: %s", targetHost)
		destConn, err = net.DialTimeout("tcp", targetHost, 10*time.Second)
	}
	if err != nil {
		p.logger.Printf("连接失败: %v", err)
		if strings.Contains(err.Error(), "refused") {
			http.Error(w, "目标连接被拒绝", http.StatusBadGateway)
		} else if strings.Contains(err.Error(), "timeout") {
			http.Error(w, "连接目标超时", http.StatusGatewayTimeout)
		} else {
			http.Error(w, "无法连接到目标服务器: "+err.Error(), http.StatusServiceUnavailable)
		}
		return
	}
	defer destConn.Close()
	p.logger.Printf("成功连接到: %s (通过%s)", r.Host, ifelse(p.upstreamProxyURL != "", "上游代理", "直接连接"))

	// 响应客户端，表示连接已建立
	w.WriteHeader(http.StatusOK)
	p.logger.Printf("返回 200 OK 给客户端 (标准隧道)")

	// 劫持连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		p.logger.Printf("错误: 不支持连接劫持(Hijacking)")
		http.Error(w, "不支持连接劫持(Hijacking)", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		p.logger.Printf("劫持连接失败: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()
	p.logger.Printf("成功劫持连接 (标准隧道)")

	// 双向复制数据
	p.logger.Printf("开始双向转发数据 (标准隧道): %s <-> %s", r.RemoteAddr, r.Host)

	// 客户端 -> 目标
	go func() {
		n, err := io.Copy(destConn, clientConn)
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") && err != io.EOF {
			p.logger.Printf("客户端->目标 复制错误 (标准隧道): %v", err)
		}
		p.logger.Printf("客户端->目标 复制完成 (标准隧道), 传输 %d 字节", n)
		// 关闭一个连接以通知另一个 goroutine 结束
		clientConn.Close()
		destConn.Close()
	}()

	// 目标 -> 客户端
	n, err := io.Copy(clientConn, destConn)
	if err != nil && !strings.Contains(err.Error(), "use of closed network connection") && err != io.EOF {
		p.logger.Printf("目标->客户端 复制错误 (标准隧道): %v", err)
	}
	p.logger.Printf("目标->客户端 复制完成 (标准隧道), 传输 %d 字节", n)

	clientConn.Close()
	destConn.Close()
	p.logger.Printf("CONNECT会话结束 (标准隧道): %s", r.Host)
}

// singleConnListener 是一个只接受单个连接的 net.Listener
type singleConnListener struct {
	conn net.Conn
	once sync.Once
	// 用于持有连接的缓冲通道
	ch chan net.Conn
}

// newSingleConnListener 为单个连接创建一个 listener
func newSingleConnListener(conn net.Conn) net.Listener {
	l := &singleConnListener{
		conn: conn,
		// 缓冲区大小为 1
		ch: make(chan net.Conn, 1),
	}
	// 立即将连接放入通道
	l.ch <- conn
	return l
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	// 尝试从通道接收连接
	c, ok := <-l.ch
	if !ok {
		// 通道已关闭，表示连接已被接受或 listener 已关闭
		// 或其他更具体的错误，如 net.ErrClosed
		return nil, io.EOF
	}
	// 返回连接
	return c, nil
}

func (l *singleConnListener) Close() error {
	// 关闭通道以表示不再有连接
	l.once.Do(func() {
		close(l.ch)
	})
	// 同时关闭底层连接
	return l.conn.Close()
}

func (l *singleConnListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

// mitmProxyLoop 使用 http.Serve 处理 MITM 连接的数据传输
func (p *ProxyServer) mitmProxyLoop(clientTlsConn net.Conn, serverTlsConn net.Conn, targetHost string) {
	p.logger.Printf("开始 MITM 代理循环 (http.Serve): %s <-> %s", clientTlsConn.RemoteAddr(), targetHost)

	// 创建一个 ReverseProxy 实例
	dummyTargetUrl := &url.URL{Scheme: "https", Host: targetHost}
	reverseProxy := httputil.NewSingleHostReverseProxy(dummyTargetUrl)

	// 自定义 transport，用于向现有的服务器 TLS 连接写入/读取数据
	reverseProxy.Transport = &mitmTransport{
		Conn:   serverTlsConn,
		Logger: p.logger,
	}

	// 自定义 Director 用于在发送请求前修改请求
	originalDirector := reverseProxy.Director
	reverseProxy.Director = func(req *http.Request) {
		// 设置基本字段
		originalDirector(req)
		p.logger.Printf("MITM Director: 正在处理请求 %s", req.URL.String())

		// 在替换 API Key 之前进行认证
		isValid, err := p.authenticator.Authenticate(req)

		authStatusCode := http.StatusOK // 默认成功
		authMessage := ""

		if err != nil {
			p.logger.Printf("认证过程发生错误: %v", err)
			var authErr *AuthServerError
			if errors.As(err, &authErr) {
				// 是主认证服务器返回的错误
				authStatusCode = authErr.StatusCode
				authMessage = authErr.Message
			} else {
				// 是其他内部错误
				authStatusCode = http.StatusInternalServerError
				authMessage = "代理认证内部错误: " + err.Error()
			}
			// 设置 Header 标记错误并携带信息
			req.Header.Set(HeaderAuthResultCode, strconv.Itoa(authStatusCode))
			req.Header.Set(HeaderAuthResultMessage, authMessage)
			return // 认证出错，直接返回，不继续处理
		}

		if !isValid {
			p.logger.Printf("认证失败 (密钥无效或缺失): %s %s", req.Method, req.URL)
			authStatusCode = http.StatusUnauthorized
			authMessage = "认证失败"
			// 设置 Header 标记认证失败
			req.Header.Set(HeaderAuthResultCode, strconv.Itoa(authStatusCode))
			req.Header.Set(HeaderAuthResultMessage, authMessage)
			return // 认证失败，直接返回，不继续处理
		}

		// 认证成功，继续执行
		p.logger.Printf("MITM Director: 认证成功")

		// --- 修改 API Key ---
		originalAPIKey := req.Header.Get("x-goog-api-key")
		nextKey := p.getNextAPIKey() // 获取下一个轮换密钥
		if originalAPIKey != "" {
			req.Header.Set("x-goog-api-key", nextKey)
			p.logger.Printf("MITM Director: 已替换 API Key Header: %s -> %s", maskKey(originalAPIKey), maskKey(nextKey))
		} else {
			q := req.URL.Query()
			urlKey := q.Get("key")
			if urlKey != "" {
				q.Set("key", nextKey)
				req.URL.RawQuery = q.Encode()
				p.logger.Printf("MITM Director: 已替换 URL API Key: %s -> %s", maskKey(urlKey), maskKey(nextKey))
			} else {
				req.Header.Set("x-goog-api-key", nextKey)
				p.logger.Printf("MITM Director: 添加了 API Key Header: %s", maskKey(nextKey))
			}
		}
		// --- 结束 API Key 修改 ---

		req.Header.Del("Proxy-Connection")
		req.Header.Del("Proxy-Authorization")
		req.Host = targetHost
		p.logger.Printf("MITM Director: 最终路径: %s, 主机: %s", req.URL.Path, req.Host)
	}

	// 自定义 ModifyResponse (可选, 用于日志记录)
	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		p.logger.Printf("MITM ModifyResponse: 从目标收到状态 %d", resp.StatusCode)
		return nil
	}

	// 自定义 ErrorHandler
	reverseProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		p.logger.Printf("MITM ReverseProxy ErrorHandler: 错误: %v", err)
		// 检查 header 是否已发送或连接是否已劫持
		if !responseWriterWrittenOrHijacked(rw) {
			http.Error(rw, fmt.Sprintf("MITM 代理错误: %v", err), http.StatusBadGateway)
		} else {
			p.logger.Printf("MITM ErrorHandler: Header 已发送或连接已劫持，无法向客户端写入错误。")
		}
	}

	// 禁用流式响应（如 SSE）的缓冲
	reverseProxy.FlushInterval = -1
	p.logger.Printf("MITM: 为 ReverseProxy 设置 FlushInterval = -1")

	// 创建一个使用 reverse proxy 作为 handler 的 HTTP 服务器
	server := &http.Server{
		// 直接使用 RP 作为 handler
		Handler: reverseProxy,
	}

	// 创建一个只服务于单个客户端 TLS 连接的 listener
	listener := newSingleConnListener(clientTlsConn)

	// 在单连接 listener 上处理 HTTP 请求
	// 此调用将阻塞，直到 listener 返回错误 (例如，在单个连接关闭后)
	p.logger.Printf("MITM: 在单连接 listener 上为 %s 调用 http.Serve", clientTlsConn.RemoteAddr())
	err := server.Serve(listener)

	if err != nil && err != io.EOF && !strings.Contains(err.Error(), "closed") && err != http.ErrServerClosed {
		// 记录意外错误，忽略 EOF/closed (这些在单连接结束后是预期的)
		p.logger.Printf("MITM: 单连接 listener 上的 http.Serve 错误: %v", err)
	} else {
		p.logger.Printf("MITM: 为 %s 干净地完成 http.Serve", clientTlsConn.RemoteAddr())
	}

	p.logger.Printf("MITM: 退出 mitmProxyLoop (http.Serve) for %s", clientTlsConn.RemoteAddr())
	// 连接 (clientTlsConn, serverTlsConn) 应由 handleConnect 中的 defer 关闭
}

// 辅助函数，检查 ResponseWriter 是否已写入 header 或已被劫持。
// 这并非万无一失，但涵盖了常见情况。
func responseWriterWrittenOrHijacked(rw http.ResponseWriter) bool {
	// 如果可用，使用接口断言检查 (Go 1.20+？请查阅文档)
	// 或检查常见实现，如 *http.response
	if r, ok := rw.(interface{ Written() bool }); ok {
		return r.Written()
	}
	// 检查是否被劫持
	if _, ok := rw.(http.Hijacker); ok {
		// 这并不能保证它 *被* 劫持了，只是说明它 *可以* 被劫持。
		// 更好的检查可能涉及尝试写入 header 并捕获 panic
		// 或者在绝对必要时检查内部字段 (不推荐)。
		// 现在，我们假设如果它可以被劫持，它可能已经被劫持了。
		// 更简单的检查可能是查看 ErrorHandler 中的特定错误
		// if err == http.ErrHijacked { return true }
	}
	// 如果特定检查失败，则回退：假设未写入/未劫持
	return false
}

// mitmTransport 是一个自定义的 http.RoundTripper，它向现有的连接写入数据
type mitmTransport struct {
	Conn   net.Conn
	Logger *log.Logger
}

func (t *mitmTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 检查 Director 中设置的认证结果 Header
	authCodeStr := req.Header.Get(HeaderAuthResultCode)
	authMsg := req.Header.Get(HeaderAuthResultMessage)

	if authCodeStr != "" {
		// 认证在 Director 中已经失败或出错
		// 移除标记头，避免发送给目标服务器
		req.Header.Del(HeaderAuthResultCode)
		req.Header.Del(HeaderAuthResultMessage)

		statusCode, err := strconv.Atoi(authCodeStr)
		if err != nil {
			// 如果状态码解析失败，则默认为 500
			t.Logger.Printf("mitmTransport: 无法解析认证结果状态码 '%s', 使用 500", authCodeStr)
			statusCode = http.StatusInternalServerError
		}
		if authMsg == "" {
			authMsg = http.StatusText(statusCode) // 如果消息为空，使用标准状态文本
			if authMsg == "" {
				authMsg = "认证时发生未知错误" // 最终回退
			}
		}

		t.Logger.Printf("mitmTransport: 检测到认证结果: 状态码 %d, 消息: %s", statusCode, authMsg)
		// 返回包含原始状态码和消息的响应
		return &http.Response{
			StatusCode: statusCode,
			Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)), // 例如 "404 Not Found"
			Proto:      req.Proto,
			ProtoMajor: req.ProtoMajor,
			ProtoMinor: req.ProtoMinor,
			Header:     http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}}, // 设置响应头
			Body:       io.NopCloser(strings.NewReader(authMsg)),                           // 返回具体的错误信息
			Request:    req,
		}, nil // 返回响应，而不是错误，让 ReverseProxy 处理
	}

	// 认证已在 Director 处理且成功，这里正常继续
	// 记录发送到实际服务器的请求
	t.Logger.Printf("mitmTransport: 正在向目标发送请求: %s %s", req.Method, req.URL)

	// === 启用请求 Dump ===
	reqDump, errDump := httputil.DumpRequestOut(req, true)
	if errDump != nil {
		t.Logger.Printf("mitmTransport: Dump 输出请求错误: %v", errDump)
	} else {
		t.Logger.Printf("mitmTransport: 输出请求 Dump:\n%s", string(reqDump))
	}
	// === 结束请求 Dump ===

	// 将请求写入现有的服务器连接
	if err := req.Write(t.Conn); err != nil {
		t.Conn.Close()
		return nil, fmt.Errorf("mitmTransport: 写入请求失败: %w", err)
	}
	t.Logger.Printf("mitmTransport: 请求已写入目标连接。")

	// 从同一连接读取响应
	resp, err := http.ReadResponse(bufio.NewReader(t.Conn), req)
	if err != nil {
		t.Conn.Close()
		return nil, fmt.Errorf("mitmTransport: 读取响应失败: %w", err)
	}
	t.Logger.Printf("mitmTransport: 从目标连接读取响应, 状态: %d", resp.StatusCode)
	return resp, nil
}

// handleHTTP 处理普通的 HTTP 请求 (非 CONNECT)
func (p *ProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	p.logger.Printf("处理HTTP请求: %s %s", r.Method, r.URL)
	p.logger.Printf("目标URL: %s", r.URL.String())

	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""

	outReq.Header.Del("Proxy-Connection")
	outReq.Header.Del("Connection")
	outReq.Header.Del("Keep-Alive")
	outReq.Header.Del("Proxy-Authorization")

	if outReq.URL.Host == "generativelanguage.googleapis.com" {
		nextKey := p.getNextAPIKey() // 获取下一个轮换密钥
		p.logger.Printf("检测到Gemini API请求，使用密钥池中的密钥替换API密钥: %s", maskKey(nextKey))
		outReq.Header.Set("x-goog-api-key", nextKey)
	}

	resp, err := p.httpClient.Do(outReq)
	if err != nil {
		p.logger.Printf("请求失败: %v", err)
		http.Error(w, "转发请求失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	p.logger.Printf("收到目标响应: %d", resp.StatusCode)
	// 复制响应头
	for name, values := range resp.Header {
		// 跳过分块传输编码和内容长度，因为 Go http server 会自动处理
		if name == "Transfer-Encoding" && len(values) == 1 && values[0] == "chunked" {
			continue
		}
		if name == "Content-Length" {
			continue
		}
		if name == "Content-Encoding" {
			// 保留 Content-Encoding (如 gzip)
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// 写入状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		p.logger.Printf("复制响应体错误: %v", err)
	}
	p.logger.Printf("复制响应体完成，传输 %d 字节", n)
	p.logger.Printf("HTTP请求处理完成: %s %s -> %d", r.Method, r.URL.String(), resp.StatusCode)
}

// dialViaProxy 通过上游代理建立到目标地址的 TCP 连接
func dialViaProxy(ctx context.Context, dialer *net.Dialer, proxyURLStr, target string) (net.Conn, error) {
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, fmt.Errorf("无效的上游代理URL: %v", err)
	}

	// 1. 连接到上游代理
	proxyConn, err := dialer.DialContext(ctx, "tcp", proxyURL.Host)
	if err != nil {
		return nil, fmt.Errorf("连接上游代理 '%s' 失败: %v", proxyURL.Host, err)
	}

	// 2. 发送 CONNECT 请求给上游代理
	connectReqStr := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: agent-go-proxy/0.1\r\nProxy-Connection: Keep-Alive\r\n\r\n", target, target)
	if _, err := proxyConn.Write([]byte(connectReqStr)); err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("向上游代理发送CONNECT请求失败: %v", err)
	}

	// 3. 读取上游代理的响应
	br := bufio.NewReader(proxyConn)
	resp, err := http.ReadResponse(br, nil) // CONNECT 请求本身没有请求体，所以 req 为 nil
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("读取上游代理响应失败: %v", err)
	}
	defer resp.Body.Close() // 读取完 body（如果有）后关闭

	// 4. 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		// 读取响应体以获取更多错误信息
		body, _ := io.ReadAll(resp.Body)
		proxyConn.Close()
		return nil, fmt.Errorf("上游代理连接失败: %s, 响应: %s", resp.Status, string(body))
	}

	// 成功，返回已经建立隧道连接的上游代理连接
	return proxyConn, nil
}

// ifelse 是一个简单的三元操作符模拟
func ifelse(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

// maskKey 对 API 密钥进行脱敏处理，只显示前后 4 位
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// StartProxy 是一个便捷函数，用于创建并启动代理服务器
func StartProxy(addr string, apiKeys []string, upstreamProxyURL string, authenticator Authenticator) error {
	proxy, err := NewProxy(addr, apiKeys, upstreamProxyURL, authenticator)
	if err != nil {
		log.Fatalf("创建代理失败: %v", err)
	}
	return proxy.Start()
}

// loadOrGenerateCA 从磁盘加载 CA 证书和密钥，如果找不到则生成新的。
func loadOrGenerateCA(logger *log.Logger) (*tls.Certificate, error) {
	caLock.Lock()
	defer caLock.Unlock()

	// 如果已加载/生成，直接返回内存中的证书
	if caCert != nil {
		return caCert, nil
	}

	certPath := filepath.Join(caDir, caCertFile)
	keyPath := filepath.Join(caDir, caKeyFile)

	// 检查文件是否存在
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			logger.Printf("加载已存在的 CA 证书从: %s 和 %s", certPath, keyPath)
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, fmt.Errorf("加载 CA key pair 失败: %w", err)
			}
			// 验证加载的证书是否可用于签名
			if len(cert.Certificate) == 0 {
				return nil, fmt.Errorf("加载的 CA tls.Certificate 中缺少证书数据")
			}
			_, err = x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				return nil, fmt.Errorf("解析加载的 CA 证书失败: %w", err)
			}
			// 存储在内存中
			caCert = &cert
			return caCert, nil
		}
	}

	logger.Printf("CA 证书未找到, 正在生成新的 CA...")

	// 如果目录不存在则创建
	if err := os.MkdirAll(caDir, 0700); err != nil {
		return nil, fmt.Errorf("创建 CA 目录失败 '%s': %w", caDir, err)
	}

	// 生成私钥
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成 RSA 私钥失败: %w", err)
	}

	// 创建证书模板
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("生成序列号失败: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"GoProxy MITM Root CA"},
			CommonName:   "GoProxy MITM Root CA",
		},
		// 有效期 10 年
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// 创建证书
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("创建 CA 证书失败: %w", err)
	}

	// 将证书写入 PEM 文件
	certOut, err := os.Create(certPath)
	if err != nil {
		return nil, fmt.Errorf("创建 CA 证书文件失败 '%s': %w", certPath, err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()
	logger.Printf("CA 证书已保存到: %s", certPath)

	// 将私钥写入 PEM 文件
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("创建 CA 私钥文件失败 '%s': %w", keyPath, err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()
	logger.Printf("CA 私钥已保存到: %s", keyPath)
	logger.Printf("!!! 重要: 请将 CA 证书 '%s' 添加到您系统的信任存储中以允许 MITM !!!", certPath)

	// 加载生成的密钥对
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("加载生成的 CA key pair 失败: %w", err)
	}
	// 存储在内存中
	caCert = &cert
	return caCert, nil
}

// signHost 使用加载的 CA 为给定的主机生成证书。
func signHost(host string) (*tls.Certificate, error) {
	caLock.Lock()
	// 在调用此函数之前需要确 caCert 已加载
	if caCert == nil {
		caLock.Unlock()
		return nil, fmt.Errorf("CA 证书未加载/初始化")
	}
	// 为安全起见，复制指针的值
	loadedCA := *caCert
	caLock.Unlock()

	// 确保 CA 密钥对包含已解析的证书
	if len(loadedCA.Certificate) == 0 {
		return nil, fmt.Errorf("加载的 CA tls.Certificate 中缺少证书数据")
	}
	x509CACert, err := x509.ParseCertificate(loadedCA.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("解析 CA 证书失败: %w", err)
	}

	// 为主机证书生成私钥
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("为 '%s' 生成 RSA 私钥失败: %w", host, err)
	}

	// 为主机创建证书模板
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("为 '%s' 生成序列号失败: %w", host, err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			// CN 是主机名
			CommonName: host,
		},
		// 有效期开始于稍早的时间
		NotBefore: time.Now().Add(-1 * time.Hour),
		// 有效期 1 年
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// 将主机添加到 SANs (Subject Alternative Names)
		DNSNames: []string{host},
	}

	// 如果主机是 IP 地址，则添加 IP 地址
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	// 使用 CA 签署证书
	// 确密钥类型匹配 (对于 loadedCA 应该是 *rsa.PrivateKey)
	caPrivateKey, ok := loadedCA.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		// 如果需要，可以尝试 EC 密钥，但我们生成的是 RSA
		return nil, fmt.Errorf("CA 私钥不是 RSA 类型")
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, x509CACert, &priv.PublicKey, caPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("为 '%s' 签发证书失败: %w", host, err)
	}

	// 创建最终的 tls.Certificate
	cert := &tls.Certificate{
		// 在证书链中包含 CA 证书
		Certificate: [][]byte{derBytes, loadedCA.Certificate[0]},
		PrivateKey:  priv,
	}

	return cert, nil
}
