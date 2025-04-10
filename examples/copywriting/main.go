package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/562589540/agent-go/agent"
	"github.com/562589540/agent-go/chain"
)

// 模拟向量数据库中的数据
type KnowledgeEntry struct {
	ID       string   `json:"id"`
	Content  string   `json:"content"`
	Type     string   `json:"type"`
	Tags     []string `json:"tags"`
	Examples []string `json:"examples,omitempty"`
}

// 模拟向量数据库
var knowledgeBase = []KnowledgeEntry{
	{
		ID:      "formula_001",
		Type:    "公式",
		Content: "PAS公式：问题(Problem) -> 加剧(Agitate) -> 解决方案(Solution)",
		Tags:    []string{"销售文案", "营销", "说服力"},
		Examples: []string{
			"你的皮肤问题让你困扰吗？(问题) 随着年龄增长，这些问题只会越来越严重。(加剧) 我们的产品使用天然成分，能有效改善肌肤状况。(解决方案)",
		},
	},
	{
		ID:      "formula_002",
		Type:    "公式",
		Content: "AIDA公式：注意(Attention) -> 兴趣(Interest) -> 欲望(Desire) -> 行动(Action)",
		Tags:    []string{"广告文案", "营销", "转化率"},
		Examples: []string{
			"限时特惠！(注意) 我们的产品比市场同类产品效果提升30%。(兴趣) 想象一下使用后的完美效果。(欲望) 立即购买，享受折扣！(行动)",
		},
	},
	{
		ID:      "technique_001",
		Type:    "技巧",
		Content: "情感诉求：通过触发情感反应来增强文案效果，如恐惧、快乐、归属感等",
		Tags:    []string{"心理学", "情感营销"},
	},
	{
		ID:      "technique_002",
		Type:    "技巧",
		Content: "稀缺性原则：强调时间或数量的限制，增加紧迫感，如'限时优惠'、'仅剩5件'",
		Tags:    []string{"心理学", "营销策略", "紧迫感"},
	},
	{
		ID:      "style_001",
		Type:    "风格",
		Content: "简洁明了：简短句子，使用主动语态，避免复杂词汇",
		Tags:    []string{"写作风格", "清晰度"},
	},
	{
		ID:      "style_002",
		Type:    "风格",
		Content: "故事叙述：通过讲述故事来建立共鸣和吸引读者",
		Tags:    []string{"写作风格", "叙事", "共鸣"},
	},
	{
		ID:      "industry_tech",
		Type:    "行业",
		Content: "技术产品文案：强调创新性、技术优势和解决方案，使用数据支持论点",
		Tags:    []string{"技术", "B2B", "产品"},
	},
	{
		ID:      "industry_beauty",
		Type:    "行业",
		Content: "美妆产品文案：强调效果、感官体验和美丽转变，使用生动的描述性语言",
		Tags:    []string{"美妆", "B2C", "产品"},
	},
}

// 模拟向量数据库搜索
func searchKnowledgeBase(query string, limit int) []KnowledgeEntry {
	// 在实际应用中，这里应该是向量相似度搜索
	// 这里我们使用简单的关键词匹配来模拟
	var results []KnowledgeEntry
	query = strings.ToLower(query)

	for _, entry := range knowledgeBase {
		// 检查标题或内容是否包含查询词
		if strings.Contains(strings.ToLower(entry.Content), query) {
			results = append(results, entry)
			continue
		}

		// 检查标签是否匹配
		for _, tag := range entry.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, entry)
				break
			}
		}

		// 如果已经找到足够的结果，就停止搜索
		if len(results) >= limit {
			break
		}
	}

	return results
}

// 模拟向量搜索工具
func vectorSearchTool(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("查询字符串不能为空")
	}

	limit := 3 // 默认返回3条结果
	if limitVal, ok := args["limit"].(float64); ok {
		limit = int(limitVal)
	}

	results := searchKnowledgeBase(query, limit)
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}

	return string(jsonData), nil
}

