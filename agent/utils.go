package agent

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

const caCertStr = `-----BEGIN CERTIFICATE-----
MIIDZzCCAk+gAwIBAgIQI4eeTM7PfTDKk00I+kLWhzANBgkqhkiG9w0BAQsFADA+
MR0wGwYDVQQKExRHb1Byb3h5IE1JVE0gUm9vdCBDQTEdMBsGA1UEAxMUR29Qcm94
eSBNSVRNIFJvb3QgQ0EwHhcNMjUwNDE4MDE0NzM0WhcNMzUwNDE4MDE0NzM0WjA+
MR0wGwYDVQQKExRHb1Byb3h5IE1JVE0gUm9vdCBDQTEdMBsGA1UEAxMUR29Qcm94
eSBNSVRNIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC9
WTS7YZgpdpoo3VDesFrfCamQOOtg42bD62eQwQh3qEEfvx3woS7ycw4V9JdR+/ad
Vjax6i46z75Dq8ccyatTt0vBpnOpSn+7yOV7/TxC7+tg6OiCanDLQ+eya4+KE1AH
CUMK+chmUjCaJkdue2SdafLelR5lyXkNu3ZS1SCBRamK8CceDFrRGEF9cbXGHOhm
GtwlS5/zFkLByldxW8QCIAtb8rmlnac4tvhX85rIV/0Ox2VrV2f9kcZvP+GODVTT
90N2vHLQC8RWhy7l1Z+WW/B2Z2mQb6F6SlDdBM/zVNA17HGNUQ33L5JCYHerKkdD
YRRkgVExaZAauab7LwBxAgMBAAGjYTBfMA4GA1UdDwEB/wQEAwIBBjAdBgNVHSUE
FjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4E
FgQUxMcI6pQc84qBvb/fDMfR0RAKv/gwDQYJKoZIhvcNAQELBQADggEBAA6W24rK
9dJoa4O/ptQY3skFcQByOJP+zBxIVMrc21VdRZ3L+JaajS48OOSyPG+/R74ATQ4+
wYaITUJslT6Rb1aZr1t3EZ6MYWFMcPy/uxjdvb6aGphT6nhzAppyLQs2fvf00lzF
UqNJLiIRfaBJRYcHNExGyDHH414thxrk++0RTBv2l8u+iDnXeseZj8+IqfBUFYlE
YlUt+5ZczAXb8GNDhZcP08hC60iAowXCqrKu6NxeuzY2J6JMAto7+n+LExBlcDTw
WsWjP8mROPFWM5+bBMRChLRb751zSWDXD+OPRQy6oKargzXinzp9n1UxBbMKwdYC
sBQfUHgg158hlXw=
-----END CERTIFICATE-----`

func PrintJSON(prefix string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%s: 错误: %v\n", prefix, err)
		return
	}
	fmt.Printf("%s: %s\n", prefix, string(data))
}

// createProxiedHttpClientWithCustomCA 创建一个配置了代理和自定义 CA 信任的 HTTP 客户端
// proxyAddr: 代理服务器的完整 URL，例如 "http://localhost:8091" 或 "http://user:pass@host:port"
// 返回配置好的 *http.Client 和错误（如果配置失败）
func CreateProxiedHttpClientWithCustomCA(proxyAddr string) (*http.Client, error) {
	// 1. 解析代理 URL
	proxyUrl, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("解析代理URL错误 '%s': %w", proxyAddr, err)
	}

	// 或者只信任自定义 CA
	rootCAs := x509.NewCertPool()

	if ok := rootCAs.AppendCertsFromPEM([]byte(caCertStr)); !ok {
		return nil, fmt.Errorf("无法将代理 CA 证书添加到信任池")
	}
	//fmt.Printf("【INFO】辅助函数: 已加载并信任代理 CA 证书\n")

	// 4. 创建包含自定义 CA 的 TLS 配置
	tlsConfig := &tls.Config{
		RootCAs: rootCAs,
		// 可以根据需要设置 MinVersion 等
		// MinVersion: tls.VersionTLS12,
	}

	// 5. 创建 HTTP Transport，配置代理和 TLS
	transport := &http.Transport{
		// Proxy: 指定用于请求的代理服务器。http.ProxyURL(proxyUrl) 会将代理 URL 转换为一个函数，
		//        该函数返回用于给定请求的代理 URL。如果为 nil，则不使用代理或使用环境变量设置的代理。
		Proxy: http.ProxyURL(proxyUrl), // 设置代理服务器

		// TLSClientConfig: 配置客户端的 TLS (HTTPS) 设置。
		//                  这里传入的 tlsConfig 可能包含了自定义的根 CA 证书池 (RootCAs)，
		//                  用于验证服务器证书（或者在 MITM 场景下验证代理伪造的证书）。
		//                  如果为 nil，则使用默认的 TLS 配置（通常信任操作系统安装的根 CA）。
		TLSClientConfig: tlsConfig, // 设置自定义 TLS 配置，用于证书验证

		// DialContext: 一个自定义的函数，用于创建到底层目标地址（或代理服务器）的网络连接 (通常是 TCP)。
		//              这里使用了标准库的 net.Dialer，并配置了连接超时和 KeepAlive 参数。
		//              KeepAlive 用于启用 TCP Keep-Alive，定期发送探测包以保持连接活跃或检测死连接。
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // 建立 TCP 连接尝试的最长等待时间。
			KeepAlive: 30 * time.Second, // TCP Keep-Alive 探测的时间间隔。如果连接空闲超过这个时间，会发送探测包。设为负值禁用。
		}).DialContext, // 使用配置好的 Dialer 来建立连接

		// MaxIdleConns: 控制客户端连接池中**总共**可以保持的空闲 (keep-alive) 连接的最大数量。
		//               这些连接可以在后续请求中被复用，避免重新建立 TCP 和 TLS 连接，从而提高性能。
		//               如果为 0，则使用 Go HTTP 库的默认值 (通常是 100)。
		MaxIdleConns: 100, // 连接池中最大允许的空闲连接数

		// IdleConnTimeout: 一个已建立的空闲 (keep-alive) 连接在被自动关闭之前可以保持空闲状态的最长时间。
		//                  如果一个连接在这个时间内没有任何活动，它将被从连接池中移除并关闭。
		//                  如果为 0，则表示没有超时（不推荐）。
		IdleConnTimeout: 90 * time.Second, // 空闲连接在池中存活的最长时间

		// TLSHandshakeTimeout: TLS 握手过程的最长允许时间。
		//                      这包括了客户端和服务器之间交换证书、验证证书、协商加密算法等所有步骤。
		//                      如果握手超时，连接会失败。
		TLSHandshakeTimeout: 10 * time.Second, // TLS 握手允许的最长时间

		// ExpectContinueTimeout: 如果客户端发送的请求包含 "Expect: 100-continue" 头（通常用于 POST/PUT 大数据之前），
		//                        这是客户端在发送请求体之前，等待服务器回应 "100 Continue" 状态码的最长时间。
		//                        如果超时或服务器返回非 100 状态，客户端可能会直接发送请求体或中止请求（取决于实现）。
		//                        如果为 0，则客户端不会发送 "Expect: 100-continue" 头。
		ExpectContinueTimeout: 1 * time.Second, // 等待服务器 "100 Continue" 响应的超时时间
	}

	// 6. 创建并返回 HTTP 客户端
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Minute, // 为整个请求设置超时
	}
	//fmt.Printf("【INFO】辅助函数: HTTP 客户端已配置使用代理 %s 并信任 CA\n", proxyAddr)

	return httpClient, nil
}
