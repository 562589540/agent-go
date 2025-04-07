package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/562589540/agent-go/agent"
)

func main() {
	fmt.Println("请选择要运行的示例:")
	fmt.Println("1. 网络搜索示例")
	fmt.Println("2. 必应搜索示例")
	fmt.Print("请输入数字(1-2): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	switch input {
	case "1":
		runWebSearchExample()
	case "2":
		runBingSearchExample()
	default:
		fmt.Println("无效的选择")
	}
}

// runWebSearchExample 运行网络搜索示例
func runWebSearchExample() {
	// 创建上下文
	ctx := context.Background()

	// 创建agent服务
	agentService := agent.NewAgentService(ctx)

	// 创建OpenAI代理配置
	config := agent.AgentConfig{
		// 配置您的API密钥和URL
		APIKey:      os.Getenv("OPENAI_API_KEY"), // 从环境变量获取API密钥
		BaseURL:     "https://api.vveai.com/v1",  // 使用国内可访问的API
		ProxyURL:    "",                          // 如果需要代理，请设置代理URL
		Debug:       true,                        // 启用调试模式
		ModelName:   "gpt-4o",                    // 使用GPT-4o模型
		MaxTokens:   4000,                        // 最大生成令牌数
		Temperature: 0.7,                         // 温度参数
		MaxLoops:    10,                          // 最大对话循环次数
	}

	// 创建OpenAI代理
	openaiAgent, err := agent.NewOpenAIAgent(config)
	if err != nil {
		fmt.Printf("创建OpenAI代理失败: %v\n", err)
		return
	}

	// 注册网络搜索工具
	webSearchDef, webSearchHandler := agent.WebSearchTool()
	err = openaiAgent.RegisterTool(webSearchDef, webSearchHandler)
	if err != nil {
		fmt.Printf("注册网络搜索工具失败: %v\n", err)
		return
	}

	// 注册HTTP请求工具
	httpClientDef, httpClientHandler := agent.SimpleHttpClient()
	err = openaiAgent.RegisterTool(httpClientDef, httpClientHandler)
	if err != nil {
		fmt.Printf("注册HTTP请求工具失败: %v\n", err)
		return
	}

	// 注册代理
	agentService.RegisterAgent(agent.OpenAI, openaiAgent)

	// 创建对话历史
	history := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手，能够搜索互联网获取最新信息并访问网页。请尽可能提供准确、有用的回答。",
		},
		{
			Role:    "user",
			Content: "请搜索一下'2023年中国GDP'，并给我简要总结。",
		},
	}

	// 处理流式消息的回调函数
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 运行对话
	usageStats, _, err := agentService.StreamRunConversation(
		ctx,
		agent.OpenAI,
		"gpt-4o",
		history,
		streamHandler,
	)

	if err != nil {
		fmt.Printf("\n运行对话失败: %v\n", err)
		return
	}

	// 打印Token使用统计
	fmt.Printf("\n\nToken使用统计:\n")
	fmt.Printf("总计: %d\n", usageStats.TotalTokens)
	fmt.Printf("提示词: %d\n", usageStats.PromptTokens)
	fmt.Printf("完成词: %d\n", usageStats.CompletionTokens)
	fmt.Printf("缓存命中: %d\n", usageStats.CacheTokens)
}

// runBingSearchExample 运行必应搜索示例
func runBingSearchExample() {
	// 创建上下文
	ctx := context.Background()

	// 创建agent服务
	agentService := agent.NewAgentService(ctx)

	// 创建OpenAI代理配置
	config := agent.AgentConfig{
		// 配置您的API密钥和URL
		APIKey:      os.Getenv("OPENAI_API_KEY"), // 从环境变量获取API密钥
		BaseURL:     "https://api.vveai.com/v1",  // 使用国内可访问的API
		ProxyURL:    "",                          // 如果需要代理，请设置代理URL
		Debug:       true,                        // 启用调试模式
		ModelName:   "gpt-4o",                    // 使用GPT-4o模型
		MaxTokens:   4000,                        // 最大生成令牌数
		Temperature: 0.7,                         // 温度参数
		MaxLoops:    10,                          // 最大对话循环次数
	}

	// 创建OpenAI代理
	openaiAgent, err := agent.NewOpenAIAgent(config)
	if err != nil {
		fmt.Printf("创建OpenAI代理失败: %v\n", err)
		return
	}

	// 注册必应搜索工具
	bingSearchDef, bingSearchHandler := agent.BingSearchTool()
	err = openaiAgent.RegisterTool(bingSearchDef, bingSearchHandler)
	if err != nil {
		fmt.Printf("注册必应搜索工具失败: %v\n", err)
		return
	}

	// 注册代理
	agentService.RegisterAgent(agent.OpenAI, openaiAgent)

	// 创建对话历史
	history := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手，能够使用必应搜索引擎获取互联网信息。请尽可能提供准确、有用的回答。",
		},
		{
			Role:    "user",
			Content: "请使用必应搜索查询'2024年中国经济增长预期'，并给我一个简要分析。",
		},
	}

	// 处理流式消息的回调函数
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 运行对话
	usageStats, _, err := agentService.StreamRunConversation(
		ctx,
		agent.OpenAI,
		"gpt-4o",
		history,
		streamHandler,
	)

	if err != nil {
		fmt.Printf("\n运行对话失败: %v\n", err)
		return
	}

	// 打印Token使用统计
	fmt.Printf("\n\nToken使用统计:\n")
	fmt.Printf("总计: %d\n", usageStats.TotalTokens)
	fmt.Printf("提示词: %d\n", usageStats.PromptTokens)
	fmt.Printf("完成词: %d\n", usageStats.CompletionTokens)
	fmt.Printf("缓存命中: %d\n", usageStats.CacheTokens)
}
