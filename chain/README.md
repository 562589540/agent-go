# Chain 包 - LangChain架构实现

本包实现了类似LangChain的组件系统，用于构建大型语言模型应用的工作流。

## 主要组件

### 1. Chain（链）

链是工作流的基本单位，负责处理输入并产生输出：

- `Chain` - 链接口，定义了链的基本行为
- `BaseChain` - 链的基本实现
- `SequentialChain` - 顺序执行多个链
- `SimplePromptChain` - 简单的提示词链，用于生成提示词
- `LLMChain` - 与LLM交互的链

### 2. Memory（记忆）

记忆组件负责存储和管理对话状态：

- `Memory` - 记忆接口
- `SimpleMemory` - 简单记忆实现
- `ConversationMemory` - 对话历史记忆

### 3. PromptTemplate（提示词模板）

提示词模板用于生成结构化的提示词：

- `PromptTemplate` - 提示词模板，支持变量替换

### 4. OutputParser（输出解析器）

输出解析器负责将LLM的文本输出转换为结构化数据：

- `OutputParser` - 输出解析器接口
- `SimpleOutputParser` - 简单文本解析器
- `StructuredOutputParser` - 结构化输出解析器

## 使用示例

### 1. 创建和运行简单链

```go
// 创建提示词模板
template, err := chain.NewPromptTemplate(
    "回答以下问题: {{.question}}",
    []string{"question"},
)

// 创建LLM链
llmChain := chain.NewLLMChain(
    "问答链",
    template,
    agentService,
    agent.OpenAI,
    "gpt-4",
    nil, // 使用默认的SimpleOutputParser
)

// 运行链
input := chain.ChainInput{
    "question": "什么是人工智能?",
}
output, err := llmChain.Run(ctx, input)
```

### 2. 顺序链

```go
// 创建一个顺序链
sequentialChain := chain.NewSequentialChain(
    "多步骤处理",
    []chain.Chain{firstChain, secondChain, thirdChain},
)

// 运行顺序链
output, err := sequentialChain.Run(ctx, input)
```

### 3. 使用记忆组件

```go
// 创建对话记忆
memory := chain.NewConversationMemory("human_message", "ai_message", 10)

// 将记忆添加到链
llmChain.Memory = memory

// 运行带记忆的链
output, err := llmChain.Run(ctx, input)

// 获取对话历史
history := memory.GetConversationHistory()
```

### 4. 结构化输出

```go
// 创建结构化输出解析器
schema := map[string]string{
    "answer": "问题的答案",
    "confidence": "置信度（0-100）",
    "sources": "信息来源",
}
outputParser := chain.NewStructuredOutputParser(schema)

// 创建带结构化输出的LLM链
structuredLLMChain := chain.NewLLMChain(
    "结构化输出链",
    template,
    agentService,
    agent.OpenAI,
    "gpt-4",
    outputParser,
)
```

## 完整示例

参见 `examples/main.go` 中的旅行规划示例，展示了如何组合多个组件构建复杂应用。

## 扩展

你可以通过实现相应接口扩展功能：

- 实现 `Chain` 接口创建自定义链
- 实现 `Memory` 接口创建自定义记忆组件
- 实现 `OutputParser` 接口创建自定义输出解析器 