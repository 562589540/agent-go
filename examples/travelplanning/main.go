package main

import (
	"context"
	"fmt"
	"log"

	"github.com/562589540/agent-go/agent"
	"github.com/562589540/agent-go/chain"
	"github.com/562589540/agent-go/toolgen"
	"github.com/openai/openai-go"
)

// 定义一个简单查询天气的工具
type WeatherQueryParams struct {
	City string `json:"city" description:"要查询天气的城市名称"`
}

// 天气查询示例实现
func queryWeather(params WeatherQueryParams) (string, error) {
	// 这里应该是实际的天气API调用，这里只是模拟
	if params.City == "" {
		return "", fmt.Errorf("城市名不能为空")
	}

	// 模拟返回天气信息
	return fmt.Sprintf("%s今天天气晴朗，温度25°C，适合外出活动。", params.City), nil
}

// 定义一个旅行建议工具
type TravelRecommendationParams struct {
	Destination string `json:"destination" description:"目的地城市"`
	Duration    int    `json:"duration" description:"旅行天数"`
	Season      string `json:"season" description:"旅行季节，例如：春、夏、秋、冬"`
}

// 旅行建议生成器示例实现
func generateTravelRecommendation(params TravelRecommendationParams) (map[string]interface{}, error) {
	// 这里应该是复杂的推荐算法，这里只是模拟
	if params.Destination == "" {
		return nil, fmt.Errorf("目的地不能为空")
	}

	recommendation := map[string]interface{}{
		"destination": params.Destination,
		"duration":    params.Duration,
		"activities":  []string{"参观博物馆", "品尝当地美食", "城市观光"},
		"hotels":      []string{"和平酒店", "星光大酒店"},
		"estimated_cost": map[string]interface{}{
			"hotel":          params.Duration * 300,
			"food":           params.Duration * 200,
			"transportation": 500,
			"activities":     params.Duration * 100,
		},
	}

	return recommendation, nil
}

func main() {
	ctx := context.Background()

	// 1. 创建代理服务
	agentService := agent.NewAgentService(ctx)

	// 创建OpenAI代理配置
	config := agent.AgentConfig{
		// 配置您的API密钥和URL
		APIKey:      "sk-ofURrnxSMwFonqGpBd4aA1A1DaC64d97A3015c88E61d5411", // 从环境变量获取API密钥
		BaseURL:     "https://api.vveai.com/v1",                            // 使用国内可访问的API
		ProxyURL:    "",                                                    // 如果需要代理，请设置代理URL
		Debug:       true,                                                  // 启用调试模式
		ModelName:   openai.ChatModelGPT4oMini2024_07_18,                   // 使用GPT-4o模型
		MaxTokens:   4000,                                                  // 最大生成令牌数
		Temperature: 0.7,                                                   // 温度参数
		MaxLoops:    10,                                                    // 最大对话循环次数
	}

	// 创建OpenAI代理
	openaiAgent, err := agent.NewOpenAIAgent(config)
	if err != nil {
		fmt.Printf("创建OpenAI代理失败: %v\n", err)
		return
	}

	// 注册代理
	agentService.RegisterAgent(agent.OpenAI, openaiAgent)

	// 2. 注册工具
	toolRegistry := toolgen.NewToolRegistry(openaiAgent)

	// 注册天气查询工具
	err = toolgen.RegisterSimpleTool(toolRegistry, "query_weather", "查询指定城市的天气情况", queryWeather)
	if err != nil {
		log.Fatalf("注册天气查询工具失败: %v", err)
	}

	// 注册旅行建议工具
	err = toolgen.RegisterTool(toolRegistry, "travel_recommendation", "根据目的地和旅行时间生成旅行建议", generateTravelRecommendation)
	if err != nil {
		log.Fatalf("注册旅行建议工具失败: %v", err)
	}

	// 3. 创建输出解析器
	outputSchema := map[string]string{
		"plan":           "详细的旅行计划",
		"estimated_cost": "预估总花费",
		"tips":           "旅行小贴士",
	}
	outputParser := chain.NewStructuredOutputParser(outputSchema)

	// fmt.Println("=== 单链示例 ===")
	// runSingleChain(ctx, agentService, outputParser)

	fmt.Println("\n\n=== 多链组合示例 ===")
	runMultipleChains(ctx, agentService, outputParser)
}

