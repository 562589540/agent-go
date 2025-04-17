package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/562589540/agent-go/pkg/proxy"
	"github.com/joho/godotenv"
)

func main() {
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

		// 启动代理服务器，传入 API 密钥切片和认证器
		err := proxy.StartProxy(addr, apiKeys, upstreamProxy, authenticator)
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
