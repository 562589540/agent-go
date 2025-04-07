package test

import (
	"fmt"
	"testing"

	"github.com/562589540/agent-go/agent"
)

func TestGoogleSearchTool(t *testing.T) {
	// 创建谷歌搜索工具
	functionDef, handlerFunc := agent.GoogleSearchTool()

	// 验证工具定义
	if functionDef.Name != "google_search" {
		t.Errorf("期望工具名称为 'google_search'，实际为 '%s'", functionDef.Name)
	}

	// 使用工具搜索
	args := map[string]interface{}{
		"query": "2024年世界经济展望",
		"count": float64(3), // 注意：从JSON解析的数字是float64类型
	}

	result, err := handlerFunc(args)
	if err != nil {
		t.Fatalf("谷歌搜索工具执行失败: %v", err)
	}

	// 输出搜索结果
	fmt.Println(result)

	// 检查结果非空
	if result == "" {
		t.Error("搜索结果不应为空")
	}

	// 测试无效参数
	invalidArgs := map[string]interface{}{
		"query": "",
	}
	_, err = handlerFunc(invalidArgs)
	if err == nil {
		t.Error("对于空查询，应该返回错误")
	}
}

func TestGoogleSearchToolInAgent(t *testing.T) {
	// 这里展示如何在智能体中注册和使用谷歌搜索工具

	// 创建谷歌搜索工具
	functionDef, handlerFunc := agent.GoogleSearchTool()

	// 打印工具定义 - 这是要注册到Agent的内容
	fmt.Printf("工具名称: %s\n", functionDef.Name)
	fmt.Printf("工具描述: %s\n", functionDef.Description)

	// 提示怎样注册到智能体
	fmt.Println("\n要在智能体中使用此工具，请按照以下步骤操作:")
	fmt.Println("1. 获取agent实例")
	fmt.Println("2. 注册谷歌搜索工具:")
	fmt.Println("   agent.RegisterTool(functionDef, handlerFunc)")
	fmt.Println("3. 智能体现在可以调用谷歌搜索工具")

	// 示例调用工具
	args := map[string]interface{}{
		"query": "最新的人工智能技术进展",
	}

	// 只在测试模式运行时执行搜索
	if testing.Short() {
		fmt.Println("\n跳过实际搜索操作")
		return
	}

	result, err := handlerFunc(args)
	if err != nil {
		t.Fatalf("测试搜索操作失败: %v", err)
	}

	fmt.Println("\n搜索测试结果:")
	fmt.Println(result)
}
