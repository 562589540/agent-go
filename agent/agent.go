package agent

import (
	"context"
	"fmt"
	"sync"
)

// 通用消息 不存储也不转换工具消息 只记录对话
type ChatMessage struct {
	Role    string `json:"role"`    //预设 标准open格式 其他库进行适配
	Content string `json:"content"` //输出
}

// FunctionDefinitionParam 通用定义函数参数
type FunctionDefinitionParam struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolFunction 定义一个可以被执行的工具函数
type ToolFunction func(args map[string]interface{}) (string, error)

// Tool 定义工具及其处理函数
type Tool struct {
	Function FunctionDefinitionParam // 函数定义
	Handler  ToolFunction            // 处理函数
}

// StreamHandler 流式消息回调函数定义
type StreamHandler func(text string)

// TokenUsage token使用统计
type TokenUsage struct {
	TotalTokens      int `json:"total_tokens"`      // 总消耗
	PromptTokens     int `json:"prompt_tokens"`     // 提示词消耗
	CompletionTokens int `json:"completion_tokens"` // 完成消耗(响应消耗)
	CacheTokens      int `json:"cache_tokens"`      // 缓存命中
}

// AgentConfig 通用agent配置
type AgentConfig struct {
	APIKey   string // API密钥
	BaseURL  string // 基础URL
	ProxyURL string // 代理URL
	Debug    bool   // 调试模式

	// 模型参数
	MaxTokens   int64   // 最大生成令牌数
	Temperature float64 // 温度参数，控制随机性，默认为0.7
	TopP        float64 // 采样阈值，控制输出多样性，默认为1.0

	// 安全控制
	MaxLoops int // 最大对话循环次数，防止AI递归，默认为5
}

// agent接口
type Agent interface {
	StreamRunConversation(
		ctx context.Context, //上下文
		modelName string, //模型名称
		history []ChatMessage, //如果要保存系统指令和user提示词直接在history中添加
		handler StreamHandler, //流式消息回调
	) (*TokenUsage, error) //返回token使用统计
	RegisterTool(function FunctionDefinitionParam, handler ToolFunction) error //注册工具
	SetDebug(debug bool)                                                       //设置调试模式
}

type AgentName string

const (
	OpenAI AgentName = "openAI"
	Gemini AgentName = "gemini"
)

type AgentService struct {
	AgentMap map[AgentName]Agent
	ctx      context.Context
	lock     sync.RWMutex
}

func NewAgentService(ctx context.Context) *AgentService {
	return &AgentService{
		AgentMap: make(map[AgentName]Agent),
		ctx:      ctx,
	}
}

// 注册agent
func (s *AgentService) RegisterAgent(name AgentName, agent Agent) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.AgentMap[name] = agent
}

// 获取指定的agent
func (s *AgentService) GetAgent(name AgentName) (Agent, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	agent, ok := s.AgentMap[name]
	if ok {
		return agent, nil
	}
	return nil, fmt.Errorf("agent %s not found", name)
}

func (s *AgentService) StreamRunConversation(
	ctx context.Context, //上下文
	agentName AgentName, //agent名称
	modelName string, //模型名称
	history []ChatMessage, //如果要保存系统指令和user提示词直接在history中添加
	handler StreamHandler, //流式消息回调
) (*TokenUsage, error) { //返回token使用统计
	agent, err := s.GetAgent(agentName)
	if err != nil {
		return nil, err
	}
	return agent.StreamRunConversation(ctx, modelName, history, handler)
}
