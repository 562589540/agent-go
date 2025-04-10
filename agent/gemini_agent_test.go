package agent

import (
	"context"
	"fmt"
	"testing"
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
	apiKey := "AIzaSyBlqIMp0iRkU66zyk-tozMAxmnD1GWT7uY"
	proxyURL := "http://127.0.0.1:7890"

	// 配置代理
	config := AgentConfig{
		APIKey:      apiKey,
		ProxyURL:    proxyURL,
		Debug:       true,
		MaxLoops:    5, // 增加循环次数，因为搜索可能需要多次交互
		MaxTokens:   2000,
		Temperature: 0.7,
		TopP:        0.9,
	}

	// 创建代理
	agent, err := NewGeminiAgent(config)
	if err != nil {
		t.Fatalf("创建GeminiAgent失败: %v", err)
	}

	// 创建配置
	searchConfig := GoogleSearchConfig{
		APIKey:         "AIzaSyByFckwiCTv6DvlL2cfvOmPwWXhGJmYNYI",
		SearchEngineID: "c28d9acdf00c6418e",
		ProxyURL:       "http://127.0.0.1:7890", // 可选
	}

	// 向Agent注册工具
	err = RegisterGoogleSearchTool(agent, searchConfig)
	if err != nil {
		t.Fatalf("注册谷歌搜索工具失败: %v", err)
	}

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
