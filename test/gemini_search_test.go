package test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/562589540/agent-go/agent"
	"google.golang.org/genai"
)

func TestGeminiGoogleSearch(t *testing.T) {
	// 设置API密钥
	geminiAPIKey := "AIzaSyBlqIMp0iRkU66zyk-tozMAxmnD1GWT7uY" // Gemini API密钥
	// Google Custom Search API密钥已在Custom Search Engine中配置

	// 设置代理
	proxyURL := "http://127.0.0.1:7890"

	// 创建HTTP客户端并配置代理
	httpClient := &http.Client{}
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			t.Fatalf("解析代理URL失败: %v", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
			},
		}
	}

	// 创建Gemini客户端配置
	clientConfig := &genai.ClientConfig{
		APIKey:     geminiAPIKey,
		HTTPClient: httpClient,
	}

	// 创建Gemini客户端
	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建工具配置
	toolConfig := &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingConfigModeAny,
		},
	}

	// 添加Google搜索工具
	tool := &genai.Tool{
		GoogleSearch: &genai.GoogleSearch{},
	}

	// 创建生成内容配置
	genConfig := &genai.GenerateContentConfig{
		ToolConfig: toolConfig,
		Tools:      []*genai.Tool{tool},
	}

	// 创建提示消息
	messages := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText("曼谷地震了吗，中文回答，最新数据"),
			},
			Role: "user",
		},
	}

	// 生成内容（流式）
	iter := client.Models.GenerateContentStream(ctx, "gemini-2.0-flash", messages, genConfig)

	fmt.Println("Gemini回复:")

	var fullResp *genai.GenerateContentResponse
	var textContent string

	iter(func(resp *genai.GenerateContentResponse, err error) bool {
		if err != nil {
			t.Fatalf("生成内容失败: %v", err)
			return false
		}

		fullResp = resp
		agent.PrintJSON("resp", resp)
		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				if part.Text != "" {
					textContent += part.Text
					fmt.Print(part.Text)
				}
			}
		}
		return true
	})

	fmt.Println("\n\n引用信息:")
	// 检查是否有引用
	if fullResp != nil && len(fullResp.Candidates) > 0 {
		candidate := fullResp.Candidates[0]
		if candidate.CitationMetadata != nil && len(candidate.CitationMetadata.Citations) > 0 {
			for i, citation := range candidate.CitationMetadata.Citations {
				fmt.Printf("%d. %s\n", i+1, citation.URI)
			}
		} else {
			fmt.Println("没有搜索引用。可能搜索功能未启用或未找到相关信息。")
		}
	}
}

func TestGeminiSearchRetrieval(t *testing.T) {
	// 设置API密钥
	geminiAPIKey := "AIzaSyBlqIMp0iRkU66zyk-tozMAxmnD1GWT7uY" // Gemini API密钥
	// Google Custom Search API密钥已在Custom Search Engine中配置

	// 设置代理
	proxyURL := "http://127.0.0.1:7890"

	// 创建HTTP客户端并配置代理
	httpClient := &http.Client{}
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			t.Fatalf("解析代理URL失败: %v", err)
		}
		httpClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
			},
		}
	}

	// 创建Gemini客户端配置
	clientConfig := &genai.ClientConfig{
		APIKey:     geminiAPIKey,
		HTTPClient: httpClient,
	}

	// 创建Gemini客户端
	client, err := genai.NewClient(context.Background(), clientConfig)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	ctx := context.Background()

	// 创建工具配置
	toolConfig := &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode: genai.FunctionCallingConfigModeAuto,
		},
	}

	// 添加Google搜索工具（改用GoogleSearch代替GoogleSearchRetrieval）
	tool := &genai.Tool{
		GoogleSearch: &genai.GoogleSearch{},
	}

	// 创建生成内容配置
	genConfig := &genai.GenerateContentConfig{
		ToolConfig: toolConfig,
		Tools:      []*genai.Tool{tool},
	}

	// 系统指令
	systemInstruction := &genai.Content{
		Parts: []*genai.Part{
			genai.NewPartFromText("你需要使用Google搜索获取最新信息来回答问题。总是先搜索，然后基于搜索结果回答。"),
		},
		Role: "model",
	}
	genConfig.SystemInstruction = systemInstruction

	// 创建提示消息
	messages := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText("最近有哪些重大国际新闻事件？请列出3条并提供简要分析。"),
			},
			Role: "user",
		},
	}

	// 生成内容（流式）
	iter := client.Models.GenerateContentStream(ctx, "gemini-2.0-flash", messages, genConfig)

	fmt.Println("Gemini回复:")

	var fullResp *genai.GenerateContentResponse
	var textContent string

	iter(func(resp *genai.GenerateContentResponse, err error) bool {
		if err != nil {
			t.Fatalf("生成内容失败: %v", err)
			return false
		}

		fullResp = resp

		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				if part.Text != "" {
					textContent += part.Text
					fmt.Print(part.Text)
				}
			}
		}
		return true
	})

	fmt.Println("\n\n引用信息:")
	// 检查是否有引用
	if fullResp != nil && len(fullResp.Candidates) > 0 {
		candidate := fullResp.Candidates[0]
		if candidate.CitationMetadata != nil && len(candidate.CitationMetadata.Citations) > 0 {
			for i, citation := range candidate.CitationMetadata.Citations {
				fmt.Printf("%d. %s\n", i+1, citation.URI)
			}
		} else {
			fmt.Println("没有搜索引用。可能搜索功能未启用或未找到相关信息。")
		}
	}
}
