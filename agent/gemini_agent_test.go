package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/562589540/agent-go/pkg/proxy"
	"google.golang.org/genai"
)

// 测试GeminiAgent基本功能
func TestGeminiAgent(t *testing.T) {
	// 获取API密钥，如果没有设置跳过测试
	apiKey := "AIzaSyBlqIMp0iRkU66zyk-tozMAxmnD1GWT7uY"
	proxyURL := "http://127.0.0.1:7890"

	// 配置代理
	config := AgentConfig{
		APIKey:      apiKey,
		ProxyURL:    proxyURL,
		Debug:       true,
		MaxLoops:    3,
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        0.9,
	}

	// 创建代理
	agent, err := NewGeminiAgent(config)
	if err != nil {
		t.Fatalf("创建GeminiAgent失败: %v", err)
	}

	// 注册一个时间工具
	currentTimeSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "时区，如'Asia/Shanghai'",
			},
		},
		"required": []string{},
	}

	timeFunction := FunctionDefinitionParam{
		Name:        "current_time",
		Description: "获取当前时间",
		Parameters:  currentTimeSchema,
	}

	err = agent.RegisterTool(timeFunction, func(args map[string]interface{}) (string, error) {
		// timezone, exists := args["timezone"].(string)
		// if !exists || timezone == "" {
		// 	timezone = "Local"
		// }

		// // 获取当前时间
		// now := time.Now()
		// return fmt.Sprintf"当前时间是: %s", now.Format("2006-01-02 15:04:05")), nil
		return "", fmt.Errorf("函数调用失败")
	})

	if err != nil {
		t.Fatalf("注册工具失败: %v", err)
	}

	// 创建测试对话
	ctx := context.Background()
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手，可以回答问题并使用工具。",
		},
		{
			Role:    "user",
			Content: "现在几点了?",
		},
	}

	// 收集流式消息
	var outputMessages []string

	// 创建流式回调
	streamHandler := func(text string) {
		outputMessages = append(outputMessages, text)
		// 在测试中也打印消息
		t.Logf("接收到消息: %s", text)
	}

	// 运行对话
	tokenUsage, _, err := agent.StreamRunConversation(ctx, "gemini-1.5-pro-latest", messages, streamHandler)
	if err != nil {
		t.Fatalf("执行对话失败: %v", err)
	}

	// 验证是否有输出
	if len(outputMessages) == 0 {
		t.Error("没有收到任何输出消息")
	}

	// 验证token使用情况
	t.Logf("Token使用情况: %+v", tokenUsage)
	if tokenUsage.TotalTokens == 0 {
		t.Error("没有记录Token使用情况")
	}
}

