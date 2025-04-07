package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

// OpenAIAgent 实现Agent接口的OpenAI代理
type OpenAIAgent struct {
	client     openai.Client
	config     AgentConfig
	tools      map[string]Tool
	toolParams []openai.ChatCompletionToolParam
}

// NewOpenAIAgent 创建一个新的OpenAI代理
func NewOpenAIAgent(config AgentConfig) (*OpenAIAgent, error) {
	// 创建OpenAI客户端选项
	var opts []option.RequestOption

	// 添加API密钥
	opts = append(opts, option.WithAPIKey(config.APIKey))

	// 如果设置了自定义URL
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}

	// 如果设置了代理
	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("解析代理URL错误: %v", err)
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		httpClient := &http.Client{
			Transport: transport,
		}
		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	// 设置默认值
	if config.MaxLoops <= 0 {
		config.MaxLoops = 5
	}

	if config.Temperature < 0 {
		config.Temperature = 0.7
	}

	if config.TopP < 0 {
		config.TopP = 1.0
	}

	// 设置默认频率限制值
	if config.RateLimitDelay < 0 {
		config.RateLimitDelay = 0
	}

	// 创建客户端
	client := openai.NewClient(opts...)

	return &OpenAIAgent{
		client:     client,
		config:     config,
		tools:      make(map[string]Tool),
		toolParams: []openai.ChatCompletionToolParam{},
	}, nil
}

