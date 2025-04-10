package chain

import (
	"context"
	"fmt"

	"github.com/562589540/agent-go/agent"
)

// LLMChain 与LLM交互的链
type LLMChain struct {
	BaseChain
	// 提示词模板
	PromptTemplate *PromptTemplate
	// 输出解析器
	OutputParser OutputParser
	// 代理服务
	AgentService *agent.AgentService
	// 使用的代理名称
	AgentName agent.AgentName
	// 使用的模型名称
	ModelName string
}

// NewLLMChain 创建新的LLM链
func NewLLMChain(
	name string,
	promptTemplate *PromptTemplate,
	agentService *agent.AgentService,
	agentName agent.AgentName,
	modelName string,
	outputParser OutputParser,
) *LLMChain {
	if outputParser == nil {
		outputParser = &SimpleOutputParser{}
	}

	return &LLMChain{
		BaseChain: BaseChain{
			Name:       name,
			InputKeys:  promptTemplate.InputVariables,
			OutputKeys: []string{"result"},
		},
		PromptTemplate: promptTemplate,
		OutputParser:   outputParser,
		AgentService:   agentService,
		AgentName:      agentName,
		ModelName:      modelName,
	}
}

// Run 运行LLM链
func (c *LLMChain) Run(ctx context.Context, input ChainInput) (ChainOutput, error) {
	// 1. 验证输入
	if err := c.ValidateInputs(input); err != nil {
		return nil, err
	}

	// 2. 使用提示词模板生成提示词
	prompt, err := c.PromptTemplate.Format(input)
	if err != nil {
		return nil, fmt.Errorf("格式化提示词失败: %w", err)
	}

	// 3. 如果有输出格式说明，添加到提示词中
	if formatInstr := c.OutputParser.GetFormatInstructions(); formatInstr != "" {
		prompt = fmt.Sprintf("%s\n\n%s", prompt, formatInstr)
	}

	// 4. 调用LLM
	var llmResponse string
	var capturedResponse string

	// 创建对话消息
	history := []agent.ChatMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// 流式处理函数，捕获响应
	streamHandler := func(text string) {
		capturedResponse = text
	}

	// 调用代理服务
	_, _, err = c.AgentService.StreamRunConversation(
		ctx,
		c.AgentName,
		c.ModelName,
		history,
		streamHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("调用LLM失败: %w", err)
	}

	llmResponse = capturedResponse

	// 5. 解析LLM响应
	parsedResponse, err := c.OutputParser.Parse(llmResponse)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 6. 准备输出
	output := c.PrepareOutput(input, parsedResponse)

	return output, nil
}
