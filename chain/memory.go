package chain

import (
	"context"
	"fmt"
	"sync"
)

// 内存存储的键值对类型
type MemoryVariables map[string]interface{}

// BaseMemory 所有记忆组件的基础实现
type BaseMemory struct {
	// 记忆中的输入键
	MemoryKeys []string
	// 需要从输入保存到记忆的键
	InputKeys []string
	// 需要从输出保存到记忆的键
	OutputKeys []string
	// 防止并发访问的互斥锁
	mu sync.RWMutex
}

// SimpleMemory 简单的内存实现，将对话历史保存在内存中
type SimpleMemory struct {
	BaseMemory
	// 内存存储
	Variables MemoryVariables
}

// NewSimpleMemory 创建新的简单内存组件
func NewSimpleMemory() *SimpleMemory {
	return &SimpleMemory{
		BaseMemory: BaseMemory{
			MemoryKeys: []string{},
			InputKeys:  []string{},
			OutputKeys: []string{},
		},
		Variables: make(MemoryVariables),
	}
}

// LoadMemory 从内存中加载变量到输入
func (m *SimpleMemory) LoadMemory(ctx context.Context, input ChainInput) (ChainInput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 创建新的输入，包含原始输入
	newInput := make(ChainInput)
	for k, v := range input {
		newInput[k] = v
	}

	// 添加内存中的变量
	for k, v := range m.Variables {
		// 如果输入中没有该键，则添加
		if _, exists := newInput[k]; !exists {
			newInput[k] = v
		}
	}

	return newInput, nil
}

// SaveContext 保存当前上下文到内存
func (m *SimpleMemory) SaveContext(ctx context.Context, input ChainInput, output ChainOutput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保存指定的输入键
	for _, key := range m.InputKeys {
		if value, exists := input[key]; exists {
			m.Variables[key] = value
		}
	}

	// 保存指定的输出键
	for _, key := range m.OutputKeys {
		if value, exists := output[key]; exists {
			m.Variables[key] = value
		}
	}

	return nil
}

// Clear 清除内存
func (m *SimpleMemory) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Variables = make(MemoryVariables)
	return nil
}

// ConversationMemory 对话历史内存，专门存储对话历史
type ConversationMemory struct {
	SimpleMemory

	// 对话历史
	History []map[string]string

	// 对话中人类消息的键
	HumanMessageKey string

	// 对话中AI回复的键
	AIMessageKey string

	// 最大保存的对话轮数，0表示不限制
	MaxTurns int
}

// NewConversationMemory 创建对话历史内存
func NewConversationMemory(humanKey, aiKey string, maxTurns int) *ConversationMemory {
	memory := &ConversationMemory{
		SimpleMemory:    *NewSimpleMemory(),
		History:         []map[string]string{},
		HumanMessageKey: humanKey,
		AIMessageKey:    aiKey,
		MaxTurns:        maxTurns,
	}

	// 设置需要保存的输入输出键
	memory.InputKeys = []string{humanKey}
	memory.OutputKeys = []string{aiKey}

	return memory
}

// GetConversationHistory 获取格式化的对话历史
func (m *ConversationMemory) GetConversationHistory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var history string
	for _, turn := range m.History {
		if human, exists := turn[m.HumanMessageKey]; exists {
			history += fmt.Sprintf("人类: %s\n", human)
		}
		if ai, exists := turn[m.AIMessageKey]; exists {
			history += fmt.Sprintf("AI: %s\n", ai)
		}
	}

	return history
}

// SaveContext 保存对话到历史中
func (m *ConversationMemory) SaveContext(ctx context.Context, input ChainInput, output ChainOutput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建新的对话轮次
	turn := make(map[string]string)

	// 保存人类消息
	if humanMsg, exists := input[m.HumanMessageKey]; exists {
		if msgStr, ok := humanMsg.(string); ok {
			turn[m.HumanMessageKey] = msgStr
		}
	}

	// 保存AI回复
	if aiMsg, exists := output[m.AIMessageKey]; exists {
		if msgStr, ok := aiMsg.(string); ok {
			turn[m.AIMessageKey] = msgStr
		}
	}

	// 只有当轮次包含至少一条消息时才添加到历史
	if len(turn) > 0 {
		m.History = append(m.History, turn)

		// 如果设置了最大轮数且当前轮数超过限制，则删除最早的轮次
		if m.MaxTurns > 0 && len(m.History) > m.MaxTurns {
			m.History = m.History[len(m.History)-m.MaxTurns:]
		}
	}

	// 将格式化的对话历史添加到变量中
	m.Variables["conversation_history"] = m.GetConversationHistory()

	return nil
}

// Clear 清除对话历史
func (m *ConversationMemory) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.History = []map[string]string{}
	m.Variables = make(MemoryVariables)

	return nil
}
