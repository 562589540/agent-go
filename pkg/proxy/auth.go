package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/562589540/agent-go/pkg/apiauth"
)

// 常量定义
const (
	// 请求头中签名信息的键名
	HeaderAuthSignature = "X-Api-Signature"
	HeaderAuthTimestamp = "X-Api-Timestamp"
	HeaderAuthNonce     = "X-Api-Nonce"

	// 默认签名有效期（秒）
	DefaultExpireSeconds = 300
)

// Authenticator 定义了代理服务器认证器的接口。
type Authenticator interface {
	// Authenticate 验证传入的 HTTP 请求。
	// 如果认证成功，返回 true, nil。
	// 如果认证失败（例如，密钥无效或缺失），返回 false, nil。
	// 如果发生内部错误（例如，无法连接数据库），返回 false, error。
	Authenticate(r *http.Request) (bool, error)
}

// --- APIAuthenticator --- 实现通过调用主服务器 API 进行认证

// APIAuthenticator 实现基于主服务器 API 的认证
type APIAuthenticator struct {
	apiEndpoint string // 主服务器验证 API 的端点
	apiKey      string // 用于生成签名的 API 密钥
	httpClient  *http.Client
	logger      *log.Logger
}

// NewAPIAuthenticator 创建一个新的 APIAuthenticator 实例
func NewAPIAuthenticator(apiEndpoint, apiKey string) *APIAuthenticator {
	logger := log.New(os.Stderr, "[AuthAPI] ", log.LstdFlags)
	logger.Printf("使用主服务器认证 API: %s", apiEndpoint)

	return &APIAuthenticator{
		apiEndpoint: apiEndpoint,
		apiKey:      apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Authenticate 实现 Authenticator 接口
func (a *APIAuthenticator) Authenticate(r *http.Request) (bool, error) {
	authKey := r.URL.Query().Get("auth_key")
	if authKey == "" {
		a.logger.Printf("认证失败: 请求 %s %s 缺少 'auth_key' 查询参数", r.Method, r.URL.Path)
		return false, nil // 认证失败，但不是内部错误
	}

	// 使用已有的 checkKeyInDatabase 函数，该函数已修改为调用主服务器验证
	isValid, err := checkKeyInDatabase(authKey)
	if err != nil {
		a.logger.Printf("认证错误: 检查密钥时出错 (Key: %s): %v", maskKey(authKey), err)
		return false, err // 返回内部错误
	}

	if !isValid {
		a.logger.Printf("认证失败: 主服务器拒绝密钥 (Key: %s)", maskKey(authKey))
		return false, nil // 认证失败
	}

	a.logger.Printf("认证成功: 主服务器验证通过 (Key: %s)", maskKey(authKey))
	return true, nil
}

// --- QueryParamAuthenticator ---

// QueryParamAuthenticator 实现基于 URL 查询参数 'auth_key' 的认证，
// 并将实际的密钥验证委托给 checkKeyInDatabase (或其他实现)。
type QueryParamAuthenticator struct {
	logger *log.Logger
}

// NewQueryParamAuthenticator 创建一个新的 QueryParamAuthenticator。
func NewQueryParamAuthenticator() *QueryParamAuthenticator {
	return &QueryParamAuthenticator{
		logger: log.New(os.Stderr, "[AuthQueryParam] ", log.LstdFlags),
	}
}

// Authenticate 检查请求 URL 中是否存在 'auth_key' 查询参数，并调用验证函数。
func (qa *QueryParamAuthenticator) Authenticate(r *http.Request) (bool, error) {
	authKey := r.URL.Query().Get("auth_key")

	if authKey == "" {
		qa.logger.Printf("认证失败: 请求 %s %s 缺少 'auth_key' 查询参数", r.Method, r.URL.Path)
		return false, nil // 认证失败，但不是内部错误
	}

	// 调用实际的验证逻辑 (现在会向主服务器发送验证请求)
	isValid, err := checkKeyInDatabase(authKey)
	if err != nil {
		qa.logger.Printf("认证错误: 检查密钥时出错 (Key: %s): %v", maskKey(authKey), err)
		return false, err // 返回内部错误
	}

	if !isValid {
		qa.logger.Printf("认证失败: 无效的 'auth_key': %s", maskKey(authKey))
		return false, nil // 认证失败
	}

	qa.logger.Printf("认证成功: 请求 %s %s (Key: %s)", r.Method, r.URL.Path, maskKey(authKey))
	return true, nil // 认证成功
}

// --- NilAuthenticator ---

// NilAuthenticator 是一个不执行任何认证的认证器（总是允许）。
type NilAuthenticator struct{}

// Authenticate 总是返回 true。
func (na *NilAuthenticator) Authenticate(r *http.Request) (bool, error) {
	return true, nil
}

// --- 修改数据库检查函数，使用主服务器认证 ---
func checkKeyInDatabase(key string) (bool, error) {
	var originalKey string

	// 尝试将key当作临时token解密
	decodedKey, err := DecodeToken(key)
	if err != nil {
		// 解密失败，继续使用原始key
		originalKey = key
	} else {
		// 解密成功，使用解密后的key
		originalKey = decodedKey
	}

	// 从环境变量获取主服务器认证 API 的 URL 和密钥
	apiURL := os.Getenv("MAIN_SERVER_AUTH_API_URL")
	if apiURL == "" {
		return false, errors.New("环境变量 MAIN_SERVER_AUTH_API_URL 未设置")
	}

	apiKey := os.Getenv("PROXY_MAIN_SERVER_SECRET")
	if apiKey == "" {
		return false, errors.New("环境变量 PROXY_MAIN_SERVER_SECRET 未设置")
	}

	// 准备请求参数
	params := map[string]string{
		"auth_key": originalKey, // 使用解密后的originalKey
	}

	// 使用 apiauth 包生成认证头
	authConfig := apiauth.AuthConfig{
		APIKey:        apiKey,
		ExpireSeconds: 300, // 5分钟过期
	}

	// 生成认证头
	authHeaders := apiauth.GenerateAuthHeaders(authConfig, params)

	// 准备请求体
	requestBody, err := json.Marshal(params)
	if err != nil {
		return false, fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return false, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for k, v := range authHeaders {
		req.Header.Set(k, v)
	}

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("发送认证请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 处理响应
	if resp.StatusCode == http.StatusOK {
		var response struct {
			Success bool   `json:"success"`
			Message string `json:"message,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return false, fmt.Errorf("解析响应失败: %w", err)
		}

		return response.Success, nil
	} else if resp.StatusCode == http.StatusTooManyRequests {
		return false, nil // 速率限制，视为认证失败
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("主服务器返回错误 (状态码: %d): %s", resp.StatusCode, string(bodyBytes))
		return false, fmt.Errorf("认证服务返回状态码: %d", resp.StatusCode)
	}
}
