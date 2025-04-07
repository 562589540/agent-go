package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// 百度搜索API响应结构
type BaiduSearchResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"abstract"`
	} `json:"results"`
}

// 必应搜索API响应结构
type BingSearchResponse struct {
	WebPages struct {
		Value []struct {
			Name        string `json:"name"`
			URL         string `json:"url"`
			Description string `json:"snippet"`
		} `json:"value"`
	} `json:"webPages"`
}

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

// WebSearchTool 创建并返回网络搜索工具定义和处理函数
func WebSearchTool() (FunctionDefinitionParam, ToolFunction) {
	// 定义工具参数结构
	functionDef := FunctionDefinitionParam{
		Name:        "web_search",
		Description: "搜索互联网获取信息。当需要查询最新信息、事实或用户可能提出的问题的答案时使用。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索查询词，应该是一个简明的搜索关键词或问题",
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

		// 调用百度搜索API
		results, err := searchBaidu(query)
		if err != nil {
			return "", fmt.Errorf("搜索失败: %v", err)
		}

		// 格式化结果
		var formattedResults strings.Builder
		formattedResults.WriteString(fmt.Sprintf("搜索结果: %s\n\n", query))

		for i, result := range results.Results {
			if i >= 3 { // 限制结果数量
				break
			}
			formattedResults.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
			formattedResults.WriteString(fmt.Sprintf("   网址: %s\n", result.URL))
			formattedResults.WriteString(fmt.Sprintf("   摘要: %s\n\n", result.Content))
		}

		return formattedResults.String(), nil
	}

	return functionDef, handlerFunc
}

// searchBaidu 调用百度搜索API获取搜索结果
func searchBaidu(query string) (*BaiduSearchResponse, error) {
	// 使用百度智能云搜索API
	// 实际应用中需要申请百度搜索API密钥：https://cloud.baidu.com/product/cts

	baseURL := "https://api.baidu.com/rpc/2.0/cts/v3/search"

	// 构建请求参数
	requestBody := map[string]interface{}{
		"query":     query,
		"page_num":  1,
		"page_size": 5,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建POST请求
	req, err := http.NewRequest("POST", baseURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// 这里需要添加API密钥
	// req.Header.Set("X-Bce-Api-Token", "YOUR_API_KEY")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析JSON响应
	var searchResult BaiduSearchResponse
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &searchResult, nil
}

// SimpleHttpClient 简单的HTTP客户端工具
func SimpleHttpClient() (FunctionDefinitionParam, ToolFunction) {
	// 定义工具参数结构
	functionDef := FunctionDefinitionParam{
		Name:        "http_request",
		Description: "发送HTTP请求获取网页内容。当需要直接访问特定URL获取内容时使用。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "要请求的完整URL",
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "HTTP方法，默认为GET",
					"enum":        []string{"GET", "POST"},
				},
			},
			"required": []string{"url"},
		},
	}

	// 处理函数
	handlerFunc := func(args map[string]interface{}) (string, error) {
		urlStr, ok := args["url"].(string)
		if !ok || urlStr == "" {
			return "", fmt.Errorf("URL不能为空")
		}

		method, ok := args["method"].(string)
		if !ok || method == "" {
			method = "GET"
		}

		// 创建HTTP请求
		req, err := http.NewRequest(method, urlStr, nil)
		if err != nil {
			return "", fmt.Errorf("创建请求失败: %v", err)
		}

		// 添加用户代理头，避免被某些网站拒绝
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		// 发送请求
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("请求失败: %v", err)
		}
		defer resp.Body.Close()

		// 读取响应内容
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取响应失败: %v", err)
		}

		// 返回响应内容的前10000个字符，避免过大的响应
		content := string(body)
		if len(content) > 10000 {
			content = content[:10000] + "...(内容已截断)"
		}

		return content, nil
	}

	return functionDef, handlerFunc
}

// BingSearchTool 创建并返回必应搜索工具定义和处理函数
func BingSearchTool() (FunctionDefinitionParam, ToolFunction) {
	// 定义工具参数结构
	functionDef := FunctionDefinitionParam{
		Name:        "bing_search",
		Description: "使用必应搜索引擎获取互联网信息。适合查询时事、新闻和专业信息。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索查询词，应该是一个简明的搜索关键词或问题",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果数量，默认为3",
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

		// 调用必应搜索API
		results, err := searchBing(query, count)
		if err != nil {
			return "", fmt.Errorf("必应搜索失败: %v", err)
		}

		// 格式化结果
		var formattedResults strings.Builder
		formattedResults.WriteString(fmt.Sprintf("必应搜索结果: %s\n\n", query))

		if len(results.WebPages.Value) == 0 {
			formattedResults.WriteString("未找到相关结果。\n")
		} else {
			for i, result := range results.WebPages.Value {
				if i >= count {
					break
				}
				formattedResults.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Name))
				formattedResults.WriteString(fmt.Sprintf("   网址: %s\n", result.URL))
				formattedResults.WriteString(fmt.Sprintf("   摘要: %s\n\n", result.Description))
			}
		}

		return formattedResults.String(), nil
	}

	return functionDef, handlerFunc
}

// searchBing 调用必应搜索API获取搜索结果
func searchBing(query string, count int) (*BingSearchResponse, error) {
	// 使用必应搜索API需要订阅密钥
	// 实际应用中需要从配置或环境变量获取
	// 可以在此申请必应API密钥：https://www.microsoft.com/en-us/bing/apis/bing-web-search-api
	bingSubscriptionKey := "" // 请填入有效的必应API密钥

	baseURL := "https://api.bing.microsoft.com/v7.0/search"

	// 构建请求URL
	values := url.Values{}
	values.Add("q", query)
	values.Add("count", fmt.Sprintf("%d", count))
	values.Add("responseFilter", "Webpages")
	values.Add("mkt", "zh-CN") // 设置市场为中国，返回中文结果

	requestURL := fmt.Sprintf("%s?%s", baseURL, values.Encode())

	// 创建HTTP请求
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建必应搜索请求失败: %v", err)
	}

	// 添加必应API订阅密钥到头部
	req.Header.Add("Ocp-Apim-Subscription-Key", bingSubscriptionKey)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("必应搜索请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取必应搜索响应失败: %v", err)
	}

	// 如果响应状态码不是200 OK，则返回错误
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("必应搜索API返回错误: %s", string(body))
	}

	// 解析JSON响应
	var searchResult BingSearchResponse
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return nil, fmt.Errorf("解析必应搜索响应失败: %v", err)
	}

	return &searchResult, nil
}

// GoogleSearchTool 创建并返回谷歌搜索工具定义和处理函数
func GoogleSearchTool() (FunctionDefinitionParam, ToolFunction) {
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
		results, err := searchGoogle(query, count)
		if err != nil {
			return "", fmt.Errorf("谷歌搜索失败: %v", err)
		}

		// 格式化结果
		var formattedResults strings.Builder
		formattedResults.WriteString(fmt.Sprintf("谷歌搜索结果: %s\n\n", query))

		if results.Items == nil || len(results.Items) == 0 {
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

	return functionDef, handlerFunc
}

// searchGoogle 调用谷歌自定义搜索API获取搜索结果
func searchGoogle(query string, count int) (*GoogleSearchResponse, error) {
	// Google Custom Search API密钥
	apiKey := "AIzaSyByFckwiCTv6DvlL2cfvOmPwWXhGJmYNYI"

	// 搜索引擎ID (cx参数)
	searchEngineID := "c28d9acdf00c6418e"

	// 设置代理
	proxyURL := "http://127.0.0.1:7890"

	// 构建请求URL
	baseURL := "https://www.googleapis.com/customsearch/v1"
	params := url.Values{}
	params.Add("key", apiKey)
	params.Add("cx", searchEngineID)
	params.Add("q", query)
	params.Add("num", fmt.Sprintf("%d", count))

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 创建HTTP客户端并配置代理
	client := &http.Client{}
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
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
