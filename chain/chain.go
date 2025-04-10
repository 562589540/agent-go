package chain

import (
	"context"
	"errors"
	"fmt"
)

// ChainInput 定义链的输入类型
type ChainInput map[string]interface{}

// ChainOutput 定义链的输出类型
type ChainOutput map[string]interface{}

// Chain 定义链接口
type Chain interface {
	// Run 执行链并返回结果
	Run(ctx context.Context, input ChainInput) (ChainOutput, error)
	// GetInputKeys 获取链所需的输入键
	GetInputKeys() []string
	// GetOutputKeys 获取链输出的键
	GetOutputKeys() []string
}

// BaseChain 提供Chain接口的基本实现
type BaseChain struct {
	// 链的名称
	Name string
	// 链所需的输入键
	InputKeys []string
	// 链输出的键
	OutputKeys []string
	// 链的内存组件(可选)
	Memory Memory
}

// GetInputKeys 返回链所需的输入键
func (c *BaseChain) GetInputKeys() []string {
	return c.InputKeys
}

// GetOutputKeys 返回链输出的键
func (c *BaseChain) GetOutputKeys() []string {
	return c.OutputKeys
}

// ValidateInputs 验证输入是否包含所有所需的键
func (c *BaseChain) ValidateInputs(input ChainInput) error {
	for _, key := range c.InputKeys {
		if _, exists := input[key]; !exists {
			return fmt.Errorf("缺少必要的输入键: %s", key)
		}
	}
	return nil
}

// PrepareOutput 准备链的输出
func (c *BaseChain) PrepareOutput(input ChainInput, result interface{}) ChainOutput {
	output := make(ChainOutput)

	// 如果结果是ChainOutput类型，直接使用
	if resultMap, ok := result.(ChainOutput); ok {
		for k, v := range resultMap {
			output[k] = v
		}
	} else if resultMap, ok := result.(map[string]interface{}); ok {
		// 处理来自StructuredOutputParser的结构化输出
		// 将所有结构化字段作为顶级键添加到输出中
		for k, v := range resultMap {
			output[k] = v
		}

		// 同时，将整个结构化输出也放入result键中，以便于链组合时的访问
		if len(c.OutputKeys) > 0 && c.OutputKeys[0] != "" {
			output[c.OutputKeys[0]] = resultMap
		} else {
			output["result"] = resultMap
		}
	} else if resultStr, ok := result.(string); ok {
		// 如果结果是字符串，使用第一个输出键
		if len(c.OutputKeys) > 0 && c.OutputKeys[0] != "" {
			output[c.OutputKeys[0]] = resultStr
		} else {
			output["result"] = resultStr
		}
	} else {
		// 其他类型，保存为result键
		if len(c.OutputKeys) > 0 && c.OutputKeys[0] != "" {
			output[c.OutputKeys[0]] = result
		} else {
			output["result"] = result
		}
	}

	return output
}

// SequentialChain 顺序执行多个链
type SequentialChain struct {
	BaseChain
	// 要顺序执行的链
	Chains []Chain
}

// NewSequentialChain 创建新的顺序链
func NewSequentialChain(name string, chains []Chain) *SequentialChain {
	// 计算输入和输出键
	var inputKeys []string
	var outputKeys []string

	// 第一个链的输入键作为整个顺序链的输入键
	if len(chains) > 0 {
		inputKeys = chains[0].GetInputKeys()
	}

	// 最后一个链的输出键作为整个顺序链的输出键
	if len(chains) > 0 {
		outputKeys = chains[len(chains)-1].GetOutputKeys()
	}

	return &SequentialChain{
		BaseChain: BaseChain{
			Name:       name,
			InputKeys:  inputKeys,
			OutputKeys: outputKeys,
		},
		Chains: chains,
	}
}

// Run 顺序执行所有链
func (c *SequentialChain) Run(ctx context.Context, input ChainInput) (ChainOutput, error) {
	if len(c.Chains) == 0 {
		return nil, errors.New("顺序链中没有链可执行")
	}

	// 验证初始输入
	if err := c.ValidateInputs(input); err != nil {
		return nil, err
	}

	// 当前输入，初始为传入的输入
	currentInput := input

	// 顺序执行每个链
	for i, chain := range c.Chains {
		// 如果有记忆组件，从记忆中加载数据
		if c.Memory != nil {
			var err error
			currentInput, err = c.Memory.LoadMemory(ctx, currentInput)
			if err != nil {
				return nil, fmt.Errorf("从记忆加载数据失败: %w", err)
			}
		}

		output, err := chain.Run(ctx, currentInput)
		if err != nil {
			return nil, fmt.Errorf("执行链 %d 失败: %w", i, err)
		}

		// 如果有记忆组件，保存上下文到记忆
		if c.Memory != nil {
			if err := c.Memory.SaveContext(ctx, currentInput, output); err != nil {
				return nil, fmt.Errorf("保存上下文到记忆失败: %w", err)
			}
		}

		// 将当前链的输出合并到输入中传给下一个链
		for k, v := range output {
			currentInput[k] = v
		}
	}

	// 最终输出
	finalOutput := make(ChainOutput)
	for _, key := range c.OutputKeys {
		if val, exists := currentInput[key]; exists {
			finalOutput[key] = val
		}
	}

	// 如果最终输出为空，可能是因为OutputKeys不匹配，
	// 则尝试包含所有可能的结构化输出键
	if len(finalOutput) == 0 {
		// 添加常见的结构化输出键
		for _, commonKey := range []string{"result", "rewritten_text", "techniques", "comparison"} {
			if val, exists := currentInput[commonKey]; exists {
				finalOutput[commonKey] = val
			}
		}
	}

	return finalOutput, nil
}

// Memory 接口定义记忆组件
type Memory interface {
	// LoadMemory 从记忆中加载值到输入
	LoadMemory(ctx context.Context, input ChainInput) (ChainInput, error)
	// SaveContext 将当前上下文保存到记忆中
	SaveContext(ctx context.Context, input ChainInput, output ChainOutput) error
	// Clear 清除记忆
	Clear() error
}
