package main

import (
	"context"
	"fmt"

	"log"
	"math"
	"strings"
	"time"

	"github.com/562589540/agent-go/agent"
	"github.com/562589540/agent-go/toolgen"
)

func main() {
	agentService := agent.NewAgentService(context.Background())

	openaiAgent, err := agent.NewOpenAIAgent(agent.AgentConfig{
		APIKey:      "sk-ofURrnxSMwFonqGpBd4aA1A1DaC64d97A3015c88E61d5411",
		BaseURL:     "https://api.vveai.com/v1",
		ProxyURL:    "", // 如果有代理，从环境变量获取
		Debug:       true,
		MaxLoops:    10,
		MaxTokens:   8000,
		Temperature: 0.7,
		TopP:        1,
	})
	if err != nil {
		log.Fatalf("failed to create openai agent: %v", err)
	}
	// 注册工具函数到OpenAI代理
	registerTools(openaiAgent)
	agentService.RegisterAgent(agent.OpenAI, openaiAgent)

	geminiAgent, err := agent.NewGeminiAgent(agent.AgentConfig{
		APIKey:      "AIzaSyBlqIMp0iRkU66zyk-tozMAxmnD1GWT7uY",
		ProxyURL:    "http://127.0.0.1:7890",
		Debug:       true,
		MaxLoops:    3,
		MaxTokens:   8000,
		Temperature: 0.7,
		TopP:        1,
	})
	if err != nil {
		log.Fatalf("failed to create gemini agent: %v", err)
	}
	// 注册工具函数到Gemini代理
	registerTools(geminiAgent)
	// 注册无参数测试工具到Gemini代理
	registerNoParamTools(geminiAgent)
	agentService.RegisterAgent(agent.Gemini, geminiAgent)

	// 测试OpenAI代理
	//testOpenAI(agentService)

	// 测试Gemini代理
	testGemini(agentService)

	// // 测试OpenAI工具函数调用
	// testOpenAITools(agentService)

	// 测试Gemini工具函数调用
	testGeminiTools(agentService)

	// 测试无参数工具
	testNoParamTools(agentService)
}

// 注册无参数测试工具
func registerNoParamTools(agentService agent.Agent) {
	// 创建工具注册器
	registry := toolgen.NewToolRegistry(agentService)

	// 注册获取系统信息的无参数工具
	err := toolgen.RegisterNoParamTool(
		registry,
		"get_system_info",
		"获取系统基本信息，包括运行时间、内存使用等",
		func() (map[string]interface{}, error) {
			// 模拟系统信息
			return map[string]interface{}{
				"hostname":           "server-001",
				"os":                 "Linux",
				"uptime":             "7天3小时15分钟",
				"cpu_usage":          32.5,
				"memory_usage":       68.2,
				"active_connections": 127,
				"timestamp":          time.Now().Format(time.RFC3339),
			}, nil
		},
	)
	if err != nil {
		log.Printf("注册系统信息工具失败: %v", err)
	}

	// 注册获取随机数的无参数工具
	err = toolgen.RegisterNoParamTool(
		registry,
		"get_random_quote",
		"获取一句随机名言",
		func() (struct {
			Quote    string `json:"quote"`
			Author   string `json:"author"`
			Category string `json:"category"`
		}, error) {
			// 模拟随机名言库
			quotes := []struct {
				Quote    string
				Author   string
				Category string
			}{
				{
					Quote:    "知识就是力量",
					Author:   "培根",
					Category: "哲学",
				},
				{
					Quote:    "人生若只如初见",
					Author:   "纳兰性德",
					Category: "文学",
				},
				{
					Quote:    "不积跬步，无以至千里",
					Author:   "荀子",
					Category: "励志",
				},
				{
					Quote:    "业精于勤，荒于嬉",
					Author:   "韩愈",
					Category: "教育",
				},
				{
					Quote:    "千里之行，始于足下",
					Author:   "老子",
					Category: "哲学",
				},
			}

			// 模拟随机选择（固定返回第一个，实际应用中可使用随机数）
			selected := quotes[0]

			return struct {
				Quote    string `json:"quote"`
				Author   string `json:"author"`
				Category string `json:"category"`
			}{
				Quote:    selected.Quote,
				Author:   selected.Author,
				Category: selected.Category,
			}, nil
		},
	)
	if err != nil {
		log.Printf("注册随机名言工具失败: %v", err)
	}

	// 注册简单无参数工具
	err = toolgen.RegisterNoParamSimpleTool(
		registry,
		"get_server_health",
		"获取服务器健康状态的简要描述",
		func() (string, error) {
			// 模拟服务器状态检查
			cpuUsage := 25.7    // 模拟CPU使用率
			memoryUsage := 60.2 // 模拟内存使用率
			diskUsage := 45.8   // 模拟磁盘使用率

			var status string
			var details string

			// 根据使用率评估服务器健康状态
			if cpuUsage < 50 && memoryUsage < 70 && diskUsage < 80 {
				status = "良好"
				details = "所有系统正常运行，资源充足"
			} else if cpuUsage < 80 && memoryUsage < 85 && diskUsage < 90 {
				status = "正常"
				details = "系统负载正常，但建议关注资源使用"
			} else {
				status = "警告"
				details = "系统资源使用率较高，建议检查"
			}

			return fmt.Sprintf("服务器状态: %s\n详细信息: %s\nCPU使用率: %.1f%%\n内存使用率: %.1f%%\n磁盘使用率: %.1f%%",
				status, details, cpuUsage, memoryUsage, diskUsage), nil
		},
	)
	if err != nil {
		log.Printf("注册服务器健康状态工具失败: %v", err)
	}
}

