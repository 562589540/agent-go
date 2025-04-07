package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

func TestOpenAIWebSearch(t *testing.T) {
	// 直接使用提供的API密钥和URL
	apiKey := "sk-ofURrnxSMwFonqGpBd4aA1A1DaC64d97A3015c88E61d5411"
	baseURL := "https://api.vveai.com/v1"

	// 创建OpenAI客户端
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	}

	// 创建客户端
	client := openai.NewClient(opts...)

	// 创建聊天上下文
	ctx := context.Background()

	// 创建消息
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("你是一个有用的助手，需要使用最新的互联网信息回答问题。"),
		openai.UserMessage("2024年中国的GDP增长预期是多少？请提供最新的数据。"),
	}

	// 创建聊天参数
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModelGPT4oMini2024_07_18, // 使用GPT-4o模型
		Messages: messages,
		// 启用网络搜索
		WebSearchOptions: openai.ChatCompletionNewParamsWebSearchOptions{
			SearchContextSize: "medium", // 设置搜索上下文大小
		},
		// 可选: 添加用户位置以获取更相关的搜索结果
		/*
			WebSearchOptions: openai.ChatCompletionNewParamsWebSearchOptions{
				SearchContextSize: "medium",
				UserLocation: openai.ChatCompletionNewParamsWebSearchOptionsUserLocation{
					Approximate: openai.ChatCompletionNewParamsWebSearchOptionsUserLocationApproximate{
						Country: param.NewOpt("CN"),
						City:    param.NewOpt("Shanghai"),
					},
				},
			},
		*/
	}

	// 发送请求
	response, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		t.Fatalf("调用API失败: %v", err)
	}

	// 输出回复和搜索结果
	fmt.Println("模型: ", response.Model)
	fmt.Println("回复内容: ", response.Choices[0].Message.Content)

	// 检查是否有注释（搜索结果的引用）
	if len(response.Choices[0].Message.Annotations) > 0 {
		fmt.Println("\n搜索引用:")
		for i, annotation := range response.Choices[0].Message.Annotations {
			if annotation.URLCitation.URL != "" {
				fmt.Printf("%d. %s (%s)\n", i+1, annotation.URLCitation.Title, annotation.URLCitation.URL)
			}
		}
	} else {
		fmt.Println("\n没有搜索引用。可能未启用网络搜索或没有使用引用。")
	}
}

func TestGeminiWebSearch(t *testing.T) {
	t.Skip("Gemini目前不支持与OpenAI相同的Web搜索API，需要使用Google Search API")

	// 注意：Gemini模型需要使用不同的API，这里仅作占位说明
	// Gemini通常不直接通过OpenAI接口支持内置网络搜索
	// 对于Gemini，需要使用Google的Search API，然后将结果提供给模型
}

func TestStreamedWebSearch(t *testing.T) {
	// 直接使用提供的API密钥和URL
	apiKey := "sk-ofURrnxSMwFonqGpBd4aA1A1DaC64d97A3015c88E61d5411"
	baseURL := "https://api.vveai.com/v1"

	// 创建OpenAI客户端
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	}

	// 创建客户端
	client := openai.NewClient(opts...)

	// 创建聊天上下文
	ctx := context.Background()

	// 创建消息
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("你是一个有用的助手，需要使用最新的互联网信息回答问题。"),
		openai.UserMessage("最近有哪些重大的国际新闻？请给出3条并简要分析。"),
	}

	// 创建聊天参数 - 流式传输模式
	params := openai.ChatCompletionNewParams{
		Model:    "gpt-4o", // 使用GPT-4o模型
		Messages: messages,
		// 启用网络搜索
		WebSearchOptions: openai.ChatCompletionNewParamsWebSearchOptions{
			SearchContextSize: "medium", // 设置搜索上下文大小
		},
		// 启用流式传输统计
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.NewOpt(true),
		},
	}

	// 创建流式响应
	stream := client.Chat.Completions.NewStreaming(ctx, params)

	fmt.Println("流式响应开始:")

	// 处理流式响应
	for stream.Next() {
		chunk := stream.Current()

		// 处理每个数据块
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}

	// 检查错误
	if err := stream.Err(); err != nil {
		t.Fatalf("流式处理错误: %v", err)
	}

	fmt.Println("\n\n流式响应结束")
}
