package proxy

import (
	"bytes"
	"crypto/tls"
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

	// 用于在 Director 和 RoundTrip 之间传递认证结果的 Header
	HeaderAuthResultCode    = "X-Auth-Result-Code"
	HeaderAuthResultMessage = "X-Auth-Result-Message"
)

// AuthServerError 是一个自定义错误类型，用于封装来自主认证服务器的错误信息
type AuthServerError struct {
	StatusCode int
	Message    string
}

func (e *AuthServerError) Error() string {
	return fmt.Sprintf("认证服务器错误: 状态码 %d, 消息: %s", e.StatusCode, e.Message)
}

// Authenticator 定义了代理服务器认证器的接口。
type Authenticator interface {
	// Authenticate 验证传入的 HTTP 请求。
	// 如果认证成功，返回 true, nil。
	// 如果认证失败（例如，密钥无效或缺失），返回 false, nil。
	// 如果发生内部错误（例如，无法连接数据库），返回 false, error。
	// 如果主认证服务器返回错误，返回 false, *AuthServerError。
	Authenticate(r *http.Request) (bool, error)
}

// --- QueryParamAuthenticator ---

// QueryParamAuthenticator 是一个基于请求头的认证器
type QueryParamAuthenticator struct{}

// NewQueryParamAuthenticator 创建一个新的请求头认证器
func NewQueryParamAuthenticator() *QueryParamAuthenticator {
	return &QueryParamAuthenticator{}
}

// Authenticate 验证请求中的 x-goog-api-key 头
func (a *QueryParamAuthenticator) Authenticate(r *http.Request) (bool, error) {
	// 从请求头获取 x-goog-api-key
	clientKey := r.Header.Get("x-goog-api-key")
	log.Printf("clientKey: %s", clientKey)
	if clientKey == "" {
		// 密钥缺失，视为认证失败，但不返回错误
		return false, nil
	}

	// 调用主服务器验证 key
	// checkKeyInDatabase 现在可能返回 *AuthServerError
	isValid, err := checkKeyInDatabase(clientKey)
	if err != nil {
		// 如果是 AuthServerError，直接返回
		var authErr *AuthServerError
		if errors.As(err, &authErr) {
			return false, authErr // 传递自定义错误
		}
		// 否则，是其他内部错误
		return false, fmt.Errorf("验证客户端密钥时发生内部错误: %w", err)
	}

	// 如果 err 为 nil，isValid 才有效
	return isValid, nil
}

// --- NilAuthenticator ---

// NilAuthenticator 是一个不执行任何认证的认证器（总是允许）。
type NilAuthenticator struct{}

// Authenticate 总是返回 true。
func (na *NilAuthenticator) Authenticate(r *http.Request) (bool, error) {
	return true, nil
}

// --- 修改数据库检查函数，使用主服务器认证 ---
// checkKeyInDatabase 现在在主服务器返回非200时返回 *AuthServerError
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

	// 创建自定义的 HTTP 客户端
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // 跳过 TLS 证书验证
		},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	// 发送请求
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
			return false, fmt.Errorf("解析主服务器成功响应失败: %w", err)
		}
		// 只有 success 为 true 才算验证通过
		return response.Success, nil // 如果 success 为 false，也返回 false, nil

	} else {
		// 主服务器返回了非 200 错误
		bodyBytes, readErr := io.ReadAll(resp.Body)
		var errMsg string
		if readErr != nil {
			log.Printf("读取主服务器错误响应体失败: %v", readErr)
			errMsg = fmt.Sprintf("无法读取错误响应体 (状态码: %d)", resp.StatusCode)
		} else {
			errMsg = string(bodyBytes)
			// 尝试解析 JSON 错误消息
			var errorResp struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(bodyBytes, &errorResp) == nil && errorResp.Message != "" {
				errMsg = errorResp.Message // 使用 JSON 中的 message
			}
		}
		log.Printf("主服务器返回错误 (状态码: %d): %s", resp.StatusCode, errMsg)
		// 返回自定义错误，包含原始状态码和消息
		return false, &AuthServerError{
			StatusCode: resp.StatusCode,
			Message:    errMsg,
		}
	}
}