// 测试GeminiAgent使用谷歌搜索功能
func TestGeminiAgentWithGoogleSearch(t *testing.T) {
	// 获取API密钥，如果没有设置跳过测试
	//apiKey := "xxxxx3xx3"
	apiKey := proxy.GenerateTempToken("AC-0003-CWWU-HWDW-4ZLD-D-HF-E7")
	// 尝试不同的代理格式
	proxyURL := "http://43.134.14.16:8091" // 重新添加认证参数

	proxyClient, err := CreateProxiedHttpClientWithCustomCA(proxyURL)
	if err != nil {
		t.Fatalf("创建代理客户端失败: %v", err)
	}

	// 输出实际代理URL
	t.Logf("使用代理URL: %s", proxyURL)

	// 配置代理
	config := AgentConfig{
		APIKey:      apiKey,
		Debug:       true,
		MaxLoops:    5, // 增加循环次数，因为搜索可能需要多次交互
		MaxTokens:   2000,
		Temperature: 0.7,
		TopP:        0.9,
		Client:      proxyClient,
	}

	// 创建代理
	agent, err := NewGeminiAgent(config)
	if err != nil {
		t.Fatalf("创建GeminiAgent失败: %v", err)
	}

	// //创建配置
	// searchConfig := GoogleSearchConfig{
	// 	APIKey:         "AIzaSyByFckwiCTv6DvlL2cfvOmPwWXhGJmYNYI",
	// 	SearchEngineID: "c28d9acdf00c6418e",
	// 	ProxyURL:       "http://127.0.0.1:7890", // 可选
	// }

	// // 向Agent注册工具
	// err = RegisterGoogleSearchTool(agent, searchConfig)
	// if err != nil {
	// 	t.Fatalf("注册谷歌搜索工具失败: %v", err)
	// }

	// 创建测试对话 - 要求模型使用搜索
	ctx := context.Background()
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手，可以回答问题并使用工具。当需要查询最新信息时，请使用google_search工具。",
		},
		{
			Role:    "user",
			Content: "查看2025年最新抖音热梗，有什么热点可以蹭",
		},
	}

	// 收集流式消息
	var outputMessages []string

	// 创建流式回调
	streamHandler := func(text string) {
		outputMessages = append(outputMessages, text)
		// 在测试中也打印消息
		t.Logf("接收到消息: %s", text)
	}

	// 运行对话
	t.Log("开始执行带有谷歌搜索的对话...")
	tokenUsage, _, err := agent.StreamRunConversation(ctx, "gemini-2.0-flash", messages, streamHandler)
	if err != nil {
		t.Fatalf("执行对话失败: %v", err)
	}

	// 验证是否有输出
	if len(outputMessages) == 0 {
		t.Error("没有收到任何输出消息")
	}

	// 验证token使用情况
	t.Logf("Token使用情况: %+v", tokenUsage)
	if tokenUsage.TotalTokens == 0 {
		t.Error("没有记录Token使用情况")
	}

	// 输出完整回复
	fullResponse := ""
	for _, msg := range outputMessages {
		fullResponse += msg
	}
	t.Logf("完整回复: %s", fullResponse)
}

// 测试直接使用 genai 客户端通过代理进行非流式调用 (使用 google.golang.org/genai 风格)
func TestGenaiDirectNonStreaming(t *testing.T) {
	apiKey := "AIzaSyCPSvcZBvoevHaF980VG-t765TtEvUUvzI" // 使用你配置在 simple_proxy 中的 API Key
	proxyURLStr := "http://127.0.0.1:7890"              // 代理服务器地址
	modelName := "gemini-1.5-flash"                     // 或其他你可用的模型
	prompt := "讲一个关于程序员的短笑话"

	t.Logf("使用代理URL: %s", proxyURLStr)
	t.Logf("使用模型1我2: %s", modelName)

	// --- 配置 HTTP Client 使用代理 ---
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		t.Fatalf("解析代理URL失败: %v", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		// 可选：添加TLS客户端配置以信任自定义CA（如果需要）
		// TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 注意：仅用于测试！
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Minute,
	}
	// --- HTTP Client 配置结束 ---

	ctx := context.Background()

	// --- 创建 genai Client ---
	// 创建Gemini客户端配置
	clientConfig := &genai.ClientConfig{
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}

	// 创建Gemini客户端
	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		t.Fatalf("创建 genai 客户端失败: %v", err)
	}
	// defer client.Close() // 新版SDK似乎没有Close方法
	// --- genai Client 创建结束 ---

	// 获取模型实例并发送请求
	model := client.Models

	t.Log("发送非流式请求...")
	// 修正：创建一个 *genai.Content 包含 genai.NewPartFromText，然后将其放入 []*genai.Content 切片
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{genai.NewPartFromText(prompt)},
			Role:  "user", // 指定角色
		},
	}
	resp, err := model.GenerateContent(ctx, modelName, contents, nil)

	// --- 检查响应 ---
	if err != nil {
		t.Fatalf("GenerateContent 请求失败: %v", err)
	}

	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		t.Fatal("收到了空的响应或候选内容")
	}

	// 提取并打印文本响应
	var responseText string
	// 修正2：直接访问 part.Text 字段
	part := resp.Candidates[0].Content.Parts[0]
	if part.Text != "" {
		responseText = part.Text
	} else {
		t.Logf("警告: 响应的第一个部分不是文本: %+v", part) // 添加日志以防万一
	}

	t.Logf("收到响应: %s", responseText)
	if responseText == "" {
		t.Error("收到的响应文本为空")
	}
	// --- 响应检查结束 ---
}

