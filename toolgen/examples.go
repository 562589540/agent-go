package toolgen

import (
	"encoding/json"
	"fmt"

	"github.com/562589540/agent-go/agent"

	"io/ioutil"
	"os"
	"strings"
	"time"
)

// 以下是使用示例，展示如何使用工具库来定义和注册工具

// User 用户数据结构
type User struct {
	ID     string `json:"id" description:"用户唯一标识符"`
	Name   string `json:"name" description:"用户名称"`
	Email  string `json:"email" description:"用户邮箱"`
	Role   string `json:"role" description:"用户角色" enum:"admin,user,guest"`
	Status int    `json:"status" description:"用户状态" enum:"0,1,2"` // 0: 未激活, 1: 激活, 2: 已禁用
}

// DeleteUserInput 删除用户的输入参数
type DeleteUserInput struct {
	ID string `json:"id" description:"要删除的用户ID"`
}

// DeleteUserOutput 删除用户的输出结果
type DeleteUserOutput struct {
	Success bool   `json:"success" description:"是否删除成功"`
	Message string `json:"message" description:"操作结果消息"`
}

// AddUserInput 添加用户的输入参数
type AddUserInput struct {
	Name   string `json:"name" description:"新用户的名称"`
	Email  string `json:"email" description:"新用户的邮箱"`
	Role   string `json:"role" description:"用户角色" enum:"admin,user,guest"`
	Status int    `json:"status" description:"用户状态" enum:"0,1,2"`
}

// AddUserOutput 添加用户的输出结果
type AddUserOutput struct {
	Success bool   `json:"success" description:"是否添加成功"`
	ID      string `json:"id" description:"新用户的ID"`
	Message string `json:"message" description:"操作结果消息"`
}

// UpdateUserInput 更新用户的输入参数
type UpdateUserInput struct {
	ID     string `json:"id" description:"要更新的用户ID"`
	Name   string `json:"name,omitempty" description:"新的用户名称，可选"`
	Email  string `json:"email,omitempty" description:"新的用户邮箱，可选"`
	Role   string `json:"role,omitempty" description:"用户角色，可选" enum:"admin,user,guest"`
	Status int    `json:"status,omitempty" description:"用户状态，可选" enum:"0,1,2"`
}

// ListUsersInput 列出用户的输入参数
type ListUsersInput struct {
	Filter string `json:"filter,omitempty" description:"可选的过滤条件"`
	Role   string `json:"role,omitempty" description:"按角色过滤" enum:"admin,user,guest"`
	Status int    `json:"status,omitempty" description:"按状态过滤" enum:"0,1,2"`
}

// ServerStatusOutput 服务器状态输出结构
type ServerStatusOutput struct {
	Status    string `json:"status" description:"服务器当前状态"`
	Uptime    string `json:"uptime" description:"服务器运行时间"`
	UserCount int    `json:"user_count" description:"系统用户数量"`
	Version   string `json:"version" description:"服务器版本"`
}

