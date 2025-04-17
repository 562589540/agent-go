package proxy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

// TokenExpiry 定义临时token的有效期（秒）
const TokenExpiry = 60 // 1分钟有效期

// 服务器密钥 - 固定写死，客户端和服务器端必须一致
var serverSecretKey = []byte("d41d8cd98f00b204e9800998ecf8427e12345678")

// GenerateTempToken 根据原始auth_key生成临时token
// 使用AES加密和当天日期作为动态因子
func GenerateTempToken(authKey string) string {
	// 当前时间戳
	now := time.Now()
	expiry := now.Add(TokenExpiry * time.Second)

	// 使用当天日期作为动态因子生成密钥
	dailyKey := generateDailyKey(now)

	// 构造要加密的明文：原始authKey + 过期时间戳
	plaintext := authKey + "|" + strconv.FormatInt(expiry.Unix(), 10)

	// 使用AES-GCM加密
	encryptedData, err := encryptAES([]byte(plaintext), dailyKey)
	if err != nil {
		// 加密失败时返回空字符串
		return ""
	}

	// Base64编码使其URL安全
	return base64.URLEncoding.EncodeToString(encryptedData)
}

// DecodeToken 解码临时token，获取原始auth_key
// 如果token无效或已过期，返回错误
func DecodeToken(token string) (string, error) {
	// 尝试解码Base64
	encryptedData, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("无效的token格式")
	}

	// 使用当天日期作为动态因子生成密钥
	dailyKey := generateDailyKey(time.Now())

	// 解密数据
	decrypted, err := decryptAES(encryptedData, dailyKey)
	if err != nil {
		// 如果当天密钥无法解密，尝试使用昨天的密钥（处理跨天边界问题）
		yesterdayKey := generateDailyKey(time.Now().AddDate(0, 0, -1))
		decrypted, err = decryptAES(encryptedData, yesterdayKey)
		if err != nil {
			return "", errors.New("token解密失败")
		}
	}

	// 解析解密后的数据
	parts := strings.Split(string(decrypted), "|")
	if len(parts) != 2 {
		return "", errors.New("token格式错误")
	}

	authKey, expiryStr := parts[0], parts[1]

	// 验证过期时间
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", errors.New("无效的过期时间")
	}

	// 检查是否过期
	if time.Now().Unix() > expiry {
		return "", errors.New("token已过期")
	}

	return authKey, nil
}

// generateDailyKey 根据当天日期生成一个动态密钥
// 使用服务器密钥和当前日期（年月日）生成
func generateDailyKey(t time.Time) []byte {
	// 格式化当前日期为 YYYYMMDD
	dateStr := t.Format("20060102")

	// 组合服务器密钥和日期
	saltedKey := append(serverSecretKey, []byte(dateStr)...)

	// 使用SHA-256生成固定长度的密钥
	hash := sha256.Sum256(saltedKey)
	return hash[:]
}

// encryptAES 使用AES-GCM加密数据
func encryptAES(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 创建随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decryptAES 使用AES-GCM解密数据
func decryptAES(encryptedData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 提取nonce
	if len(encryptedData) < gcm.NonceSize() {
		return nil, errors.New("加密数据长度不足")
	}

	nonce, ciphertext := encryptedData[:gcm.NonceSize()], encryptedData[gcm.NonceSize():]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
