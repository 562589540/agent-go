# Agent 模块

Agent模块提供了统一的代理接口，用于对接不同的LLM服务提供商，如OpenAI、Google Gemini等，并支持工具函数调用功能。

## 主要接口

### Agent 接口

`Agent` 是核心接口，定义了代理应具备的基本能力：

```go
type Agent interface {
    // 流式执行对话，支持工具调用
    StreamRunConversation(
        ctx context.Context,     // 上下文
        modelName string,        // 模型名称
        history []ChatMessage,   // 对话历史
        handler StreamHandler,   // 流式消息回调
    ) (*TokenUsage, error)       // 返回token使用统计
    
    // 注册工具函数
    RegisterTool(function FunctionDefinitionParam, handler ToolFunction) error
    
    // 设置调试模式
    SetDebug(debug bool)
}
```

### AgentService

`AgentService` 是一个管理多个Agent实例的服务，支持注册和获取不同类型的Agent：

```go
type AgentService struct {
    AgentMap map[AgentName]Agent
    ctx      context.Context
    lock     sync.RWMutex
}
```

主要方法：
- `NewAgentService(ctx context.Context) *AgentService` - 创建新的AgentService
- `RegisterAgent(name AgentName, agent Agent)` - 注册一个Agent实例
- `GetAgent(name AgentName) (Agent, error)` - 获取指定名称的Agent
- `StreamRunConversation(...)` - 使用指定Agent执行对话

## 数据结构

### ChatMessage

```go
type ChatMessage struct {
    Role    string `json:"role"`    // 角色: system, user, assistant, tool
    Content string `json:"content"` // 消息内容
}
```

### FunctionDefinitionParam

定义工具函数的参数：

```go
type FunctionDefinitionParam struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    Parameters  map[string]interface{} `json:"parameters,omitempty"`
}
```

### TokenUsage

Token使用统计：

```go
type TokenUsage struct {
    TotalTokens      int `json:"total_tokens"`      // 总消耗
    PromptTokens     int `json:"prompt_tokens"`     // 提示词消耗
    CompletionTokens int `json:"completion_tokens"` // 完成消耗(响应消耗)
    CacheTokens      int `json:"cache_tokens"`      // 缓存命中
}
```

### AgentConfig

Agent配置参数：

```go
type AgentConfig struct {
    APIKey      string  // API密钥
    BaseURL     string  // 基础URL
    ProxyURL    string  // 代理URL
    Debug       bool    // 调试模式
    MaxTokens   int64   // 最大生成令牌数
    Temperature float64 // 温度参数
    TopP        float64 // 采样阈值
    MaxLoops    int     // 最大对话循环次数
}
```

## 预定义Agent类型

```go
type AgentName string

const (
    OpenAI AgentName = "openAI"
    Gemini AgentName = "gemini"
)
```

## 使用示例

```go
// 创建Agent服务
agentService := agent.NewAgentService(context.Background())

// 创建并注册Gemini代理
geminiAgent, _ := agent.NewGeminiAgent(agent.AgentConfig{
    APIKey:      "your-api-key",
    ProxyURL:    "http://127.0.0.1:7890",
    Debug:       true,
    MaxLoops:    3,
    MaxTokens:   8000,
    Temperature: 0.7,
    TopP:        1,
})
agentService.RegisterAgent(agent.Gemini, geminiAgent)

// 注册工具函数
geminiAgent.RegisterTool(
    agent.FunctionDefinitionParam{
        Name:        "get_weather",
        Description: "获取指定城市的天气信息",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "city": map[string]interface{}{
                    "type":        "string",
                    "description": "城市名称，如'北京'",
                },
            },
            "required": []string{"city"},
        },
    },
    func(args map[string]interface{}) (string, error) {
        city := args["city"].(string)
        return fmt.Sprintf("%s今日天气: 晴朗，25°C", city), nil
    },
)

// 执行对话
messages := []agent.ChatMessage{
    {Role: "user", Content: "北京今天天气怎么样？"},
}

// 流式回调
handler := func(text string) {
    fmt.Print(text)
}

// 调用对话
tokenUsage, err := agentService.StreamRunConversation(
    context.Background(),
    agent.Gemini,
    "gemini-pro",
    messages,
    handler,
)
``` 