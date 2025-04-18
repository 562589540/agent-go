package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
