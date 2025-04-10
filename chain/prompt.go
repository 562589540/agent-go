package chain

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
)

// PromptTemplate 提示词模板，用于生成LLM的提示词
type PromptTemplate struct {
	// 模板字符串
	Template string
	// 模板参数
	InputVariables []string
	// 解析后的模板
	parsedTemplate *template.Template
}

// NewPromptTemplate 创建新的提示词模板
func NewPromptTemplate(templateStr string, inputVariables []string) (*PromptTemplate, error) {
	// 创建模板
	parsedTemplate, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("解析模板失败: %w", err)
	}

	return &PromptTemplate{
		Template:       templateStr,
		InputVariables: inputVariables,
		parsedTemplate: parsedTemplate,
	}, nil
}

// Format 格式化提示词模板，使用给定的值填充变量
func (p *PromptTemplate) Format(values map[string]interface{}) (string, error) {
	// 检查是否提供了所有必要的变量
	for _, v := range p.InputVariables {
		if _, exists := values[v]; !exists {
			return "", fmt.Errorf("缺少必要的变量: %s", v)
		}
	}

	// 执行模板
	var buf bytes.Buffer
	if err := p.parsedTemplate.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("执行模板失败: %w", err)
	}

	return buf.String(), nil
}

// ValidateInputVariables 验证是否提供了所有必要的输入变量
func (p *PromptTemplate) ValidateInputVariables(values map[string]interface{}) error {
	for _, v := range p.InputVariables {
		if _, exists := values[v]; !exists {
			return fmt.Errorf("缺少必要的变量: %s", v)
		}
	}
	return nil
}

// SimplePromptChain 简单的提示词链，使用提示词模板生成提示词并调用LLM
type SimplePromptChain struct {
	BaseChain
	// 提示词模板
	PromptTemplate *PromptTemplate
	// 输出解析器，将LLM输出转换为所需格式
	OutputParser OutputParser
}

// NewSimplePromptChain 创建新的简单提示词链
func NewSimplePromptChain(name string, promptTemplate *PromptTemplate, outputParser OutputParser) *SimplePromptChain {
	if outputParser == nil {
		// 默认使用简单文本解析器
		outputParser = &SimpleOutputParser{}
	}

	return &SimplePromptChain{
		BaseChain: BaseChain{
			Name:       name,
			InputKeys:  promptTemplate.InputVariables,
			OutputKeys: []string{"result"},
		},
		PromptTemplate: promptTemplate,
		OutputParser:   outputParser,
	}
}

// OutputParser 接口定义输出解析器
type OutputParser interface {
	// Parse 解析LLM输出
	Parse(text string) (interface{}, error)
	// GetFormatInstructions 获取格式说明，添加到提示词中
	GetFormatInstructions() string
}

// SimpleOutputParser 简单文本解析器，直接返回文本
type SimpleOutputParser struct{}

// Parse 简单解析，直接返回文本
func (p *SimpleOutputParser) Parse(text string) (interface{}, error) {
	return text, nil
}

// GetFormatInstructions 获取格式说明
func (p *SimpleOutputParser) GetFormatInstructions() string {
	return ""
}

// StructuredOutputParser 结构化输出解析器，将输出解析为结构化数据
type StructuredOutputParser struct {
	// 期望的输出结构
	OutputSchema map[string]string
}

// NewStructuredOutputParser 创建结构化输出解析器
func NewStructuredOutputParser(schema map[string]string) *StructuredOutputParser {
	return &StructuredOutputParser{
		OutputSchema: schema,
	}
}

// Parse 解析结构化输出
func (p *StructuredOutputParser) Parse(text string) (interface{}, error) {
	result := make(map[string]interface{})

	// 按行分割输入文本
	lines := strings.Split(text, "\n")

	// 追踪当前正在处理的键
	var currentKey string

	// 追踪每个键的值
	keyValues := make(map[string][]string)

	// 第一遍：识别所有键和他们的起始行
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		// 检查这一行是否是新的键
		foundKey := false
		for key := range p.OutputSchema {
			// 匹配形如 "key:" 的模式
			prefix := key + ":"
			if strings.HasPrefix(trimmedLine, prefix) {
				currentKey = key
				// 提取这一行中键后面的值（如果有）
				value := strings.TrimSpace(strings.TrimPrefix(trimmedLine, prefix))
				if value != "" {
					keyValues[currentKey] = append(keyValues[currentKey], value)
				}
				foundKey = true
				break
			}
		}

		// 如果不是新键，且我们正在处理某个键，将这行添加到该键的值
		if !foundKey && currentKey != "" {
			keyValues[currentKey] = append(keyValues[currentKey], trimmedLine)
		}
	}

	// 第二遍：将每个键的值合并成一个字符串
	for key, values := range keyValues {
		if len(values) == 0 {
			result[key] = ""
		} else if len(values) == 1 {
			result[key] = values[0]
		} else {
			// 多行内容，用换行符合并
			result[key] = strings.Join(values, "\n")
		}
	}

	// 确保预期的每个键都存在于结果中
	for key := range p.OutputSchema {
		if _, exists := result[key]; !exists {
			result[key] = ""
		}
	}

	return result, nil
}

// GetFormatInstructions 获取格式说明
func (p *StructuredOutputParser) GetFormatInstructions() string {
	var instructions strings.Builder
	instructions.WriteString("请按以下格式返回您的回答:\n\n")

	for key, description := range p.OutputSchema {
		instructions.WriteString(fmt.Sprintf("%s: <%s>\n", key, description))
	}

	return instructions.String()
}

// Run 执行简单提示词链
func (c *SimplePromptChain) Run(ctx context.Context, input ChainInput) (ChainOutput, error) {
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

	// 4. 模拟LLM响应 (在实际应用中，这里应该调用实际的LLM)
	// 这只是一个简化的示例实现
	llmResponse := fmt.Sprintf("这是对于提示词的模拟响应: %s", prompt)

	// 5. 解析LLM响应
	parsedResponse, err := c.OutputParser.Parse(llmResponse)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 6. 准备输出
	output := c.PrepareOutput(input, parsedResponse)

	return output, nil
}
