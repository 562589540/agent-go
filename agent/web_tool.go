package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// 谷歌搜索API响应结构
type GoogleSearchResponse struct {
	Items []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		Snippet     string `json:"snippet"`
		DisplayLink string `json:"displayLink"`
	} `json:"items"`
	SearchInformation struct {
		TotalResults     string  `json:"totalResults"`
		FormattedResults string  `json:"formattedTotalResults"`
		SearchTime       float64 `json:"searchTime"`
	} `json:"searchInformation"`
}

// GoogleSearchConfig 谷歌搜索配置参数
type GoogleSearchConfig struct {
	APIKey         string // Google Custom Search API密钥
	SearchEngineID string // 搜索引擎ID (cx参数)
	ProxyURL       string // 代理URL，可选
}

// RegisterGoogleSearchTool 向Agent注册谷歌搜索工具
func RegisterGoogleSearchTool(agent Agent, config GoogleSearchConfig) error {
	// 定义工具参数结构
	functionDef := FunctionDefinitionParam{
		Name:        "google_search",
		Description: "使用谷歌自定义搜索引擎获取互联网信息。适合查询时事、技术文档和全球信息。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索查询词，应该是一个简明的搜索关键词或问题",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果数量，默认为3，最大为10",
				},
			},
			"required": []string{"query"},
		},
	}

	// 处理函数
	handlerFunc := func(args map[string]interface{}) (string, error) {
		query, ok := args["query"].(string)
		if !ok || query == "" {
			return "", fmt.Errorf("搜索查询不能为空")
		}

		count := 3
		if countArg, ok := args["count"].(float64); ok {
			count = int(countArg)
			if count < 1 {
				count = 1
			} else if count > 10 {
				count = 10
			}
		}

		// 调用谷歌搜索API
		results, err := searchGoogle(query, count, config)
		if err != nil {
			return "", fmt.Errorf("谷歌搜索失败: %v", err)
		}

		// 格式化结果
		var formattedResults strings.Builder
		formattedResults.WriteString(fmt.Sprintf("谷歌搜索结果: %s\n\n", query))

		if len(results.Items) == 0 {
			formattedResults.WriteString("未找到相关结果。\n")
		} else {
			searchInfo := fmt.Sprintf("找到约 %s 个结果 (%.2f 秒)\n\n",
				results.SearchInformation.TotalResults,
				results.SearchInformation.SearchTime)
			formattedResults.WriteString(searchInfo)

			for i, item := range results.Items {
				if i >= count {
					break
				}
				formattedResults.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Title))
				formattedResults.WriteString(fmt.Sprintf("   网址: %s\n", item.Link))
				formattedResults.WriteString(fmt.Sprintf("   摘要: %s\n\n", item.Snippet))
			}
		}

		return formattedResults.String(), nil
	}

	// 直接注册工具到Agent
	return agent.RegisterTool(functionDef, handlerFunc)
}

// searchGoogle 调用谷歌自定义搜索API获取搜索结果
func searchGoogle(query string, count int, config GoogleSearchConfig) (*GoogleSearchResponse, error) {
	// 参数验证
	if config.APIKey == "" {
		return nil, fmt.Errorf("google API密钥不能为空")
	}
	if config.SearchEngineID == "" {
		return nil, fmt.Errorf("搜索引擎ID不能为空")
	}

	// 构建请求URL
	baseURL := "https://www.googleapis.com/customsearch/v1"
	params := url.Values{}
	params.Add("key", config.APIKey)
	params.Add("cx", config.SearchEngineID)
	params.Add("q", query)
	params.Add("num", fmt.Sprintf("%d", count))

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 创建HTTP客户端并配置代理
	client := &http.Client{}
	if config.ProxyURL != "" {
		proxyURLParsed, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("解析代理URL失败: %v", err)
		}
		client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
			},
		}
	}

	// 发送请求
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查响应状态码
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败，状态码: %d, 响应: %s", response.StatusCode, string(body))
	}

	// 解析JSON响应
	var searchResult GoogleSearchResponse
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("解析JSON响应失败: %v", err)
	}

	return &searchResult, nil
}
