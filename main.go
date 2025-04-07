package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/562589540/agent-go/agent"
	"github.com/562589540/agent-go/toolgen"
)

// 定义数据结构
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// 存储用户数据的文件路径
const dataFile = "users.json"

// 初始化数据文件
func initDataFile() {
	// 检查文件是否存在
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		// 创建初始数据
		users := []User{
			{ID: "1", Name: "张三", Email: "zhangsan@example.com"},
			{ID: "2", Name: "李四", Email: "lisi@example.com"},
		}
		// 写入文件
		data, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			log.Fatalf("无法序列化初始数据: %v", err)
		}
		if err := ioutil.WriteFile(dataFile, data, 0644); err != nil {
			log.Fatalf("无法写入数据文件: %v", err)
		}
		fmt.Println("已创建数据文件:", dataFile)
	}
}

// 读取所有用户
func readUsers() ([]User, error) {
	data, err := ioutil.ReadFile(dataFile)
	if err != nil {
		return nil, err
	}

	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// 保存用户数据
func saveUsers(users []User) error {
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dataFile, data, 0644)
}

func main() {
	// 初始化数据文件
	initDataFile()

	agentService := agent.NewAgentService(context.Background())

	geminiAgent, err := agent.NewGeminiAgent(agent.AgentConfig{
		APIKey:      "AIzaSyBlqIMp0iRkU66zyk-tozMAxmnD1GWT7uY",
		ProxyURL:    "http://127.0.0.1:7890",
		Debug:       true,
		MaxLoops:    3,
		MaxTokens:   8000,
		Temperature: 0.3,
		TopP:        1,
	})
	if err != nil {
		log.Fatalf("failed to create gemini agent: %v", err)
	}
	agentService.RegisterAgent(agent.Gemini, geminiAgent)

	// 使用toolgen注册工具
	registry := toolgen.NewToolRegistry(geminiAgent)

	// 定义工具输入输出结构

	// 查询用户列表工具
	type ListUsersInput struct {
		Filter string `json:"filter,omitempty" description:"可选的过滤条件，默认返回所有用户"`
	}

	err = toolgen.RegisterTool(
		registry,
		"list_users",
		"获取所有用户列表",
		func(input ListUsersInput) ([]User, error) {
			users, err := readUsers()
			if err != nil {
				return nil, err
			}

			// 如果有过滤条件，进行过滤
			if input.Filter != "" {
				var filtered []User
				for _, user := range users {
					// 检查名称或邮箱是否包含过滤字符串
					if strings.Contains(strings.ToLower(user.Name), strings.ToLower(input.Filter)) ||
						strings.Contains(strings.ToLower(user.Email), strings.ToLower(input.Filter)) {
						filtered = append(filtered, user)
					}
				}
				return filtered, nil
			}

			return users, nil
		},
	)
	if err != nil {
		log.Fatalf("注册查询用户列表工具失败: %v", err)
	}

	// 添加用户工具
	type AddUserInput struct {
		Name  string `json:"name" description:"用户名称"`
		Email string `json:"email" description:"用户邮箱"`
	}

	type AddUserOutput struct {
		ID      string `json:"id" description:"新用户的ID"`
		Message string `json:"message" description:"操作结果消息"`
	}

	err = toolgen.RegisterTool(
		registry,
		"add_user",
		"添加新用户",
		func(input AddUserInput) (AddUserOutput, error) {
			users, err := readUsers()
			if err != nil {
				return AddUserOutput{}, err
			}

			// 自动生成ID
			maxID := 0
			for _, user := range users {
				// 尝试将ID转换为数字
				if userID, err := strconv.Atoi(user.ID); err == nil {
					if userID > maxID {
						maxID = userID
					}
				}
			}
			// 新ID为最大ID+1
			id := strconv.Itoa(maxID + 1)

			// 添加新用户
			newUser := User{
				ID:    id,
				Name:  input.Name,
				Email: input.Email,
			}
			users = append(users, newUser)

			if err := saveUsers(users); err != nil {
				return AddUserOutput{}, err
			}

			return AddUserOutput{
				ID:      id,
				Message: fmt.Sprintf("已成功添加用户: %s，ID为: %s", input.Name, id),
			}, nil
		},
	)
	if err != nil {
		log.Fatalf("注册添加用户工具失败: %v", err)
	}

	// 更新用户工具
	type UpdateUserInput struct {
		ID    string `json:"id" description:"要更新的用户ID"`
		Name  string `json:"name,omitempty" description:"新的用户名称，可选"`
		Email string `json:"email,omitempty" description:"新的用户邮箱，可选"`
	}

	type UpdateUserOutput struct {
		Success bool   `json:"success" description:"是否更新成功"`
		Message string `json:"message" description:"操作结果消息"`
	}

	err = toolgen.RegisterTool(
		registry,
		"update_user",
		"更新用户信息",
		func(input UpdateUserInput) (UpdateUserOutput, error) {
			users, err := readUsers()
			if err != nil {
				return UpdateUserOutput{Success: false, Message: err.Error()}, err
			}

			// 查找并更新用户
			found := false
			for i, user := range users {
				if user.ID == input.ID {
					if input.Name != "" {
						users[i].Name = input.Name
					}
					if input.Email != "" {
						users[i].Email = input.Email
					}
					found = true
					break
				}
			}

			if !found {
				return UpdateUserOutput{
					Success: false,
					Message: fmt.Sprintf("未找到ID为%s的用户", input.ID),
				}, fmt.Errorf("未找到ID为%s的用户", input.ID)
			}

			if err := saveUsers(users); err != nil {
				return UpdateUserOutput{Success: false, Message: err.Error()}, err
			}

			return UpdateUserOutput{
				Success: true,
				Message: fmt.Sprintf("已成功更新ID为%s的用户信息", input.ID),
			}, nil
		},
	)
	if err != nil {
		log.Fatalf("注册更新用户工具失败: %v", err)
	}

	// 删除用户工具
	type DeleteUserInput struct {
		ID string `json:"id" description:"要删除的用户ID"`
	}

	err = toolgen.RegisterSimpleTool(
		registry,
		"delete_user",
		"删除用户",
		func(input DeleteUserInput) (string, error) {
			users, err := readUsers()
			if err != nil {
				return "", err
			}

			// 查找并删除用户
			found := false
			var newUsers []User
			for _, user := range users {
				if user.ID == input.ID {
					found = true
				} else {
					newUsers = append(newUsers, user)
				}
			}

			if !found {
				return "", fmt.Errorf("未找到ID为%s的用户", input.ID)
			}

			if err := saveUsers(newUsers); err != nil {
				return "", err
			}

			return fmt.Sprintf("已成功删除ID为%s的用户", input.ID), nil
		},
	)
	if err != nil {
		log.Fatalf("注册删除用户工具失败: %v", err)
	}

	// 测试对话
	fmt.Println("开始与Gemini代理测试对话...")
	history := []agent.ChatMessage{
		{Role: "system", Content: `
你是一个用户管理助手，兼聊天助手，可以帮助用户管理用户数据。要保持风趣幽默，不要让用户觉得你是一个机器人。
你可以查询、添加、更新和删除用户信息。
必须遵循以下规则：
1. 当用户提出任何请求但你不知道具体信息时，主动使用list_users工具获取用户列表
2. 当用户回复"我不知道"或类似模糊回答时，你应当主动使用list_users工具获取信息
3. 你必须对每个用户请求都给出明确回应，不允许不回答
4. 始终使用工具完成任务，不要仅用文字回复
5. 回复中应当包含具体的用户信息或操作结果
6. 如果用户没有提供详细id的时候，你需要先获取用户列表，然后根据用户提供的name或者email来找到对应的id，不需要请求用户同意。
`},
	}

	// 添加交互循环
	var input string
	scanner := bufio.NewScanner(os.Stdin)

	// 记录AI的上一次回复
	lastAIResponse := ""

	// 添加历史记录处理
	processHistory := func() {
		// 确保历史记录不会太长，最多保留最近10轮对话
		if len(history) > 21 { // system(1) + 10轮对话(20)
			// 保留系统提示和最近的对话
			newHistory := make([]agent.ChatMessage, 0, 21)
			newHistory = append(newHistory, history[0])                   // 系统提示
			newHistory = append(newHistory, history[len(history)-20:]...) // 最近10轮对话
			history = newHistory
		}
	}

	for {
		fmt.Print("\n请输入指令（输入'退出'结束）: ")
		if !scanner.Scan() {
			break
		}

		input = scanner.Text()
		if input == "退出" || input == "exit" || input == "quit" {
			fmt.Println("结束对话")
			break
		}

		// 处理特殊指令
		if input == "清空历史" || input == "clear history" {
			history = []agent.ChatMessage{history[0]} // 只保留系统提示
			fmt.Println("已清空历史记录")
			continue
		}

		if input == "显示历史" || input == "show history" {
			fmt.Println("当前历史记录:")
			for i, msg := range history {
				fmt.Printf("%d. %s: %s\n", i, msg.Role, msg.Content)
			}
			continue
		}

		// 添加用户的输入到历史记录
		userMsg := agent.ChatMessage{
			Role:    "user",
			Content: input,
		}

		// 检查历史记录的最后一条消息是不是也是用户消息
		// 如果是，则需要先添加一个空的AI回复，确保历史记录中是一问一答的交替结构
		if len(history) > 0 && history[len(history)-1].Role == "user" {
			// 在用户消息之间添加一个空的AI回复
			history = append(history, agent.ChatMessage{
				Role:    "assistant",
				Content: "...",
			})
			fmt.Println("【警告】检测到连续的用户消息，已自动添加占位符回复以保持对话结构")
		}

		// 添加当前用户输入
		history = append(history, userMsg)

		// 确保历史记录不会太长
		processHistory()

		// 重置AI回复
		lastAIResponse = ""

		// 创建一个缓冲器来收集响应
		var responseBuffer strings.Builder

		// 调用agent处理用户输入
		fmt.Println("\nAI正在思考...")
		usage, _, err := agentService.StreamRunConversation(
			context.Background(),
			agent.Gemini,
			"gemini-2.0-flash",
			history,
			func(text string) {
				fmt.Print(text)
				responseBuffer.WriteString(text)
			},
		)

		if err != nil {
			log.Printf("对话失败: %v", err)
			errorMsg := fmt.Sprintf("处理请求时出错: %v", err)
			fmt.Println(errorMsg)

			// 添加错误信息到历史记录
			history = append(history, agent.ChatMessage{
				Role:    "system",
				Content: errorMsg,
			})
			continue
		}

		fmt.Println()

		// 保存AI的回复
		lastAIResponse = responseBuffer.String()

		// 如果AI没有给出任何回复，强制添加一条系统消息提示AI回答
		if lastAIResponse == "" {
			fmt.Println("\n【系统提示】AI未返回回复，将再次尝试...")
			// 添加系统消息到历史记录
			history = append(history, agent.ChatMessage{
				Role:    "system",
				Content: "你没有回答用户的问题。请使用工具获取必要信息并给出明确回复。",
			})
			continue
		}

		// 立即将AI的回复添加到历史记录中，确保历史记录中是一问一答的结构
		history = append(history, agent.ChatMessage{
			Role:    "assistant",
			Content: lastAIResponse,
		})

		fmt.Printf("\n\nToken使用情况: %+v\n", usage)
	}
}
