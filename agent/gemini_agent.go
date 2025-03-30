package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"google.golang.org/genai"
)

// GeminiAgent 实现Agent接口的Gemini代理
type GeminiAgent struct {
	client     *genai.Client
	config     AgentConfig
	tools      map[string]Tool
	toolParams []*genai.Tool
}

// NewGeminiAgent 创建一个新的Gemini代理
func NewGeminiAgent(config AgentConfig) (*GeminiAgent, error) {
	// 创建HTTP客户端
	httpClient := &http.Client{}

	// 如果设置了代理
	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("解析代理URL错误: %v", err)
		}

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		httpClient = &http.Client{
			Transport: transport,
		}
	}

	// 创建Gemini客户端配置
	clientConfig := &genai.ClientConfig{
		APIKey:     config.APIKey,
		HTTPClient: httpClient,
	}

	// 创建Gemini客户端
	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建Gemini客户端错误: %v", err)
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

	return &GeminiAgent{
		client:     client,
		config:     config,
		tools:      make(map[string]Tool),
		toolParams: []*genai.Tool{},
	}, nil
}

// StreamRunConversation 实现Agent接口的流式对话方法
func (ga *GeminiAgent) StreamRunConversation(
	ctx context.Context,
	modelName string,
	history []ChatMessage,
	handler StreamHandler,
) (*TokenUsage, error) {
	// 对话循环计数器
	loopCount := 0
	// 初始化token统计
	tokenUsage := &TokenUsage{}

	var messages []*genai.Content
	// 提取系统消息
	systemMsg, otherMsgs := ga.extractSystemMessage(history)

	for _, msg := range otherMsgs {
		messages = append(messages, ga.convertMessage(msg))
	}

	// 工具配置
	toolConfig := genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingConfigModeAuto,
		},
	}

	//自定义函数调用配置
	if ga.config.FunctionCallingConfig != nil {
		//工具使用的模式
		if ga.config.FunctionCallingConfig.Mode != "" {
			toolConfig.FunctionCallingConfig.Mode = genai.FunctionCallingConfigMode(ga.config.FunctionCallingConfig.Mode)
		}
		//指定使用哪些工具
		if len(ga.config.FunctionCallingConfig.AllowedFunctionNames) > 0 {
			toolConfig.FunctionCallingConfig.AllowedFunctionNames = ga.config.FunctionCallingConfig.AllowedFunctionNames
		}
	}

	// 对话循环
	for {
		// 检查循环次数是否超过限制
		loopCount++
		if loopCount > ga.config.MaxLoops {
			return tokenUsage, fmt.Errorf("对话循环次数超过最大限制(%d)，可能存在递归", ga.config.MaxLoops)
		}

		ga.debugf("当前循环 %d", loopCount)
		if loopCount > 1 {
			if ga.config.Debug {
				PrintJSON("messages", messages)
			}
		}

		// 每次循环创建新的genConfig
		genConfig := ga.createGenerateContentConfig()

		// 配置工具设置
		genConfig.ToolConfig = &toolConfig

		if ga.config.MaxTokens > 0 {
			maxTokens := int32(ga.config.MaxTokens)
			genConfig.MaxOutputTokens = &maxTokens
		}

		if ga.config.Temperature > 0 {
			temp := float32(ga.config.Temperature)
			genConfig.Temperature = &temp
		}

		if ga.config.TopP > 0 {
			topP := float32(ga.config.TopP)
			genConfig.TopP = &topP
		}

		// 如果有系统指令，添加到配置中
		if systemMsg != "" {
			genConfig.SystemInstruction = &genai.Content{
				Parts: []*genai.Part{
					genai.NewPartFromText(systemMsg),
				},
			}
		}

		if ga.config.Debug {
			PrintJSON("genConfig", genConfig)
			PrintJSON("messages", messages)
		}

		// 获取流式迭代器
		iter := ga.client.Models.GenerateContentStream(ctx, modelName, messages, genConfig)

		hasToolCalls := false
		var functionCalls []*genai.FunctionCall
		var partsList []*genai.Part
		var currentResp *genai.GenerateContentResponse
		var streamErr error
		var textContent string // 用于累积文本内容

		// 处理流式响应
		iter(func(resp *genai.GenerateContentResponse, err error) bool {
			if err != nil {
				ga.debugf("流处理错误: %v", err)
				streamErr = err
				return false
			}

			// 保存最新的响应，用于获取token使用信息
			currentResp = resp

			// 处理响应内容
			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				content := resp.Candidates[0].Content
				for _, part := range content.Parts {
					// 检查是否是文本内容
					if part.Text != "" {
						// 累积文本而不是添加新的部分
						textContent += part.Text

						// 调用回调函数处理流消息
						if handler != nil {
							handler(part.Text)
						}
					}

					// 检测是否有函数调用
					if part.FunctionCall != nil {
						hasToolCalls = true
						functionCalls = append(functionCalls, part.FunctionCall)
						callPart := &genai.Part{FunctionCall: part.FunctionCall}
						partsList = append(partsList, callPart)
						ga.debugf("检测到函数调用: %s", part.FunctionCall.Name)
					}
				}
			}
			return true
		})

		// 如果流处理中出现错误，返回错误
		if streamErr != nil {
			return tokenUsage, fmt.Errorf("流处理错误: %w", streamErr)
		}

		// 添加累积的文本内容（如果有）
		if textContent != "" {
			partsList = append([]*genai.Part{genai.NewPartFromText(textContent)}, partsList...)
		}

		// 获取完整的助手消息
		assistantMsg := &genai.Content{
			Parts: partsList,
			Role:  "model",
		}

		// 添加到消息历史
		messages = append(messages, assistantMsg)

		// 更新token统计（如果有）
		if currentResp != nil && currentResp.UsageMetadata != nil {
			// 总消耗token
			tokenUsage.TotalTokens += int(currentResp.UsageMetadata.TotalTokenCount)

			// 提示词token
			if currentResp.UsageMetadata.PromptTokenCount != nil {
				tokenUsage.PromptTokens += int(*currentResp.UsageMetadata.PromptTokenCount)
			}

			// 完成/响应token
			if currentResp.UsageMetadata.CandidatesTokenCount != nil {
				tokenUsage.CompletionTokens += int(*currentResp.UsageMetadata.CandidatesTokenCount)
			}

			// 缓存token
			if currentResp.UsageMetadata.CachedContentTokenCount != nil {
				tokenUsage.CacheTokens += int(*currentResp.UsageMetadata.CachedContentTokenCount)
			}
		}

		// 如果有工具调用
		if hasToolCalls && len(functionCalls) > 0 {
			// 创建用户响应
			userResponse := &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{},
			}

			// 处理所有工具调用
			for _, functionCall := range functionCalls {
				toolName := functionCall.Name

				// 查找工具
				tool, exists := ga.tools[toolName]
				if !exists {
					ga.debugf("未找到工具: %s", toolName)
					continue
				}

				// 将args转换为map[string]interface{}
				var args map[string]interface{}
				argsJSON, err := json.Marshal(functionCall.Args)
				if err != nil {
					ga.debugf("参数序列化错误: %v", err)
					continue
				}

				if err := json.Unmarshal(argsJSON, &args); err != nil {
					ga.debugf("参数解析错误: %v", err)
					continue
				}
				var responseMap map[string]any

				// 打印要执行的方法和参数
				argsJSON, _ = json.Marshal(args)
				ga.debugf("执行工具: %s, 参数: %s", toolName, string(argsJSON))

				// 执行工具
				result, err := tool.Handler(args)
				if err != nil {
					ga.debugf("工具执行错误: %v", err)
					// 将错误信息作为结果返回给模型
					errResult := fmt.Sprintf("执行错误: %v", err)
					responseMap = map[string]any{"output": errResult, "error": true}
				} else {
					ga.debugf("工具执行成功: %v", result)
					// 添加函数响应到用户消息
					responseMap = map[string]any{"output": result}
				}
				// 添加函数响应到用户消息
				userResponse.Parts = append(userResponse.Parts,
					genai.NewPartFromFunctionResponse(toolName, responseMap))
			}

			// 添加用户响应到消息列表
			messages = append(messages, userResponse)

			// 继续对话，将工具结果发送给模型
			continue
		} else {
			fmt.Println()
			// 没有工具调用，结束对话并返回token统计
			ga.debugf("对话结束，返回Token统计: %+v", tokenUsage)
			return tokenUsage, nil
		}
	}
}