// 测试设置调试模式
func TestSetDebug(t *testing.T) {
	config := AgentConfig{
		APIKey: "dummy-key",
		Debug:  false,
	}

	agent, err := NewGeminiAgent(config)
	if err != nil {
		t.Fatalf("创建GeminiAgent失败: %v", err)
	}

	// 验证初始状态
	if agent.config.Debug {
		t.Error("Debug模式应该初始为false")
	}

	// 设置debug为true
	agent.SetDebug(true)
	if !agent.config.Debug {
		t.Error("Debug模式应该被设置为true")
	}

	// 设置debug为false
	agent.SetDebug(false)
	if agent.config.Debug {
		t.Error("Debug模式应该被设置为false")
	}
}

func TestGeminiKeys(t *testing.T) {
	keysRaw := `AIzaSyB1A8HeawYSH2Ckg4FhRiffusDuphkH6pA----qr5453n8@tbd.hendiao.org----4zt9q9cqig----iehlai87@outlook.com
AIzaSyBHzp1Ns4-qznp83GVwpTuqM23G-yJlOvU----qr5453n8@tbd.hendiao.org----4zt9q9cqig----iehlai87@outlook.com
AIzaSyC-sb9O2llxzYTEsQ2885O_bnIr6iaPjUo----qrn5iu3ur8@tbd.hendiao.org----gcuseicnfs----pcx5g338@outlook.com
AIzaSyCOXLTj9u71U_jUF7peuNZzREnrvQL_Qb4----qrn5iu3ur8@tbd.hendiao.org----gcuseicnfs----pcx5g338@outlook.com
AIzaSyCOVCfZ-7Bx_A0C0oBB9Qq9JyVEr7eNLAM----qsie6ipu@tbd.hendiao.org----om949htn----rcip8e864s@outlook.com
AIzaSyBK4t11o3_JAJtpnwGE6-d6rjg8z2jcR2U----qsie6ipu@tbd.hendiao.org----om949htn----rcip8e864s@outlook.com
AIzaSyDhafHF_Yb8JyDinw3-ZepEBuMBcfCzixg----qu4wrf8493@tbd.hendiao.org----vsaw1197d4----87627xtr@outlook.com
AIzaSyAHnGElictTc9Bwvehkowb4q6cGoLFglRI----qu4wrf8493@tbd.hendiao.org----vsaw1197d4----87627xtr@outlook.com
AIzaSyBZXRoYJq9GtwXCyUKpAD3sBYgp4n2ZEdI----qupm42ic@tbd.hendiao.org----c5u4e4hsm8----8xw3z1a16@outlook.com
AIzaSyDnkVwTRw_i8AB9l-8OvszzxDqT7gROkA0----qupm42ic@tbd.hendiao.org----c5u4e4hsm8----8xw3z1a16@outlook.com
AIzaSyC3Z61iPEsk3YoXUG-2satxFKuXfqmH07U----quwqqr76h3@tbd.hendiao.org----ucmlqil6q----eiufqfgb1e@outlook.com
AIzaSyAAt1aAFxc01cg0wppIlaOzx7fx_O0z_sM----quwqqr76h3@tbd.hendiao.org----ucmlqil6q----eiufqfgb1e@outlook.com
AIzaSyBwXvKdbV_Kvc-gjgTDV3R0IF43x3ElNCM----qxh87y93g2@tbd.hendiao.org----8uz1im39d----259uq53n@outlook.com
AIzaSyCw6bsreKCE3f4UJps75ekqCBEv6GqtxMY----qxh87y93g2@tbd.hendiao.org----8uz1im39d----259uq53n@outlook.com
AIzaSyA1LcmW_fYgmUH_4LirmJonY0gvdOaytJk----r22ho8bz@tbd.hendiao.org----198iad7z69----h633u76i22@outlook.com
AIzaSyDR82KiZdchHl3_zQHQnH0uCtHPKecJFSM----r22ho8bz@tbd.hendiao.org----198iad7z69----h633u76i22@outlook.com
AIzaSyDP1u-uxJGtjFnT5CeldAV-g4nIWPVy1Jo----r2ya48i1c@tbd.hendiao.org----m3n7cr57e----o9d9afo4@outlook.com
AIzaSyCTczEimSIW-Vv_3O1dwhnHLKUZ3OWTEMw----r2ya48i1c@tbd.hendiao.org----m3n7cr57e----o9d9afo4@outlook.com
AIzaSyB_kxNJV_pg9DAWACIkdbgGSB3R-wWyVAM----r433j86m2@tbd.hendiao.org----9p9f3csgc----la227c7h@outlook.com
AIzaSyBPHwUO6z2sE7O5FyTSMusmA4pmi0fpObw----r433j86m2@tbd.hendiao.org----9p9f3csgc----la227c7h@outlook.com`

	proxyURLStr := "http://127.0.0.1:7890" // 使用本地代理进行测试
	modelName := "gemini-1.5-flash"        // 使用一个快速的模型进行测试
	prompt := "hi"                         // 简单的测试提示

	lines := strings.Split(keysRaw, "\n")
	var allKeys []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "----")
		if len(parts) > 0 {
			key := strings.TrimSpace(parts[0])
			if key != "" {
				allKeys = append(allKeys, key)
			}
		}
	}

	t.Logf("总共找到 %d 个 Key，开始逐个测试发送消息...", len(allKeys))

	var workingKeys []string
	var failedKeys []string

	// --- 配置 HTTP Client 使用代理 (只需配置一次) ---
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		t.Fatalf("解析代理URL失败: %v", err)
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // 较短的连接超时
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // 整体请求超时
	}
	// --- HTTP Client 配置结束 ---

	for i, apiKey := range allKeys {
		t.Logf("--- 测试 Key %d/%d: %s ---", i+1, len(allKeys), apiKey)
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second) // 为每个 key 的测试设置单独的超时

		var client *genai.Client
		var models *genai.Models

		// --- 创建 genai Client --- 使用与 NewGeminiAgent 类似的方式
		clientConfig := &genai.ClientConfig{
			APIKey:     apiKey,
			HTTPClient: httpClient, // 复用上面创建的带代理的 http client
		}
		client, err = genai.NewClient(ctx, clientConfig)
		if err != nil {
			t.Logf("Key %s 创建 genai 客户端失败: %v", apiKey, err)
			failedKeys = append(failedKeys, apiKey)
			cancel() // 取消上下文
			continue // 继续测试下一个 key
		}
		models = client.Models
		// --- genai Client 创建结束 ---

		// --- 发送 GenerateContent 请求 ---
		contents := []*genai.Content{
			{
				Parts: []*genai.Part{genai.NewPartFromText(prompt)},
				Role:  "user",
			},
		}
		resp, err := models.GenerateContent(ctx, modelName, contents, nil)

		// --- 检查响应 ---
		if err != nil {
			t.Logf("Key %s 调用 GenerateContent 失败: %v", apiKey, err)
			failedKeys = append(failedKeys, apiKey)
		} else if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
			t.Logf("Key %s 返回了空的响应或候选内容", apiKey)
			failedKeys = append(failedKeys, apiKey)
		} else {
			// 尝试提取文本，确认响应有效
			part := resp.Candidates[0].Content.Parts[0]
			if part.Text == "" {
				t.Logf("Key %s 响应的第一个部分不是有效文本", apiKey)
				failedKeys = append(failedKeys, apiKey)
			} else {
				t.Logf("Key %s 测试成功！收到回复片段: %s", apiKey, part.Text)
				workingKeys = append(workingKeys, apiKey)
			}
		}
		// --- 响应检查结束 ---

		// client.Close() // genai SDK 似乎不需要显式 Close
		cancel() // 释放为本次 key 测试创建的上下文
	}

	t.Logf("--- 测试完成 ---")
	t.Logf("发送消息成功的 Keys (%d): %s", len(workingKeys), strings.Join(workingKeys, ","))
	t.Logf("发送消息失败/无有效回复的 Keys (%d): %s", len(failedKeys), strings.Join(failedKeys, ","))

	// 断言：至少要有一个 key 是有效的
	if len(workingKeys) == 0 && len(allKeys) > 0 {
		t.Errorf("所有 %d 个 Key 都未能成功发送消息并获得有效回复", len(allKeys))
	}
}
