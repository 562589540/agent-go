package agent

import (
	"context"
	"testing"
)

// 测试OpenAIAgent基本功能
func TestOpenAIAgent(t *testing.T) {
	// 获取API密钥，如果没有设置跳过测试
	apiKey := "sk-ofURrnxSMwFonqGpBd4aA1A1DaC64d97A3015c88E61d5411"
	baseURL := "https://api.vveai.com/v1"

	// 初始化OpenAI客户端
	// apiKey := "ak-dxv9jbsZfQmTGgkRWi1OQgnqRrLT4gHRdxBdKRpcRqh0eoe3"
	// baseURL := "https://api.nextapi.fun/v1"

	// 配置代理
	config := AgentConfig{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Debug:       true,
		MaxLoops:    3,
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        1,
	}

	// 创建代理
	agent, err := NewOpenAIAgent(config)
	if err != nil {
		t.Fatalf("创建OpenAIAgent失败: %v", err)
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
		return "当前时间是: 2023-08-01 15:30:00", nil
	})

	if err != nil {
		t.Fatalf("注册工具失败: %v", err)
	}

	// 创建测试对话
	ctx := context.Background()
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手，请用中文回答问题。",
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
	tokenUsage, err := agent.StreamRunConversation(ctx, "gpt-4o-mini-2024-07-18", messages, streamHandler)
	if err != nil {
		t.Fatalf("执行对话失败: %v", err)
	}

	// 验证是否有输出
	if len(outputMessages) == 0 {
		t.Error("没有收到任何输出消息")
	}

	// 验证token使用情况
	t.Logf("Token使用情况: %+v", tokenUsage)
}