// RegisterTool 注册工具
func (ga *GeminiAgent) RegisterTool(function FunctionDefinitionParam, handler ToolFunction) error {
	if function.Name == "" {
		return fmt.Errorf("工具名称不能为空")
	}

	if handler == nil {
		return fmt.Errorf("工具处理函数不能为空")
	}

	// 保存工具
	ga.tools[function.Name] = Tool{
		Function: function,
		Handler:  handler,
	}

	// 更新工具参数
	ga.rebuildToolParams()
	return nil
}

// 重建工具参数
func (ga *GeminiAgent) rebuildToolParams() {
	ga.toolParams = []*genai.Tool{}

	for _, tool := range ga.tools {
		// 创建函数声明
		functionDec := &genai.FunctionDeclaration{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
		}

		// 如果有参数，设置参数
		if len(tool.Function.Parameters) > 0 {
			// 创建Schema对象
			schema := &genai.Schema{}

			// 将参数转换为JSON字符串
			paramsJSON, err := json.Marshal(tool.Function.Parameters)
			if err != nil {
				ga.debugf("参数序列化错误: %v", err)
				continue
			}

			// 将JSON数据应用到schema
			if err := json.Unmarshal(paramsJSON, schema); err != nil {
				ga.debugf("参数解析到Schema错误: %v", err)
				continue
			}

			// 设置函数参数
			functionDec.Parameters = schema
		}

		// 创建工具参数
		toolParam := &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{functionDec},
		}

		// 添加工具参数
		ga.toolParams = append(ga.toolParams, toolParam)
	}
}

