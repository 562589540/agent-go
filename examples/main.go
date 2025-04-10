package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("Agent-Go 示例选择器")
	fmt.Println("===================")
	fmt.Println("请选择要运行的示例:")
	fmt.Println("1. 网络搜索示例 (websearch)")
	fmt.Println("2. 必应搜索示例 (bingsearch)")
	fmt.Println("3. 旅行规划链示例 (travelplanning)")
	fmt.Println("4. 文案改写系统示例 (copywriting)")
	fmt.Print("请输入数字(1-4)或示例名称: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var exampleDir string

	switch input {
	case "1", "websearch":
		exampleDir = "websearch"
	case "2", "bingsearch":
		exampleDir = "bingsearch"
	case "3", "travelplanning":
		exampleDir = "travelplanning"
	case "4", "copywriting":
		exampleDir = "copywriting"
	default:
		if dirExists(filepath.Join("examples", input)) {
			exampleDir = input
		} else {
			fmt.Println("无效的选择")
			return
		}
	}

	fmt.Printf("\n正在运行 %s 示例...\n\n", exampleDir)
	runExample(exampleDir)
}

// 检查目录是否存在
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// 运行指定示例目录中的程序
func runExample(dir string) {
	// 确定要运行的命令
	examplePath := filepath.Join("examples", dir)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = examplePath

	// 设置输入输出
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 运行命令
	err := cmd.Run()
	if err != nil {
		fmt.Printf("运行示例失败: %v\n", err)
	}
}