// ExampleRegisterUserTools 注册用户管理相关的工具示例
func ExampleRegisterUserTools(a agent.Agent) error {
	// 创建工具注册器
	registry := NewToolRegistry(a)

	// 数据文件路径
	const dataFile = "users.json"

	// 读取用户
	readUsers := func() ([]User, error) {
		data, err := ioutil.ReadFile(dataFile)
		if err != nil {
			if os.IsNotExist(err) {
				return []User{}, nil
			}
			return nil, err
		}

		var users []User
		if err := json.Unmarshal(data, &users); err != nil {
			return nil, err
		}
		return users, nil
	}

	// 保存用户
	saveUsers := func(users []User) error {
		data, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			return err
		}
		return ioutil.WriteFile(dataFile, data, 0644)
	}

	// 注册列出用户的工具
	err := RegisterTool(
		registry,
		"list_users",
		"获取所有用户列表",
		func(input ListUsersInput) ([]User, error) {
			users, err := readUsers()
			if err != nil {
				return nil, err
			}

			// 如果有过滤条件，进行过滤
			filtered := users

			if input.Filter != "" {
				var tempFiltered []User
				for _, user := range filtered {
					// 简单实现：检查名称或邮箱是否包含过滤字符串
					if contains(user.Name, input.Filter) || contains(user.Email, input.Filter) {
						tempFiltered = append(tempFiltered, user)
					}
				}
				filtered = tempFiltered
			}

			// 按角色过滤
			if input.Role != "" {
				var tempFiltered []User
				for _, user := range filtered {
					if user.Role == input.Role {
						tempFiltered = append(tempFiltered, user)
					}
				}
				filtered = tempFiltered
			}

			// 按状态过滤
			if input.Status != 0 {
				var tempFiltered []User
				for _, user := range filtered {
					if user.Status == input.Status {
						tempFiltered = append(tempFiltered, user)
					}
				}
				filtered = tempFiltered
			}

			return filtered, nil
		},
	)
	if err != nil {
		return err
	}

	// 注册添加用户的工具
	err = RegisterTool(
		registry,
		"add_user",
		"添加新用户",
		func(input AddUserInput) (AddUserOutput, error) {
			users, err := readUsers()
			if err != nil {
				return AddUserOutput{Success: false, Message: err.Error()}, err
			}

			// 生成新ID
			maxID := 0
			for _, user := range users {
				if id, err := parseInt(user.ID); err == nil && id > maxID {
					maxID = id
				}
			}
			newID := fmt.Sprintf("%d", maxID+1)

			// 创建新用户
			newUser := User{
				ID:     newID,
				Name:   input.Name,
				Email:  input.Email,
				Role:   input.Role,
				Status: input.Status,
			}

			// 添加到用户列表
			users = append(users, newUser)

			if err := saveUsers(users); err != nil {
				return AddUserOutput{Success: false, Message: err.Error()}, err
			}

			return AddUserOutput{
				Success: true,
				ID:      newID,
				Message: fmt.Sprintf("已成功添加用户: %s，ID为: %s", input.Name, newID),
			}, nil
		},
	)
	if err != nil {
		return err
	}

	// 注册更新用户的工具
	err = RegisterTool(
		registry,
		"update_user",
		"更新用户信息",
		func(input UpdateUserInput) (DeleteUserOutput, error) {
			users, err := readUsers()
			if err != nil {
				return DeleteUserOutput{Success: false, Message: err.Error()}, err
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
					if input.Role != "" {
						users[i].Role = input.Role
					}
					if input.Status != 0 {
						users[i].Status = input.Status
					}
					found = true
					break
				}
			}

			if !found {
				msg := fmt.Sprintf("未找到ID为%s的用户", input.ID)
				return DeleteUserOutput{Success: false, Message: msg}, fmt.Errorf(msg)
			}

			if err := saveUsers(users); err != nil {
				return DeleteUserOutput{Success: false, Message: err.Error()}, err
			}

			return DeleteUserOutput{
				Success: true,
				Message: fmt.Sprintf("已成功更新ID为%s的用户信息", input.ID),
			}, nil
		},
	)
	if err != nil {
		return err
	}

	// 注册删除用户的工具 - 使用简单工具，只返回字符串
	err = RegisterSimpleTool(
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
		return err
	}

	// 注册获取服务器状态的无参数工具 - 返回结构化数据
	err = RegisterNoParamTool(
		registry,
		"get_server_status",
		"获取服务器状态信息",
		func() (ServerStatusOutput, error) {
			// 读取用户数量
			users, err := readUsers()
			if err != nil {
				return ServerStatusOutput{}, err
			}

			// 返回服务器状态信息
			return ServerStatusOutput{
				Status:    "running",
				Uptime:    "3d 5h 12m",
				UserCount: len(users),
				Version:   "1.0.0",
			}, nil
		},
	)
	if err != nil {
		return err
	}

	// 注册获取当前时间的无参数简单工具 - 只返回字符串
	err = RegisterNoParamSimpleTool(
		registry,
		"get_current_time",
		"获取服务器当前时间",
		func() (string, error) {
			// 获取当前时间并格式化
			now := time.Now().Format("2006-01-02 15:04:05")
			return fmt.Sprintf("服务器当前时间: %s", now), nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

// 辅助函数

// contains 检查s是否包含substr
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// parseInt 将字符串转换为整数
func parseInt(s string) (int, error) {
	i := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("非数字字符: %c", c)
		}
		i = i*10 + int(c-'0')
	}
	return i, nil
}
