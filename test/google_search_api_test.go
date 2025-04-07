package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
)

// 搜索结果结构
type SearchResult struct {
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

func TestGoogleCustomSearchAPI(t *testing.T) {
	// Google Custom Search API密钥
	apiKey := "AIzaSyByFckwiCTv6DvlL2cfvOmPwWXhGJmYNYI"

	// 搜索引擎ID (cx参数)
	searchEngineID := "c28d9acdf00c6418e"

	// 设置代理
	proxyURL := "http://127.0.0.1:7890"

	// 搜索查询
	query := "2024年中国GDP增长预期"

	// 构建请求URL
	baseURL := "https://www.googleapis.com/customsearch/v1"
	params := url.Values{}
	params.Add("key", apiKey)
	params.Add("cx", searchEngineID)
	params.Add("q", query)

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 创建HTTP客户端并配置代理
	client := &http.Client{}
	if proxyURL != "" {
		proxyURLParsed, err := url.Parse(proxyURL)
		if err != nil {
			t.Fatalf("解析代理URL失败: %v", err)
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
		t.Fatalf("创建请求失败: %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}

	// 检查响应状态码
	if response.StatusCode != http.StatusOK {
		t.Fatalf("API请求失败，状态码: %d, 响应: %s", response.StatusCode, string(body))
	}

	// 解析JSON响应
	var searchResult SearchResult
	if err := json.Unmarshal(body, &searchResult); err != nil {
		t.Fatalf("解析JSON响应失败: %v", err)
	}

	// 输出搜索结果
	fmt.Printf("搜索用时: %.2f秒, 找到约 %s 个结果\n\n",
		searchResult.SearchInformation.SearchTime,
		searchResult.SearchInformation.TotalResults)

	// 显示搜索结果
	fmt.Println("搜索结果:")
	for i, item := range searchResult.Items {
		fmt.Printf("%d. %s\n", i+1, item.Title)
		fmt.Printf("   链接: %s\n", item.Link)
		fmt.Printf("   摘要: %s\n\n", item.Snippet)
	}
}

// 测试Web搜索工具功能
func TestCustomSearchTool(t *testing.T) {
	// 创建一个简单的搜索函数
	search := func(query string) ([]map[string]string, error) {
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
		var searchResult SearchResult
		if err := json.Unmarshal(body, &searchResult); err != nil {
			return nil, fmt.Errorf("解析JSON响应失败: %v", err)
		}

		// 转换为简化的结果格式
		results := make([]map[string]string, 0, len(searchResult.Items))
		for _, item := range searchResult.Items {
			result := map[string]string{
				"title":   item.Title,
				"link":    item.Link,
				"snippet": item.Snippet,
			}
			results = append(results, result)
		}

		return results, nil
	}

	// 测试搜索功能
	results, err := search("最近的国际新闻事件")
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	fmt.Println("搜索结果:")
	for i, result := range results {
		fmt.Printf("%d. %s\n", i+1, result["title"])
		fmt.Printf("   链接: %s\n", result["link"])
		fmt.Printf("   摘要: %s\n\n", result["snippet"])
	}

	// 这个搜索函数可以集成到您的Agent工具中
	fmt.Println("您可以将此搜索功能集成到您的Agent工具中")
}
