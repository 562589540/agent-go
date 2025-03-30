# Toolgen 模块

Toolgen 模块提供了一个用于生成和管理工具函数的框架，简化了工具函数的定义和注册过程，并提供了强类型的参数支持。

## 核心功能

1. **工具函数注册**：提供统一的工具注册机制
2. **参数类型推断**：自动从结构体生成JSON Schema
3. **枚举类型支持**：支持枚举类型的参数定义
4. **无参数工具支持**：支持无需参数的工具函数
5. **类型安全**：强类型参数，编译时检查

## 主要结构体

### ToolRegistry

工具注册中心，管理所有工具的定义和处理函数：

```go
type ToolRegistry struct {
    Tools    []agent.FunctionDefinitionParam
    Handlers map[string]agent.ToolFunction
    Debug    bool
}
```

主要方法：
- `NewToolRegistry() *ToolRegistry`：创建新的工具注册中心
- `RegisterToAgent(a agent.Agent) error`：将所有工具注册到Agent

### EmptyParams

用于表示无参数的工具函数：

```go
type EmptyParams struct{}
```

## 核心函数

### 带参数工具注册

```go
// 注册带参数的工具
func (registry *ToolRegistry) RegisterTool[T any](
    name string,
    description string,
    handler func(T) (string, error),
) error
```

### 无参数工具注册

```go
// 注册无参数工具
func (registry *ToolRegistry) RegisterToolWithoutParams(
    name string,
    description string,
    handler func() (string, error),
) error
```

### 枚举工具注册

```go
// 注册带枚举参数的工具
func (registry *ToolRegistry) RegisterToolWithEnum[T any, E ~string](
    name string,
    description string,
    enumField string,
    enumValues []E,
    handler func(T) (string, error),
) error
```

## 工具参数处理

### 类型反射与Schema生成

```go
// 从结构体生成JSON Schema
func GenerateJSONSchema[T any]() (map[string]interface{}, error)

// 处理结构体字段
func processStructFields(t reflect.Type) (map[string]interface{}, []string, error)

// 处理基本类型
func processField(field reflect.StructField) (map[string]interface{}, bool, error)
```

### 枚举处理

```go
// 处理枚举类型
func processEnum[E ~string](fieldName string, enumValues []E) (map[string]interface{}, error)
```

## 使用示例

### 带参数工具注册

```go
type GetWeatherParams struct {
    City    string `json:"city" description:"城市名称"`
    Format  string `json:"format,omitempty" description:"返回格式，支持text或json"`
}

registry := toolgen.NewToolRegistry()
registry.RegisterTool(
    "get_weather",
    "获取指定城市的天气信息",
    func(params GetWeatherParams) (string, error) {
        // 获取天气逻辑
        return fmt.Sprintf("%s今日天气: 晴朗，25°C", params.City), nil
    },
)
```

### 带枚举参数工具注册

```go
type ListUsersParams struct {
    SortBy string `json:"sort_by" description:"排序字段"`
    Order  string `json:"order" description:"排序方式"`
}

registry.RegisterToolWithEnum(
    "list_users",
    "列出所有用户",
    "order", // 指定哪个字段是枚举
    []string{"asc", "desc"}, // 可选值
    func(params ListUsersParams) (string, error) {
        // 列出用户逻辑
        return "用户列表: [...]", nil
    },
)
```

### 无参数工具注册

```go
registry.RegisterToolWithoutParams(
    "get_system_info",
    "获取系统信息",
    func() (string, error) {
        // 获取系统信息逻辑
        return "CPU: 8核, 内存: 16GB, 磁盘: 512GB", nil
    },
)
```

### 注册到Agent

```go
// 创建Agent
geminiAgent, _ := agent.NewGeminiAgent(agent.AgentConfig{
    APIKey: "your-api-key",
    Debug:  true,
})

// 注册工具到Agent
registry.RegisterToAgent(geminiAgent)
``` 