// 运行单一LLM链示例
func runSingleChain(ctx context.Context, agentService *agent.AgentService, outputParser chain.OutputParser) {
	// 创建旅行规划提示词模板
	travelPlanningTemplateStr := `你是一个旅行规划专家，帮助用户规划完美的旅行。

用户请求: {{.query}}

请使用以下信息帮助规划旅行:
{{if .weather}}
天气信息: {{.weather}}
{{end}}
{{if .recommendation}}
旅行建议: {{.recommendation}}
{{end}}

请根据以上信息提供一个详细的旅行计划。`

	promptTemplate, err := chain.NewPromptTemplate(
		travelPlanningTemplateStr,
		[]string{"query", "weather", "recommendation"},
	)
	if err != nil {
		log.Fatalf("创建提示词模板失败: %v", err)
	}

	// 创建LLM链
	llmChain := chain.NewLLMChain(
		"旅行规划助手",
		promptTemplate,
		agentService,
		agent.OpenAI,
		openai.ChatModelGPT4oMini2024_07_18,
		outputParser,
	)

	// 运行链
	input := chain.ChainInput{
		"query":          "我想去北京旅游3天，现在是夏季",
		"weather":        "北京今天天气晴朗，温度25°C，适合外出活动。",
		"recommendation": nil,
	}

	// 创建一个带记忆的链
	memory := chain.NewConversationMemory("query", "result", 5)
	llmChain.Memory = memory

	fmt.Println("正在运行单一旅行规划链...")
	output, err := llmChain.Run(ctx, input)
	if err != nil {
		log.Fatalf("运行链失败: %v", err)
	}

	// 打印结果
	fmt.Println("\n单链旅行计划:")
	fmt.Println(output["result"])

	// 打印对话历史
	fmt.Println("\n对话历史:")
	conversationMemory := llmChain.Memory.(*chain.ConversationMemory)
	fmt.Println(conversationMemory.GetConversationHistory())
}

// 运行多链组合示例
func runMultipleChains(ctx context.Context, agentService *agent.AgentService, finalOutputParser chain.OutputParser) {
	// 1. 创建天气分析链
	weatherTemplateStr := `你是一位气象专家。请分析以下城市在指定季节的典型天气情况，并提供天气建议。
	
城市: {{.city}}
季节: {{.season}}

请提供:
1. 典型气温范围
2. 降水情况
3. 适合的衣物建议
4. 其他天气相关注意事项`

	weatherTemplate, err := chain.NewPromptTemplate(
		weatherTemplateStr,
		[]string{"city", "season"},
	)
	if err != nil {
		log.Fatalf("创建天气模板失败: %v", err)
	}

	// 创建天气链的简单输出解析器
	weatherChain := chain.NewLLMChain(
		"天气分析",
		weatherTemplate,
		agentService,
		agent.OpenAI,
		openai.ChatModelGPT4oMini2024_07_18,
		nil, // 使用默认解析器
	)

	// 2. 创建景点推荐链
	attractionsTemplateStr := `你是一位旅游专家。请根据以下信息推荐合适的景点:

城市: {{.city}}
游玩天数: {{.duration}}天
天气分析: {{.result}}

请列出:
1. 值得游览的景点(考虑天气情况)
2. 每个景点的特色
3. 景点游玩所需时间
4. 考虑天气因素的景点排序建议`

	attractionsTemplate, err := chain.NewPromptTemplate(
		attractionsTemplateStr,
		[]string{"city", "duration", "result"},
	)
	if err != nil {
		log.Fatalf("创建景点模板失败: %v", err)
	}

	attractionsChain := chain.NewLLMChain(
		"景点推荐",
		attractionsTemplate,
		agentService,
		agent.OpenAI,
		openai.ChatModelGPT4oMini2024_07_18,
		nil,
	)

	// 3. 创建行程规划链
	itineraryTemplateStr := `你是一位行程规划专家。请根据以下信息制定详细的旅行计划:

城市: {{.city}}
游玩天数: {{.duration}}天
景点信息: {{.result}}

请提供:
1. 每日详细行程安排
2. 交通建议
3. 用餐建议
4. 预估费用明细

确保行程合理，考虑景点之间的距离和游览时间。`

	itineraryTemplate, err := chain.NewPromptTemplate(
		itineraryTemplateStr,
		[]string{"city", "duration", "result"},
	)
	if err != nil {
		log.Fatalf("创建行程模板失败: %v", err)
	}

	itineraryChain := chain.NewLLMChain(
		"行程规划",
		itineraryTemplate,
		agentService,
		agent.OpenAI,
		openai.ChatModelGPT4oMini2024_07_18,
		finalOutputParser,
	)

	// 4. 创建顺序链，组合三个链
	travelPlanningChain := chain.NewSequentialChain(
		"完整旅行规划流程",
		[]chain.Chain{weatherChain, attractionsChain, itineraryChain},
	)

	// 5. 运行链
	fmt.Println("正在运行多链组合旅行规划...")
	input := chain.ChainInput{
		"city":     "北京",
		"season":   "夏季",
		"duration": 3,
	}

	// 创建一个带记忆的链
	memory := chain.NewConversationMemory("city", "result", 5)
	travelPlanningChain.Memory = memory

	output, err := travelPlanningChain.Run(ctx, input)
	if err != nil {
		log.Fatalf("运行多链组合失败: %v", err)
	}

	// 打印结果
	fmt.Println("\n多链组合旅行计划:")
	fmt.Println(output["result"])
}