// StreamRunConversation 实现Agent接口的流式对话方法
func (oa *OpenAIAgent) StreamRunConversation(
	ctx context.Context,
	modelName string,
	history []ChatMessage,
	handler StreamHandler,
) (*TokenUsage, []ChatMessage, error) {
	// 打包工具参数
	oa.rebuildToolParams()

	// 初始化token统计
	tokenUsage := &TokenUsage{}

	// 初始化对话历史，只记录本次对话
	var conversationHistory []ChatMessage

	// 添加最后一条用户消息到对话历史（本次问题）
	if len(history) > 0 {
		lastMsg := history[len(history)-1]
		if lastMsg.Role == "user" {
			conversationHistory = append(conversationHistory, lastMsg)
		}
	}

	// 如果没有提供模型名称，使用默认值
	if modelName == "" {
		modelName = oa.config.ModelName
		if modelName == "" {
			modelName = "gpt-4o" // 默认模型
		}
	}

	// 创建消息数组，首先提取系统消息
	var messages []openai.ChatCompletionMessageParamUnion
	for _, msg := range history {
		messages = append(messages, oa.convertMessage(msg))
	}

	// 对话循环计数器
	loopCount := 0

	if oa.config.Debug {
		PrintJSON("oa.toolParams", oa.toolParams)
	}

	// 对话循环
	for {
		// 检查循环次数是否超过限制
		loopCount++
		if loopCount > oa.config.MaxLoops {
			return tokenUsage, conversationHistory, fmt.Errorf("对话循环次数超过最大限制(%d)，可能存在递归", oa.config.MaxLoops)
		}

		// 频率限制：如果不是第一轮对话且启用了频率限制，则添加延迟
		if loopCount > 1 && oa.config.EnableRateLimit && oa.config.RateLimitDelay > 0 {
			oa.debugf("频率限制：等待 %d 毫秒后继续", oa.config.RateLimitDelay)
			select {
			case <-ctx.Done():
				return tokenUsage, conversationHistory, ctx.Err()
			case <-time.After(time.Duration(oa.config.RateLimitDelay) * time.Millisecond):
				// 等待指定时间后继续
			}
		}

		oa.debugf("开始流式请求，模型=%s, 循环次数=%d/%d", modelName, loopCount, oa.config.MaxLoops)
		if loopCount > 1 {
			if oa.config.Debug {
				PrintJSON("messages", messages)
			}
		}

		// 创建请求参数
		params := openai.ChatCompletionNewParams{
			Model:    modelName,
			Messages: messages,
			//Seed:     openai.Int(0),
			Tools: oa.toolParams,
			ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: param.NewOpt("auto"),
			},
			StreamOptions: openai.ChatCompletionStreamOptionsParam{
				IncludeUsage: param.NewOpt(true),
			},
		}

		//必须先调用工具
		if loopCount == 1 && oa.config.OnecFunctionCallingConfigModeAny {
			params.ToolChoice.OfAuto = param.NewOpt("any")
		} else {
			params.ToolChoice.OfAuto = param.NewOpt("auto")
		}

		//自定义函数调用配置
		if oa.config.FunctionCallingConfig != nil {
			//工具使用的模式
			if oa.config.FunctionCallingConfig.Mode != "" {
				params.ToolChoice.OfAuto = param.NewOpt(oa.config.FunctionCallingConfig.Mode)
			}
			//指定使用哪些工具
			if len(oa.config.FunctionCallingConfig.AllowedFunctionNames) > 0 {
				//todo::
				fmt.Println("openai 不支持指定多个工具")
			}
		}

		// 设置最大回复token
		if oa.config.MaxTokens > 0 {
			params.MaxCompletionTokens = param.NewOpt(oa.config.MaxTokens)
		}

		//设置温度
		if oa.config.Temperature > 0 {
			params.Temperature = param.NewOpt(oa.config.Temperature)
		}

		//设置topp
		if oa.config.TopP > 0 {
			params.TopP = param.NewOpt(oa.config.TopP)
		}

		// 创建流式请求
		stream := oa.client.Chat.Completions.NewStreaming(ctx, params)

		// 使用累加器处理流式响应
		acc := openai.ChatCompletionAccumulator{}
		toolCallReceived := false

		// 处理流式响应
		for stream.Next() {
			chunk := stream.Current()

			// 添加当前块到累加器
			acc.AddChunk(chunk)

			//文本完成
			if _, ok := acc.JustFinishedContent(); ok {
				println("finish-event: Content stream finished")
			}

			//AI拒绝回答的原因
			if refusal, ok := acc.JustFinishedRefusal(); ok {
				fmt.Println("AI 拒绝回答:", refusal)
			}

			// 检查是否有工具调用完成
			if tool, ok := acc.JustFinishedToolCall(); ok {
				toolCallReceived = true
				fmt.Println("检测到完整工具调用:", tool.Index, tool.Name, tool.Arguments)
			}

			// 从chunk中提取文本内容并处理流式消息
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				content := chunk.Choices[0].Delta.Content
				// 调用处理函数
				if handler != nil {
					handler(content)
				}
			}
		}

		// 检查流是否发生错误
		if err := stream.Err(); err != nil {
			return tokenUsage, conversationHistory, fmt.Errorf("流处理错误: %v", err)
		}

		// 流结束后，获取完整响应
		if len(acc.Choices) == 0 {
			return tokenUsage, conversationHistory, fmt.Errorf("没有收到回复")
		}

		// 更新Token使用情况
		usage := acc.Usage

		if usage.TotalTokens > 0 {
			tokenUsage.TotalTokens += int(usage.TotalTokens)
			tokenUsage.PromptTokens += int(usage.PromptTokens)
			tokenUsage.CompletionTokens += int(usage.CompletionTokens)
			tokenUsage.CacheTokens += int(usage.PromptTokensDetails.CachedTokens)
			oa.debugf("Token使用情况 - 总计: %d, 提示词: %d, 完成: %d",
				tokenUsage.TotalTokens, tokenUsage.PromptTokens, tokenUsage.CompletionTokens)
		}

		// 获取完整的助手消息
		assistantMessage := acc.Choices[0].Message
		oa.debugf("收到助手消息，内容: %s", assistantMessage.Content)

		// 将助手消息添加到对话中（OpenAI格式）
		assistantParam := assistantMessage.ToParam()
		messages = append(messages, assistantParam)

		// 创建通用格式的助手消息
		assistantChatMsg := ChatMessage{
			Role:    "assistant",
			Content: assistantMessage.Content,
		}

		// 处理工具调用
		toolCalls := assistantMessage.ToolCalls
		if toolCallReceived && len(toolCalls) > 0 {
			oa.debugf("收到助手消息，包含 %d 个工具调用", len(toolCalls))

			// 添加工具调用到通用消息格式
			assistantChatMsg.ToolCalls = oa.convertOpenAIToolCallsToToolCalls(toolCalls)

			// 添加助手消息到对话历史
			conversationHistory = append(conversationHistory, assistantChatMsg)

			// 处理所有工具调用
			allToolsHandled := true

			for i, toolCall := range assistantMessage.ToolCalls {
				oa.debugf("工具调用 #%d:", i+1)
				oa.debugf("  ID: %s", toolCall.ID)
				oa.debugf("  名称: %s", toolCall.Function.Name)
				oa.debugf("  参数: %s", toolCall.Function.Arguments)

				// 处理ID为空的情况
				callID := toolCall.ID
				if callID == "" {
					callID = fmt.Sprintf("auto_id_%d", i)
					oa.debugf("工具调用ID为空，自动生成ID: %s", callID)
				}

				// 查找工具
				tool, exists := oa.tools[toolCall.Function.Name]
				if !exists {
					oa.debugf("未找到工具: %s", toolCall.Function.Name)
					allToolsHandled = false
					continue
				}

				// 解析参数
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					oa.debugf("参数解析错误: %v", err)
					allToolsHandled = false
					continue
				}

				// 执行工具
				result, err := tool.Handler(args)
				if err != nil {
					oa.debugf("工具执行错误: %v", err)
					// 如果需要，可以将错误消息返回给模型
					result = fmt.Sprintf("执行错误: %v", err)
				}

				oa.debugf("工具执行结果: %s", result)

				// 将工具响应添加到OpenAI对话
				toolMsg := openai.ToolMessage(result, callID)
				messages = append(messages, toolMsg)

				// 创建工具响应消息
				toolResponseMsg := ChatMessage{
					Role: "tool", // 使用tool角色而不是user
				}
				// 将工具响应添加到通用消息格式
				functionResponse := FunctionResponse{
					ID:     callID,
					Name:   toolCall.Function.Name,
					Result: map[string]any{"output": result},
				}
				toolResponseMsg.FunctionResponses = append(toolResponseMsg.FunctionResponses, functionResponse)
				conversationHistory = append(conversationHistory, toolResponseMsg)
			}

			// 添加工具响应消息到对话历史

			// 如果有工具调用失败，可以选择是否继续对话
			if !allToolsHandled {
				oa.debugf("部分工具调用失败，但继续对话")
			}

			// 继续对话
			continue
		} else {
			// 没有工具调用，添加普通助手消息到对话历史
			conversationHistory = append(conversationHistory, assistantChatMsg)

			// 返回响应内容
			oa.debugf("对话结束，返回Token统计: %+v", tokenUsage)
			return tokenUsage, conversationHistory, nil
		}
	}
}