// 测试无参数工具
func testNoParamTools(agentService *agent.AgentService) {
	fmt.Println("\n===== 测试无参数工具 =====")

	// 准备消息
	messages := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手。请用简体中文回答，保持简洁。需要时使用可用的工具函数。",
		},
		{
			Role:    "user",
			Content: "请帮我获取系统信息、服务器健康状态和一句名言。",
		},
	}

	// 创建流式回调
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 执行对话
	ctx := context.Background()
	tokenUsage, err := agentService.StreamRunConversation(
		ctx,
		agent.Gemini,
		"gemini-2.0-flash", // 使用的模型
		messages,
		streamHandler,
	)

	fmt.Println() // 换行

	if err != nil {
		fmt.Printf("无参数工具测试错误: %v\n", err)
		return
	}

	// 打印Token使用情况
	fmt.Printf("\n无参数工具测试 Token使用: 总计=%d, 提示词=%d, 完成=%d\n",
		tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
}

// 注册工具函数到代理
func registerTools(agentService agent.Agent) {
	// 1. 时间查询工具
	timeToolSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "时区，如'Asia/Shanghai'，可为空",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "时间格式，如'2006-01-02'，可为空",
			},
		},
		"required": []string{},
	}

	timeTool := agent.FunctionDefinitionParam{
		Name:        "get_current_time",
		Description: "获取当前时间信息，可指定时区和格式",
		Parameters:  timeToolSchema,
	}

	agentService.RegisterTool(timeTool, func(args map[string]interface{}) (string, error) {
		now := time.Now()
		format, _ := args["format"].(string)
		if format == "" {
			format = "2006-01-02 15:04:05"
		}

		// 获取星期
		weekdays := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
		weekday := weekdays[now.Weekday()]

		return fmt.Sprintf("当前时间: %s %s", now.Format(format), weekday), nil
	})

	// 2. 计算器工具
	calculatorSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{
				"type":        "string",
				"description": "数学表达式，如'3+4*2'",
			},
		},
		"required": []string{"expression"},
	}

	calculatorTool := agent.FunctionDefinitionParam{
		Name:        "calculator",
		Description: "简单数学计算器，支持加减乘除、平方根等基本运算",
		Parameters:  calculatorSchema,
	}

	agentService.RegisterTool(calculatorTool, func(args map[string]interface{}) (string, error) {
		expr, ok := args["expression"].(string)
		if !ok {
			return "", fmt.Errorf("缺少表达式参数")
		}

		// 这里只是模拟简单计算，实际应用中应使用表达式解析库
		// 此处仅处理几种简单情况作为示例
		expr = strings.TrimSpace(expr)

		if strings.Contains(expr, "+") {
			parts := strings.Split(expr, "+")
			if len(parts) == 2 {
				var a, b float64
				fmt.Sscanf(parts[0], "%f", &a)
				fmt.Sscanf(parts[1], "%f", &b)
				return fmt.Sprintf("%.2f + %.2f = %.2f", a, b, a+b), nil
			}
		} else if strings.Contains(expr, "*") {
			parts := strings.Split(expr, "*")
			if len(parts) == 2 {
				var a, b float64
				fmt.Sscanf(parts[0], "%f", &a)
				fmt.Sscanf(parts[1], "%f", &b)
				return fmt.Sprintf("%.2f * %.2f = %.2f", a, b, a*b), nil
			}
		} else if strings.Contains(expr, "sqrt") {
			var num float64
			fmt.Sscanf(expr, "sqrt(%f)", &num)
			return fmt.Sprintf("sqrt(%.2f) = %.4f", num, math.Sqrt(num)), nil
		}

		return fmt.Sprintf("无法计算: %s", expr), nil
	})

	// 3. 天气查询工具
	weatherSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{
				"type":        "string",
				"description": "城市名称，如'北京'",
			},
		},
		"required": []string{"city"},
	}

	weatherTool := agent.FunctionDefinitionParam{
		Name:        "get_weather",
		Description: "获取指定城市的天气信息",
		Parameters:  weatherSchema,
	}

	agentService.RegisterTool(weatherTool, func(args map[string]interface{}) (string, error) {
		city, ok := args["city"].(string)
		if !ok {
			return "", fmt.Errorf("缺少城市参数")
		}

		// 模拟天气数据
		weatherData := map[string]string{
			"北京": "晴朗，25°C，微风",
			"上海": "多云，28°C，微风",
			"广州": "雨，30°C，微风",
			"深圳": "阵雨，31°C，微风",
			"成都": "阴，22°C，无风",
		}

		if weather, found := weatherData[city]; found {
			return fmt.Sprintf("%s今日天气: %s", city, weather), nil
		}

		return fmt.Sprintf("无法获取%s的天气信息，请尝试其他城市"), nil
	})

	// 4. 翻译工具
	translateSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "要翻译的文本",
			},
			"target_language": map[string]interface{}{
				"type":        "string",
				"description": "目标语言，如'english'或'chinese'",
			},
		},
		"required": []string{"text", "target_language"},
	}

	translateTool := agent.FunctionDefinitionParam{
		Name:        "translate",
		Description: "将文本翻译成指定语言",
		Parameters:  translateSchema,
	}

	agentService.RegisterTool(translateTool, func(args map[string]interface{}) (string, error) {
		text, _ := args["text"].(string)
		targetLang, _ := args["target_language"].(string)

		if text == "" || targetLang == "" {
			return "", fmt.Errorf("缺少必要参数")
		}

		// 模拟翻译功能，仅支持几个简单短语
		translations := map[string]map[string]string{
			"你好": {
				"english": "Hello",
				"french":  "Bonjour",
			},
			"谢谢": {
				"english": "Thank you",
				"french":  "Merci",
			},
			"再见": {
				"english": "Goodbye",
				"french":  "Au revoir",
			},
			"Hello": {
				"chinese": "你好",
				"french":  "Bonjour",
			},
			"Thank you": {
				"chinese": "谢谢",
				"french":  "Merci",
			},
		}

		if langMap, ok := translations[text]; ok {
			if translation, found := langMap[strings.ToLower(targetLang)]; found {
				return fmt.Sprintf("翻译结果: %s", translation), nil
			}
		}

		return fmt.Sprintf("无法翻译文本'%s'到'%s'", text, targetLang), nil
	})
}