// 创建内容生成配置
func (ga *GeminiAgent) createGenerateContentConfig() *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}

	// 设置生成参数
	if ga.config.MaxTokens > 0 {
		maxTokens := int32(ga.config.MaxTokens)
		config.MaxOutputTokens = &maxTokens
	}

	if ga.config.Temperature > 0 {
		temp := float32(ga.config.Temperature)
		config.Temperature = &temp
	}

	if ga.config.TopP > 0 {
		topP := float32(ga.config.TopP)
		config.TopP = &topP
	}

	// 设置工具
	config.Tools = ga.toolParams

	return config
}

// 提取系统消息
func (ga *GeminiAgent) extractSystemMessage(messages []ChatMessage) (string, []ChatMessage) {
	var systemMsg string
	var otherMsgs []ChatMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
		} else {
			otherMsgs = append(otherMsgs, msg)
		}
	}
	return systemMsg, otherMsgs
}

// convertMessage 转换消息角色和内容
func (ga *GeminiAgent) convertMessage(msg ChatMessage) *genai.Content {
	switch msg.Role {
	case "assistant", "model":
		return &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(msg.Content),
			},
			Role: "model",
		}
	default: // "user" 或其他
		return &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(msg.Content),
			},
			Role: "user",
		}
	}
}

// SetDebug 设置调试模式
func (ga *GeminiAgent) SetDebug(debug bool) {
	ga.config.Debug = debug
}

// debugf 调试输出，统一处理所有调试信息
func (ga *GeminiAgent) debugf(format string, args ...interface{}) {
	if ga.config.Debug {
		fmt.Printf("【DEBUG】"+format+"\n", args...)
	}
}