// RegisterTool 注册一个工具
func (oa *OpenAIAgent) RegisterTool(function FunctionDefinitionParam, handler ToolFunction) error {
	if function.Name == "" {
		return fmt.Errorf("工具名称不能为空")
	}

	if handler == nil {
		return fmt.Errorf("工具处理函数不能为空")
	}

	// 保存工具
	oa.tools[function.Name] = Tool{
		Function: function,
		Handler:  handler,
	}

	return nil
}

// SetDebug 设置调试模式
func (oa *OpenAIAgent) SetDebug(debug bool) {
	oa.config.Debug = debug
}

// 重建工具参数
func (oa *OpenAIAgent) rebuildToolParams() {
	oa.toolParams = []openai.ChatCompletionToolParam{}

	for _, tool := range oa.tools {
		// 把agent.FunctionDefinitionParam转换为openai.FunctionDefinitionParam
		functionDef := openai.FunctionDefinitionParam{
			Name:        tool.Function.Name,
			Description: param.NewOpt(tool.Function.Description),
			Parameters:  tool.Function.Parameters,
		}

		// 创建工具参数
		toolParam := openai.ChatCompletionToolParam{
			Type:     "function",
			Function: functionDef,
		}

		oa.toolParams = append(oa.toolParams, toolParam)
	}

	// 打印调试信息
	if oa.config.Debug {
		oa.debugf("工具参数构建完成: %d 个工具", len(oa.toolParams))
		for i, param := range oa.toolParams {
			oa.debugf("工具 #%d: %s", i, param.Function.Name)
		}
	}
}