// 测试OpenAI工具函数调用
func testOpenAITools(agentService *agent.AgentService) {
	fmt.Println("\n===== 测试OpenAI工具函数调用 =====")

	// 准备消息，让AI使用工具
	messages := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手。请用中文简洁回答问题。需要时请使用可用的工具函数。",
		},
		{
			Role:    "user",
			Content: "今天是几号星期几？北京的天气怎么样？另外，计算一下23*45等于多少，并把'你好'翻译成英文。",
		},
	}

	// 创建流式回调
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 执行对话
	ctx := context.Background()
	tokenUsage, err := agentService.StreamRunConversation(
		ctx,
		agent.OpenAI,
		"gpt-4o-mini-2024-07-18", // 使用的模型
		messages,
		streamHandler,
	)

	fmt.Println() // 换行

	if err != nil {
		fmt.Printf("OpenAI工具调用错误: %v\n", err)
		return
	}

	// 打印Token使用情况
	fmt.Printf("\nOpenAI Token使用: 总计=%d, 提示词=%d, 完成=%d\n",
		tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
}

// 测试Gemini工具函数调用
func testGeminiTools(agentService *agent.AgentService) {
	fmt.Println("\n===== 测试Gemini工具函数调用 =====")

	// 准备消息，让AI使用工具
	messages := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手。请用简体中文回答，保持简洁。需要时使用可用的工具函数。",
		},
		{
			Role:    "user",
			Content: "请告诉我现在的时间，上海的天气，计算sqrt(16)的值，并把'谢谢'翻译成法语。",
		},
	}

	// 创建流式回调
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 执行对话
	ctx := context.Background()
	tokenUsage, err := agentService.StreamRunConversation(
		ctx,
		agent.Gemini,
		"gemini-1.5-pro-latest", // 使用的模型
		messages,
		streamHandler,
	)

	fmt.Println() // 换行

	if err != nil {
		fmt.Printf("Gemini工具调用错误: %v\n", err)
		return
	}

	// 打印Token使用情况
	fmt.Printf("\nGemini Token使用: 总计=%d, 提示词=%d, 完成=%d\n",
		tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
}

