package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/562589540/agent-go/pkg/proxy"
	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	defer ctx.Done()

	// 加载 .env 文件
	err := godotenv.Load()
	if err != nil {
		log.Println("提示: 未找到 .env 文件，将仅依赖系统环境变量。")
	} else {
		log.Println("已成功加载 .env 文件。")
	}

	// 设置监听地址和API密钥
	addr := getEnvValue("LISTEN_ADDR", "0.0.0.0:8091")
	apiKeysStr := getEnvValue("API_KEYS", "")
	upstreamProxy := getEnvValue("UPSTREAM_PROXY", "")

	// 读取速率限制配置
	enableRateLimit := getEnvValueBool("ENABLE_RATE_LIMIT", true)
	rateLimitWindow := getEnvValueInt("RATE_LIMIT_WINDOW", proxy.DefaultRateLimitWindow)
	maxRequests := getEnvValueInt("MAX_REQUESTS_PER_WINDOW", proxy.DefaultMaxRequestsPerWindow)
	blacklistTimeout := getEnvValueInt("BLACKLIST_TIMEOUT", proxy.DefaultBlacklistTimeout)

	// 读取域名黑名单配置
	enableDomainBlock := getEnvValueBool("ENABLE_DOMAIN_BLOCK", true)
	domainBlacklistStr := getEnvValue("DOMAIN_BLACKLIST", "")

	// 读取日志抑制配置
	enableLogSuppression := getEnvValueBool("ENABLE_LOG_SUPPRESSION", true)
	logSuppressionWindow := getEnvValueInt("LOG_SUPPRESSION_WINDOW", proxy.DefaultLogSuppressionWindow)
	logSuppressionThreshold := getEnvValueInt("LOG_SUPPRESSION_THRESHOLD", proxy.DefaultLogSuppressionThreshold)

	// 解析额外的黑名单域名
	var additionalBlockedDomains []string
	if domainBlacklistStr != "" {
		additionalBlockedDomains = strings.Split(domainBlacklistStr, ",")
		// 清理可能存在的前后空格
		for i := range additionalBlockedDomains {
			additionalBlockedDomains[i] = strings.TrimSpace(additionalBlockedDomains[i])
		}
	}

	// 检查必要的配置
	if apiKeysStr == "" {
		log.Fatal("错误: 环境变量 API_KEYS 未设置 (请在 .env 文件或系统中设置，用逗号分隔多个密钥)")
	}

	// 解析 API 密钥池
	apiKeys := strings.Split(apiKeysStr, ",")
	if len(apiKeys) == 0 || (len(apiKeys) == 1 && apiKeys[0] == "") {
		log.Fatal("错误: API_KEYS 环境变量格式不正确或为空")
	}
	// 清理可能存在的前后空格
	for i := range apiKeys {
		apiKeys[i] = strings.TrimSpace(apiKeys[i])
	}
	log.Printf("已从 API_KEYS 加载 %d 个密钥", len(apiKeys))

	fmt.Printf("启动 HTTP 代理服务器在 %s\n", addr)
	fmt.Printf("代理将使用 %d 个 API 密钥轮换替换 Google API 请求中的密钥。\n", len(apiKeys))
	if upstreamProxy != "" {
		fmt.Printf("使用上游代理: %s\n", upstreamProxy)
	} else {
		fmt.Println("上游代理: (未配置, 将直接连接)")
	}

	fmt.Println("认证方式: URL 查询参数 'auth_key' (需要客户端在 URL 中提供)")

	// 打印防御系统配置
	if enableRateLimit {
		fmt.Printf("已启用速率限制: 每%d秒最多%d个请求, 超限封禁%d分钟\n",
			rateLimitWindow, maxRequests, blacklistTimeout)
	} else {
		fmt.Println("速率限制: 已禁用")
	}

	if enableDomainBlock {
		fmt.Printf("已启用域名黑名单")
		if len(additionalBlockedDomains) > 0 {
			fmt.Printf("，额外添加了 %d 个黑名单域名\n", len(additionalBlockedDomains))
		} else {
			fmt.Println("，使用默认黑名单")
		}
	} else {
		fmt.Println("域名黑名单: 已禁用")
	}

	if enableLogSuppression {
		fmt.Printf("已启用日志抑制: 窗口%d秒内超过%d次相同错误将减少记录\n",
			logSuppressionWindow, logSuppressionThreshold)
	} else {
		fmt.Println("日志抑制: 已禁用")
	}

	fmt.Println("\n使用方法:")
	fmt.Println("  1. 客户端连接代理时需在 URL 中附加 '?auth_key=<你的密钥>'")
	fmt.Printf("     例如: http://127.0.0.1%s?auth_key=client1_secret\n", addr[strings.LastIndex(addr, ":"):])
	fmt.Println("  2. 代理服务器会自动拦截 Gemini API 请求并使用密钥池中的密钥替换")
	fmt.Println("  3. 如果配置了上游代理，将通过上游代理连接目标服务器")
	fmt.Println("  4. 同时支持 HTTP 和 HTTPS 请求")

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在后台启动代理
	errChan := make(chan error, 1)
	go func() {
		// 创建查询参数认证器
		authenticator := proxy.NewQueryParamAuthenticator()
		log.Println("已启用基于 URL 查询参数 'auth_key' 的认证。")

		// 创建代理实例
		proxyServer, err := proxy.NewProxy(ctx, addr, apiKeys, upstreamProxy, authenticator)
		if err != nil {
			errChan <- fmt.Errorf("创建代理实例失败: %w", err)
			return
		}

		// 配置防御系统
		if defenseSystem, ok := proxyServer.GetDefenseSystem().(*proxy.DefaultDefenseSystem); ok {
			// 设置速率限制
			defenseSystem.EnableRateLimit(enableRateLimit)
			if enableRateLimit {
				defenseSystem.SetRateLimitConfig(rateLimitWindow, maxRequests, blacklistTimeout)
			}

			// 设置域名黑名单
			defenseSystem.EnableDomainBlock(enableDomainBlock)
			// 添加额外的黑名单域名
			for _, domain := range additionalBlockedDomains {
				if domain != "" {
					defenseSystem.BlockDomain(domain)
				}
			}

			// 设置日志抑制
			defenseSystem.EnableLogSuppression(enableLogSuppression)
			if enableLogSuppression {
				defenseSystem.SetLogSuppressionConfig(logSuppressionWindow, logSuppressionThreshold)
			}
		}

		// 启动代理服务器
		err = proxyServer.Start()
		if err != nil {
			errChan <- fmt.Errorf("代理启动失败: %w", err)
		}
	}()

	// 等待终止信号或错误
	select {
	case <-sigChan:
		fmt.Println("\n收到终止信号，正在关闭...")
		time.Sleep(1 * time.Second)
	case err := <-errChan:
		log.Fatalf("代理服务器错误: %v\n", err)
	}
	log.Println("代理服务器已退出。")
}

// getEnvValue 从环境变量获取值，如果不存在则返回默认值
func getEnvValue(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if defaultValue != "" {
		log.Printf("环境变量 '%s' 未设置, 使用默认值: %s", key, defaultValue)
	}
	return defaultValue
}

// getEnvValueBool 从环境变量获取布尔值
func getEnvValueBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		boolValue, err := strconv.ParseBool(value)
		if err == nil {
			return boolValue
		}
		log.Printf("警告: 环境变量 '%s' 值 '%s' 无法解析为布尔值，使用默认值: %v", key, value, defaultValue)
	}
	log.Printf("环境变量 '%s' 未设置, 使用默认值: %v", key, defaultValue)
	return defaultValue
}

// getEnvValueInt 从环境变量获取整数值
func getEnvValueInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, err := strconv.Atoi(value)
		if err == nil {
			return intValue
		}
		log.Printf("警告: 环境变量 '%s' 值 '%s' 无法解析为整数，使用默认值: %d", key, value, defaultValue)
	}
	log.Printf("环境变量 '%s' 未设置, 使用默认值: %d", key, defaultValue)
	return defaultValue
}