// convertMessage 转换消息角色和内容
func (oa *OpenAIAgent) convertMessage(msg ChatMessage) openai.ChatCompletionMessageParamUnion {
	switch msg.Role {
	case "system":
		return openai.SystemMessage(msg.Content)

	case "user":
		// 用户消息
		return openai.UserMessage(msg.Content)

	case "assistant":
		// 如果有工具调用，处理工具调用（这里实际上我们在流程中已经处理好了，这里主要是转换历史消息）
		if len(msg.ToolCalls) > 0 {
			// 正确处理带工具调用的助手消息
			var assistant openai.ChatCompletionAssistantMessageParam
			assistant.Content.OfString = param.NewOpt(msg.Content)

			// 转换所有工具调用
			assistant.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls))
			for i, toolCall := range msg.ToolCalls {
				assistant.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
					ID:   toolCall.ID,
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name: toolCall.Name,
						Arguments: func() string {
							// 将参数转回JSON字符串
							argsBytes, err := json.Marshal(toolCall.Args)
							if err != nil {
								oa.debugf("转换工具调用参数错误: %v", err)
								return "{}"
							}
							return string(argsBytes)
						}(),
					},
				}
			}

			return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
		}

		// 普通助手消息
		return openai.AssistantMessage(msg.Content)

	case "tool":
		// 工具响应消息
		if len(msg.FunctionResponses) > 0 {
			// 只处理第一个函数响应，因为OpenAI的ToolMessage只支持一个工具调用ID和内容
			funcResp := msg.FunctionResponses[0]

			var content string
			if output, ok := funcResp.Result["output"]; ok {
				content = fmt.Sprintf("%v", output)
			} else {
				outputJSON, _ := json.Marshal(funcResp.Result)
				content = string(outputJSON)
			}

			return openai.ToolMessage(content, funcResp.ID)
		}

		// 没有函数响应的工具消息，这种情况不太常见
		return openai.ToolMessage(msg.Content, "")

	default:
		// 默认作为用户消息处理
		return openai.UserMessage(msg.Content)
	}
}

// debugf 调试输出，统一处理所有调试信息
func (oa *OpenAIAgent) debugf(format string, args ...interface{}) {
	if oa.config.Debug {
		fmt.Printf("【OpenAI】"+format+"\n", args...)
	}
}

// 从OpenAI响应转换到通用格式的函数
func (oa *OpenAIAgent) convertOpenAIToolCallsToToolCalls(toolCalls []openai.ChatCompletionMessageToolCall) []FunctionCall {
	functionCalls := make([]FunctionCall, 0, len(toolCalls))

	// 添加工具调用到通用消息格式
	for _, toolCall := range toolCalls {
		// 解析参数
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			oa.debugf("参数解析错误: %v", err)
			continue
		}

		// 将OpenAI的工具调用转换为通用格式
		functionCall := FunctionCall{
			ID:   toolCall.ID,
			Name: toolCall.Function.Name,
			Args: args,
		}

		functionCalls = append(functionCalls, functionCall)
	}

	return functionCalls
}