func main() {
	ctx := context.Background()

	// 1. 创建代理服务
	agentService := agent.NewAgentService(ctx)

	// 创建OpenAI代理配置
	config := agent.AgentConfig{
		APIKey:      "sk-ofURrnxSMwFonqGpBd4aA1A1DaC64d97A3015c88E61d5411",
		BaseURL:     "https://api.vveai.com/v1",
		ProxyURL:    "",
		Debug:       true,
		ModelName:   "gpt-4o-mini",
		MaxTokens:   4000,
		Temperature: 0.7,
		MaxLoops:    10,
	}

	// 创建OpenAI代理
	openaiAgent, err := agent.NewOpenAIAgent(config)
	if err != nil {
		fmt.Printf("创建OpenAI代理失败: %v\n", err)
		return
	}

	// 注册代理
	agentService.RegisterAgent(agent.OpenAI, openaiAgent)

	// 2. 注册向量搜索工具
	vectorSearchDef := agent.FunctionDefinitionParam{
		Name:        "vector_search",
		Description: "搜索文案知识库，查找相关的文案公式、技巧和风格",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索查询，可以是关键词、行业、类型等",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "返回结果的数量限制",
				},
			},
			"required": []string{"query"},
		},
	}

	err = openaiAgent.RegisterTool(vectorSearchDef, vectorSearchTool)
	if err != nil {
		log.Fatalf("注册向量搜索工具失败: %v", err)
	}

	// 3. 创建文案分析和知识检索链
	analysisTemplateStr := `你是一位文案专家和分析师。请分析以下文案，并从知识库中检索相关的文案知识。

原始文案: {{.original_text}}

目标: {{.target}}

请执行以下任务:
1. 分析原始文案的风格、类型和行业特点
2. 确定适合的文案公式和技巧
3. 使用vector_search工具从知识库中检索相关知识
4. 根据检索结果，推荐最适合的改写方向和策略

确保你的分析全面且有针对性，以便为后续的文案改写提供充分指导。`

	analysisTemplate, err := chain.NewPromptTemplate(
		analysisTemplateStr,
		[]string{"original_text", "target"},
	)
	if err != nil {
		log.Fatalf("创建分析模板失败: %v", err)
	}

	analysisChain := chain.NewLLMChain(
		"文案分析与知识检索",
		analysisTemplate,
		agentService,
		agent.OpenAI,
		"gpt-4o-mini",
		nil,
	)

	// 4. 创建文案改写链
	rewriteTemplateStr := `你是一位专业文案撰写人。请根据以下信息改写原始文案。

原始文案: {{.original_text}}

目标: {{.target}}

分析与知识检索结果: {{.result}}

请按照分析中推荐的方向和策略，结合检索到的知识，对原始文案进行改写。确保改写后的文案:
1. 更符合目标需求
2. 应用适当的文案公式和技巧
3. 保持行业相关性和专业性
4. 语言流畅、有吸引力

请严格按照以下格式输出:

rewritten_text: [改写后的文案内容]

techniques: [使用的文案技巧和公式]

comparison: [改写前后的对比分析]

确保每个部分使用上述确切的标签，并在标签后使用冒号，这对于后续处理非常重要。`

	rewriteTemplate, err := chain.NewPromptTemplate(
		rewriteTemplateStr,
		[]string{"original_text", "target", "result"},
	)
	if err != nil {
		log.Fatalf("创建改写模板失败: %v", err)
	}

	// 创建结构化输出解析器
	outputSchema := map[string]string{
		"rewritten_text": "改写后的文案内容",
		"techniques":     "使用的文案技巧和公式",
		"comparison":     "改写前后的对比分析",
	}
	outputParser := chain.NewStructuredOutputParser(outputSchema)

	rewriteChain := chain.NewLLMChain(
		"文案改写",
		rewriteTemplate,
		agentService,
		agent.OpenAI,
		"gpt-4o-mini",
		outputParser,
	)

	// 5. 创建顺序链，组合两个链
	copywritingChain := chain.NewSequentialChain(
		"文案改写流程",
		[]chain.Chain{analysisChain, rewriteChain},
	)

	// 6. 运行链
	fmt.Println("=== 文案改写系统 ===")
	fmt.Println("正在处理文案改写...")

	// 示例文案
	originalText := "我们的软件可以帮助企业提升效率。它具有多种功能，使用简单，价格合理。现在就联系我们了解更多信息。"
	target := "增加紧迫感，提高转化率"

	input := chain.ChainInput{
		"original_text": originalText,
		"target":        target,
	}

	// 创建记忆组件
	memory := chain.NewConversationMemory("original_text", "result", 5)
	copywritingChain.Memory = memory

	output, err := copywritingChain.Run(ctx, input)
	if err != nil {
		log.Fatalf("运行文案改写链失败: %v", err)
	}

	// 7. 打印结果
	fmt.Println("\n=== 文案改写结果 ===")
	fmt.Println("\n原始文案:")
	fmt.Println(originalText)
	fmt.Println("\n改写目标:")
	fmt.Println(target)
	fmt.Println("\n改写结果:")

	// 修复结果展示问题
	if result, ok := output["result"].(string); ok {
		// 如果result是字符串格式，直接输出
		fmt.Println(result)
	} else {
		// 处理结构化输出
		fmt.Println("\n改写后文案:")
		if rewrittenText, ok := output["rewritten_text"]; ok {
			fmt.Println(rewrittenText)
		}

		fmt.Println("\n使用的技巧:")
		if techniques, ok := output["techniques"]; ok {
			fmt.Println(techniques)
		}

		fmt.Println("\n对比分析:")
		if comparison, ok := output["comparison"]; ok {
			fmt.Println(comparison)
		}
	}
}
