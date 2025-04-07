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
		MaxLoops:    10,
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

	// 测试复杂场景下的工具调用
	registerComplexTools(geminiAgent)
	testComplexScenario(agentService)

	// 测试OpenAI代理
	//testOpenAI(agentService)

	// 测试Gemini代理
	//testGemini(agentService)

	// // 测试OpenAI工具函数调用
	// testOpenAITools(agentService)

	// 测试Gemini工具函数调用
	//testGeminiTools(agentService)

	// 测试无参数工具
	//testNoParamTools(agentService)

	// 测试Gemini在大量历史记录下的工具调用能力
	//testGeminiWithHistory(agentService)
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
	tokenUsage, _, err := agentService.StreamRunConversation(
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
	tokenUsage, _, err := agentService.StreamRunConversation(
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
	tokenUsage, _, err := agentService.StreamRunConversation(
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
	tokenUsage, _, err := agentService.StreamRunConversation(
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
	tokenUsage, _, err := agentService.StreamRunConversation(
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

// 测试Gemini模型在大量历史上下文情况下的工具调用能力
func testGeminiWithHistory(agentService *agent.AgentService) {
	fmt.Println("\n===== 测试Gemini在大量历史记录下的工具调用能力 =====")

	// 准备消息，模拟大量历史对话记录
	messages := []agent.ChatMessage{
		{
			Role:    "system",
			Content: "你是一个有用的AI助手。请用简体中文回答，保持简洁。需要时使用可用的工具函数。",
		},
		// 第一轮对话 - 本应调用翻译工具但没有调用
		{
			Role:    "user",
			Content: "请将'人工智能'翻译成英文",
		},
		{
			Role:    "assistant",
			Content: "人工智能的英文是 Artificial Intelligence，通常缩写为 AI。",
		},
		// 第二轮对话 - 本应调用天气工具但没有调用
		{
			Role:    "user",
			Content: "请问上海今天的天气怎么样？",
		},
		{
			Role:    "assistant",
			Content: "根据最新信息，上海今天多云，气温大约在28°C左右，微风，天气较为舒适。",
		},
		// 第三轮对话 - 本应调用计算器但没有调用
		{
			Role:    "user",
			Content: "帮我计算一下23乘以45等于多少？",
		},
		{
			Role:    "assistant",
			Content: "23乘以45等于1035。",
		},
		// 第四轮对话 - 本应调用时间工具但没有调用
		{
			Role:    "user",
			Content: "现在几点了？",
		},
		{
			Role:    "assistant",
			Content: "作为AI助手，我无法获取实时信息。请查看您的设备时间以获取准确的当前时间。",
		},
		// 第五轮对话 - 需要工具调用的问题
		{
			Role:    "user",
			Content: "谢谢解释。对了，请告诉我现在的时间，顺便查询一下北京的天气。另外，把'谢谢你的帮助'翻译成英文。",
		},
	}

	// 创建流式回调
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 执行对话
	ctx := context.Background()
	tokenUsage, _, err := agentService.StreamRunConversation(
		ctx,
		agent.Gemini,
		"gemini-2.0-flash", // 使用的模型
		messages,
		streamHandler,
	)

	fmt.Println() // 换行

	if err != nil {
		fmt.Printf("Gemini历史记录测试错误: %v\n", err)
		return
	}

	// 打印Token使用情况
	fmt.Printf("\nGemini历史记录测试 Token使用: 总计=%d, 提示词=%d, 完成=%d\n",
		tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
}

// 注册复杂工具函数
func registerComplexTools(agentService agent.Agent) {
	// 创建工具注册器
	registry := toolgen.NewToolRegistry(agentService)

	// 复杂工具1: 数据分析工具（嵌套参数结构）
	type DataFilterCriteria struct {
		Field         string   `json:"field" description:"要筛选的字段名称"`
		Operator      string   `json:"operator" description:"操作符，如 equals, contains, greater_than 等"`
		Value         string   `json:"value" description:"筛选值"`
		CaseSensitive bool     `json:"case_sensitive,omitempty" description:"是否区分大小写（仅适用于文本字段）"`
		ExcludeTerms  []string `json:"exclude_terms,omitempty" description:"排除词列表"`
	}

	type DataGroupBy struct {
		Field string `json:"field" description:"分组字段"`
		Type  string `json:"type" description:"分组类型: sum, avg, count, min, max"`
	}

	type DataAnalysisRequest struct {
		DataSource string `json:"data_source" description:"数据源名称，如'sales_data'、'user_metrics'等"`
		TimeRange  struct {
			Start string `json:"start" description:"开始时间，格式为ISO8601"`
			End   string `json:"end" description:"结束时间，格式为ISO8601"`
		} `json:"time_range" description:"时间范围"`
		Filters      []DataFilterCriteria `json:"filters,omitempty" description:"过滤条件列表"`
		GroupBy      []DataGroupBy        `json:"group_by,omitempty" description:"分组条件列表"`
		Limit        int                  `json:"limit,omitempty" description:"返回结果数量限制"`
		SortBy       string               `json:"sort_by,omitempty" description:"排序字段"`
		SortOrder    string               `json:"sort_order,omitempty" description:"排序方向：asc或desc"`
		OutputFormat string               `json:"output_format,omitempty" description:"输出格式：table或chart"`
	}

	type DataAnalysisResult struct {
		Status      string                   `json:"status"`
		RequestTime string                   `json:"request_time"`
		DataPoints  int                      `json:"data_points"`
		Summary     map[string]float64       `json:"summary"`
		Results     []map[string]interface{} `json:"results"`
		Message     string                   `json:"message"`
	}

	err := toolgen.RegisterTool(
		registry,
		"analyze_complex_data",
		"高级数据分析工具，支持复杂查询、过滤、分组和聚合功能，处理结构化数据。在需要分析复杂数据、找出趋势和模式时使用。",
		func(input DataAnalysisRequest) (DataAnalysisResult, error) {
			// 模拟数据分析过程
			return DataAnalysisResult{
				Status:      "成功",
				RequestTime: time.Now().Format(time.RFC3339),
				DataPoints:  1245,
				Summary: map[string]float64{
					"总计":  1245.78,
					"平均值": 78.34,
					"最大值": 342.56,
					"最小值": 12.45,
					"中位数": 76.5,
					"标准差": 34.2,
				},
				Results: []map[string]interface{}{
					{
						"分类":   "电子产品",
						"销售额":  458.75,
						"销售数量": 15,
						"增长率":  0.23,
					},
					{
						"分类":   "服装",
						"销售额":  327.42,
						"销售数量": 27,
						"增长率":  0.12,
					},
					{
						"分类":   "食品",
						"销售额":  245.33,
						"销售数量": 48,
						"增长率":  0.05,
					},
				},
				Message: "数据分析完成，共处理1245个数据点，发现3个主要趋势。",
			}, nil
		},
	)
	if err != nil {
		log.Printf("注册复杂数据分析工具失败: %v", err)
	}

	// 复杂工具2: 智能文档处理工具
	type DocumentSection struct {
		Title       string            `json:"title" description:"文档章节标题"`
		Content     string            `json:"content" description:"文档章节内容"`
		Metadata    map[string]string `json:"metadata" description:"元数据，如作者、创建日期等"`
		Annotations []struct {
			Type     string `json:"type" description:"注释类型"`
			Text     string `json:"text" description:"注释内容"`
			Position struct {
				Start int `json:"start" description:"开始位置"`
				End   int `json:"end" description:"结束位置"`
			} `json:"position" description:"注释位置"`
		} `json:"annotations,omitempty" description:"文档注释"`
	}

	type DocumentProcessRequest struct {
		DocumentID      string   `json:"document_id" description:"文档ID"`
		ProcessingTasks []string `json:"processing_tasks" description:"处理任务列表：summarize, extract_entities, translate, classify, etc."`
		TargetSections  []string `json:"target_sections,omitempty" description:"目标章节，如为空则处理整个文档"`
		OutputFormat    string   `json:"output_format,omitempty" description:"输出格式：text, json, html, markdown"`
		Parameters      struct {
			SummaryLength        int      `json:"summary_length,omitempty" description:"摘要长度"`
			EntityTypes          []string `json:"entity_types,omitempty" description:"实体类型，如人名、地名、组织机构等"`
			TranslationLanguage  string   `json:"translation_language,omitempty" description:"翻译目标语言"`
			ClassificationLabels []string `json:"classification_labels,omitempty" description:"分类标签"`
		} `json:"parameters,omitempty" description:"处理参数"`
	}

	type DocumentProcessResult struct {
		DocumentID     string              `json:"document_id"`
		ProcessedAt    string              `json:"processed_at"`
		TaskResults    map[string]string   `json:"task_results"`
		Sections       []DocumentSection   `json:"sections,omitempty"`
		Entities       map[string][]string `json:"entities,omitempty"`
		Classification []struct {
			Label      string  `json:"label"`
			Confidence float64 `json:"confidence"`
		} `json:"classification,omitempty"`
		Summary string `json:"summary,omitempty"`
		Message string `json:"message"`
	}

	err = toolgen.RegisterTool(
		registry,
		"process_complex_document",
		"智能文档处理工具，支持文档摘要、实体提取、翻译、分类等多种处理任务。当用户需要处理复杂文档、提取关键信息或进行内容分析时使用。",
		func(input DocumentProcessRequest) (DocumentProcessResult, error) {
			// 模拟文档处理结果
			return DocumentProcessResult{
				DocumentID:  input.DocumentID,
				ProcessedAt: time.Now().Format(time.RFC3339),
				TaskResults: map[string]string{
					"summarize":        "完成",
					"extract_entities": "完成",
					"classify":         "完成",
				},
				Sections: []DocumentSection{
					{
						Title:   "引言",
						Content: "这是一个示例文档的引言部分，介绍了文档的主要内容和目的。",
						Metadata: map[string]string{
							"author": "张三",
							"date":   "2023-05-15",
						},
					},
					{
						Title:   "方法论",
						Content: "本节详细描述了研究方法和数据收集过程。",
						Metadata: map[string]string{
							"author": "李四",
							"date":   "2023-05-16",
						},
					},
				},
				Entities: map[string][]string{
					"人名": {"张三", "李四", "王五"},
					"地点": {"北京", "上海", "广州"},
					"组织": {"清华大学", "百度", "阿里巴巴"},
				},
				Classification: []struct {
					Label      string  `json:"label"`
					Confidence float64 `json:"confidence"`
				}{
					{Label: "科技", Confidence: 0.85},
					{Label: "教育", Confidence: 0.65},
					{Label: "经济", Confidence: 0.45},
				},
				Summary: "这是一篇关于人工智能在教育领域应用的研究论文，探讨了AI技术如何改进教学方法和学习效果。论文通过三个案例研究，分析了智能辅导系统、自适应学习平台和智能评估工具的实施情况及其效果。研究结果表明，AI技术能够显著提高学生的学习参与度和成绩，特别是在个性化学习方面表现出色。",
				Message: "文档处理完成，成功执行了摘要、实体提取和分类任务。",
			}, nil
		},
	)
	if err != nil {
		log.Printf("注册复杂文档处理工具失败: %v", err)
	}

	// 复杂工具3: 多模态内容生成工具
	type ContentTemplate struct {
		ID          string `json:"id" description:"模板ID"`
		Name        string `json:"name" description:"模板名称"`
		Description string `json:"description" description:"模板描述"`
		Type        string `json:"type" description:"模板类型：article, social_post, email, advertisement, etc."`
	}

	type ContentAsset struct {
		Type        string `json:"type" description:"资源类型：image, video, audio, etc."`
		URL         string `json:"url,omitempty" description:"资源URL"`
		Description string `json:"description,omitempty" description:"资源描述"`
		Position    string `json:"position,omitempty" description:"资源位置：header, body, footer"`
	}

	type ContentGenerationRequest struct {
		ContentType    string `json:"content_type" description:"内容类型：blog, social_media, email, ad, etc."`
		Topic          string `json:"topic" description:"内容主题"`
		TargetAudience struct {
			Demographics struct {
				AgeRange    []int    `json:"age_range,omitempty" description:"目标年龄范围"`
				Gender      []string `json:"gender,omitempty" description:"目标性别"`
				Locations   []string `json:"locations,omitempty" description:"目标地区"`
				Occupations []string `json:"occupations,omitempty" description:"目标职业"`
				Interests   []string `json:"interests,omitempty" description:"目标兴趣"`
			} `json:"demographics" description:"人口统计信息"`
			PsychographicTraits []string `json:"psychographic_traits,omitempty" description:"心理特征"`
		} `json:"target_audience" description:"目标受众"`
		ToneAndStyle struct {
			Tone         string   `json:"tone" description:"语调：professional, casual, humorous, etc."`
			Style        string   `json:"style" description:"风格：informative, persuasive, storytelling, etc."`
			Keywords     []string `json:"keywords,omitempty" description:"关键词列表"`
			AvoidPhrases []string `json:"avoid_phrases,omitempty" description:"避免使用的短语"`
		} `json:"tone_and_style" description:"语调和风格"`
		StructureAndFormat struct {
			Template        string   `json:"template,omitempty" description:"使用的模板ID"`
			Sections        []string `json:"sections,omitempty" description:"内容章节"`
			Length          string   `json:"length" description:"内容长度：short, medium, long"`
			IncludeMetadata bool     `json:"include_metadata,omitempty" description:"是否包含元数据"`
		} `json:"structure_and_format" description:"结构和格式"`
		Assets         []ContentAsset `json:"assets,omitempty" description:"内容资源"`
		Campaign       string         `json:"campaign,omitempty" description:"营销活动名称"`
		PublishingInfo struct {
			Platform    string `json:"platform,omitempty" description:"发布平台"`
			ScheduledAt string `json:"scheduled_at,omitempty" description:"计划发布时间"`
			Frequency   string `json:"frequency,omitempty" description:"发布频率"`
		} `json:"publishing_info,omitempty" description:"发布信息"`
	}

	type ContentGenerationResult struct {
		RequestID   string `json:"request_id"`
		GeneratedAt string `json:"generated_at"`
		ContentType string `json:"content_type"`
		Title       string `json:"title"`
		Content     string `json:"content"`
		Summary     string `json:"summary,omitempty"`
		Keywords    string `json:"keywords,omitempty"`
		Metadata    struct {
			ReadTime         int      `json:"read_time"`
			WordCount        int      `json:"word_count"`
			SEOScore         int      `json:"seo_score"`
			ReadabilityScore int      `json:"readability_score"`
			Tags             []string `json:"tags"`
		} `json:"metadata,omitempty"`
		Assets              []ContentAsset `json:"assets,omitempty"`
		RecommendedHashtags []string       `json:"recommended_hashtags,omitempty"`
		Message             string         `json:"message"`
	}

	err = toolgen.RegisterTool(
		registry,
		"generate_complex_content",
		"高级多模态内容生成工具，支持根据详细规格生成结构化内容。当用户需要创建营销文案、社交媒体帖子、邮件内容或其他专业内容时使用。",
		func(input ContentGenerationRequest) (ContentGenerationResult, error) {
			// 模拟内容生成结果
			return ContentGenerationResult{
				RequestID:   "content-gen-" + fmt.Sprint(time.Now().Unix()),
				GeneratedAt: time.Now().Format(time.RFC3339),
				ContentType: input.ContentType,
				Title:       "人工智能如何改变我们的生活和工作方式",
				Content:     "在过去的十年中，人工智能技术取得了长足的进步，从简单的算法到如今能够模拟人类思维的复杂系统。这一技术革命正在深刻改变我们的生活和工作方式。\n\n首先，在日常生活中，AI已经无处不在。智能手机上的语音助手可以帮助我们设置闹钟、查询天气、播放音乐；智能家居设备可以自动调节室温、照明和安全系统；推荐算法则根据我们的喜好推荐电影、音乐和新闻。\n\n其次，在工作领域，AI正在提高效率和创新能力。自动化工具可以处理重复性任务，让人类专注于更具创造性的工作；数据分析系统可以从海量数据中提取有价值的见解；客服机器人则可以全天候为客户提供服务。\n\n然而，AI的快速发展也带来了挑战。就业市场正在经历结构性变化，某些工作岗位可能会被自动化系统取代；数据隐私和算法偏见等伦理问题需要认真对待；监管框架也需要跟上技术发展的步伐。\n\n展望未来，AI与人类的协作将成为主流模式。我们需要发展新技能，适应AI驱动的经济；建立负责任的AI开发和使用准则；确保技术发展惠及所有人，而不是加剧社会不平等。\n\n总之，人工智能正在以前所未有的方式改变我们的世界。通过理性地拥抱这一技术，制定明智的政策，我们可以确保AI成为推动人类进步的积极力量。",
				Summary:     "本文探讨了人工智能如何改变日常生活和工作方式，分析了AI带来的机遇与挑战，并展望了人机协作的未来发展趋势。",
				Keywords:    "人工智能,技术革命,工作自动化,伦理挑战,未来展望",
				Metadata: struct {
					ReadTime         int      `json:"read_time"`
					WordCount        int      `json:"word_count"`
					SEOScore         int      `json:"seo_score"`
					ReadabilityScore int      `json:"readability_score"`
					Tags             []string `json:"tags"`
				}{
					ReadTime:         5,
					WordCount:        560,
					SEOScore:         85,
					ReadabilityScore: 78,
					Tags:             []string{"AI", "技术", "未来工作", "社会变革"},
				},
				RecommendedHashtags: []string{"#AI革命", "#科技未来", "#工作变革", "#人机协作"},
				Message:             "内容生成成功，已根据目标受众和风格要求优化。",
			}, nil
		},
	)
	if err != nil {
		log.Printf("注册复杂内容生成工具失败: %v", err)
	}
}

// 测试复杂场景
func testComplexScenario(agentService *agent.AgentService) {
	fmt.Println("\n===== 测试复杂场景下的工具调用能力 =====")

	// 准备一段复杂的系统提示词
	complexSystemPrompt := `你是绘影AI，一个极其先进的人工智能助手，专为处理高复杂度任务设计。

【核心指令 - 最高优先级】
在回应用户查询时，你必须始终遵循以下工作流程：
1. 首先分析用户意图，确定需要调用的工具
2. 必须先调用适当的工具获取数据，禁止在没有调用工具的情况下直接回答
3. 根据工具返回的数据制定回应策略
4. 向用户提供基于工具数据的准确、全面的回答

【工具调用协议】
- 每次收到用户查询后，分析后必须优先调用工具获取信息
- 即使你认为可以直接回答，也必须先验证通过工具调用
- 禁止猜测数据，必须通过工具获取实际数据
- 禁止跳过工具调用直接回答

【复杂数据处理规范】
处理复杂数据时，应遵循以下步骤：
1. 首先使用analyze_complex_data工具获取基础数据
2. 根据初步分析结果，确定是否需要调用其他工具进行深入分析
3. 综合多个工具的结果，提供全面分析

【内容生成标准】
生成内容时，必须遵循：
1. 使用process_complex_document工具处理相关文档
2. 使用generate_complex_content工具创建专业内容
3. 确保生成内容适合目标受众，风格一致

【高级交互模式】
- 对复杂问题，采用"分步思考"方法，将问题分解为可管理的子任务
- 每个子任务必须先通过适当工具获取数据后再处理
- 禁止在任何步骤跳过工具验证

【历史警告】
AI记住：在过去的互动中，你曾经直接回答问题而不调用工具获取数据，这导致了严重错误。必须避免重蹈覆辙，每次都必须先调用工具验证。

重要提醒：无论问题看起来多么简单，或者你认为已经知道答案，都必须先通过工具验证。这是确保回答准确性的唯一方法。`

	// 准备模拟历史消息
	historicalMessages := []agent.ChatMessage{
		// {
		// 	Role:    "user",
		// 	Content: "你能分析一下我们最近的销售数据吗？",
		// },
		// {
		// 	Role:    "assistant",
		// 	Content: "我需要先获取您的销售数据才能提供分析。请问您想分析哪个时间段的数据？需要关注哪些特定指标？",
		// },
		// {
		// 	Role:    "user",
		// 	Content: "主要看看最近三个月各产品类别的表现，重点是销售额和增长率。",
		// },
		// {
		// 	Role:    "assistant",
		// 	Content: "为了给您提供准确的分析，我需要调用数据分析工具获取最近三个月的销售数据。请稍等。",
		// },
		// {
		// 	Role:    "user",
		// 	Content: "好的，另外我还想知道我们的内容营销效果如何？",
		// },
		// {
		// 	Role:    "assistant",
		// 	Content: "我会分两部分回答您的问题。首先，我需要获取内容营销相关的数据，然后再进行分析。请稍等我调用相关工具获取信息。",
		// },
	}

	// 准备当前用户消息
	currentMessage := agent.ChatMessage{
		Role:    "user",
		Content: "现在我需要你分析一下我们最近的销售数据和市场趋势，并生成一份适合在管理层会议上展示的报告内容。报告需要包含销售数据分析、市场趋势洞察和未来战略建议。直接使用tool函数获取你需要的数据无需询问我",
	}

	// 构建完整消息
	messages := []agent.ChatMessage{
		{
			Role:    "system",
			Content: complexSystemPrompt,
		},
	}

	// 添加历史消息
	messages = append(messages, historicalMessages...)
	// 添加当前用户消息
	messages = append(messages, currentMessage)

	// 创建流式回调
	streamHandler := func(text string) {
		fmt.Print(text)
	}

	// 执行对话
	ctx := context.Background()
	tokenUsage, _, err := agentService.StreamRunConversation(
		ctx,
		agent.Gemini,
		"gemini-2.0-flash", // 使用的模型
		messages,
		streamHandler,
	)

	fmt.Println() // 换行

	if err != nil {
		fmt.Printf("复杂场景测试错误: %v\n", err)
		return
	}

	// 打印Token使用情况
	fmt.Printf("\n复杂场景测试 Token使用: 总计=%d, 提示词=%d, 完成=%d\n",
		tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
}
