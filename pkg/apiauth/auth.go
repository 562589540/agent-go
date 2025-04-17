package apiauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// 请求头中签名信息的键名
	HeaderAuthSignature = "X-Api-Signature"
	HeaderAuthTimestamp = "X-Api-Timestamp"
	HeaderAuthNonce     = "X-Api-Nonce"

	// 默认签名有效期（秒）
	DefaultExpireSeconds = 300
)

// AuthConfig 认证配置
type AuthConfig struct {
	// API密钥（应从环境变量获取）
	APIKey string
	// 允许的IP列表（可选）
	AllowedIPs []string
	// 签名过期时间（秒），默认5分钟
	ExpireSeconds int
}

// 客户端：生成请求签名头
func GenerateAuthHeaders(config AuthConfig, params map[string]string) map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateNonce(16)

	signature := generateSignature(config.APIKey, timestamp, nonce, params)

	return map[string]string{
		HeaderAuthSignature: signature,
		HeaderAuthTimestamp: timestamp,
		HeaderAuthNonce:     nonce,
	}
}

// 服务端：验证请求签名
func VerifySignature(r *http.Request, config AuthConfig) error {
	// 获取头部信息
	signature := r.Header.Get(HeaderAuthSignature)
	timestamp := r.Header.Get(HeaderAuthTimestamp)
	nonce := r.Header.Get(HeaderAuthNonce)

	if signature == "" || timestamp == "" || nonce == "" {
		return errors.New("认证头部不完整")
	}

	// 验证时间戳是否过期
	expireSeconds := config.ExpireSeconds
	if expireSeconds <= 0 {
		expireSeconds = DefaultExpireSeconds
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("时间戳格式错误")
	}

	now := time.Now().Unix()
	if now-ts > int64(expireSeconds) {
		return errors.New("请求已过期")
	}

	// 验证IP（如果配置了IP白名单）
	if len(config.AllowedIPs) > 0 {
		clientIP := getClientIP(r)
		if !isIPAllowed(clientIP, config.AllowedIPs) {
			return fmt.Errorf("IP不在白名单内: %s", clientIP)
		}
	}

	// 从请求获取参数
	params := extractRequestParams(r)

	// 计算签名
	expectedSignature := generateSignature(config.APIKey, timestamp, nonce, params)

	// 比较签名
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return errors.New("签名验证失败")
	}

	return nil
}

// 生成签名
func generateSignature(apiKey, timestamp, nonce string, params map[string]string) string {
	// 对参数按键排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接参数
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
		sb.WriteString("&")
	}

	// 添加时间戳和随机数
	sb.WriteString("timestamp=")
	sb.WriteString(timestamp)
	sb.WriteString("&nonce=")
	sb.WriteString(nonce)

	// 最后添加apiKey
	sb.WriteString("&key=")
	sb.WriteString(apiKey)

	// 打印签名前的完整字符串用于调试
	signString := sb.String()
	fmt.Printf("签名字符串: %s\n", signString)

	// 计算SHA256（不使用HMAC）
	h := sha256.New()
	h.Write([]byte(signString))
	signature := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("计算签名: %s\n", signature)

	return signature
}

// 生成随机字符串
func generateNonce(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// 获取客户端IP
func getClientIP(r *http.Request) string {
	// 尝试从X-Forwarded-For获取
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	// 尝试从X-Real-IP获取
	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	// 从RemoteAddr获取
	return strings.Split(r.RemoteAddr, ":")[0]
}

// 检查IP是否在白名单中
func isIPAllowed(ip string, allowedIPs []string) bool {
	for _, allowedIP := range allowedIPs {
		if ip == allowedIP {
			return true
		}
	}
	return false
}

// 从请求中提取参数
func extractRequestParams(r *http.Request) map[string]string {
	params := make(map[string]string)

	// 处理URL查询参数
	queryParams := r.URL.Query()
	for k, v := range queryParams {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	// 如果是POST请求，处理表单参数
	if r.Method == "POST" {
		// 检查Content-Type
		contentType := r.Header.Get("Content-Type")

		// 处理JSON请求
		if strings.Contains(contentType, "application/json") {
			// 读取请求体
			if r.Body != nil {
				defer r.Body.Close()

				// 解析JSON
				var jsonParams map[string]interface{}
				decoder := json.NewDecoder(r.Body)
				if err := decoder.Decode(&jsonParams); err == nil {
					// 将JSON参数转换为字符串
					for k, v := range jsonParams {
						switch val := v.(type) {
						case string:
							params[k] = val
						case float64:
							params[k] = strconv.FormatFloat(val, 'f', -1, 64)
						case int:
							params[k] = strconv.Itoa(val)
						case bool:
							params[k] = strconv.FormatBool(val)
						default:
							// 对于其他类型，尝试JSON序列化
							if jsonValue, err := json.Marshal(v); err == nil {
								params[k] = string(jsonValue)
							}
						}
					}

					// 打印提取的JSON参数用于调试
					fmt.Printf("提取的JSON参数: %v\n", params)
				} else {
					fmt.Printf("JSON解析错误: %v\n", err)
				}

				// 由于已经读取了请求体，需要重置它以便后续处理
				// 创建一个新的请求体
				bodyBytes, _ := json.Marshal(jsonParams)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// 处理表单请求
			err := r.ParseForm()
			if err == nil {
				for k, v := range r.PostForm {
					if len(v) > 0 {
						params[k] = v[0]
					}
				}
			}
		}
	}

	// 打印最终提取的所有参数
	fmt.Printf("提取的所有参数: %v\n", params)

	return params
}