// 测试OpenAI代理
func testOpenAI(agentService *agent.AgentService) {
	fmt.Println("\n===== 测试OpenAI代理 =====")

	// 准备消息
	messages := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手。请用中文简洁回答问题。",
		},
		{
			Role:    "user",
			Content: "介绍一下自己，不要超过50个字。",
		},
	}

	// 创建流式回调
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 执行对话
	ctx := context.Background()
	tokenUsage, err := agentService.StreamRunConversation(
		ctx,
		agent.OpenAI,
		"gpt-4o-mini-2024-07-18", // 使用的模型
		messages,
		streamHandler,
	)

	fmt.Println() // 换行

	if err != nil {
		fmt.Printf("OpenAI对话错误: %v\n", err)
		return
	}

	// 打印Token使用情况
	fmt.Printf("\nOpenAI Token使用: 总计=%d, 提示词=%d, 完成=%d\n",
		tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
}

// 测试Gemini代理
func testGemini(agentService *agent.AgentService) {
	fmt.Println("\n===== 测试Gemini代理 =====")

	// 准备消息
	messages := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手。请用简体中文回答，保持简洁。",
		},
		{
			Role:    "user",
			Content: "你能告诉我今天的日期吗？并简单介绍自己。",
		},
	}

	// 创建流式回调
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 执行对话
	ctx := context.Background()
	tokenUsage, err := agentService.StreamRunConversation(
		ctx,
		agent.Gemini,
		"gemini-2.0-flash", // 使用的模型
		messages,
		streamHandler,
	)

	fmt.Println() // 换行

	if err != nil {
		fmt.Printf("Gemini对话错误: %v\n", err)
		return
	}

	// 打印Token使用情况
	fmt.Printf("\nGemini Token使用: 总计=%d, 提示词=%d, 完成=%d\n",
		tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
}
