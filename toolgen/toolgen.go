package toolgen

import (
	"encoding/json"
	"fmt"

	"reflect"
	"strings"

	"github.com/562589540/agent-go/agent"
)

// EmptyParams 表示没有参数的空参数结构体
type EmptyParams struct{}

// ToolRegistry 工具注册管理器
type ToolRegistry struct {
	agent agent.Agent // 使用已存在的Agent接口
}

// NewToolRegistry 创建工具注册管理器
func NewToolRegistry(agent agent.Agent) *ToolRegistry {
	return &ToolRegistry{agent: agent}
}

// RegisterToolFunc 注册工具函数类型，支持泛型输入输出
type RegisterToolFunc[T any, R any] func(input T) (R, error)

// RegisterTool 注册带输入输出的工具
func RegisterTool[T any, R any](
	registry *ToolRegistry,
	name string,
	description string,
	handler RegisterToolFunc[T, R],
) error {
	// 1. 通过反射获取参数结构体信息
	var paramType T
	paramSchema := structToJSONSchema(reflect.TypeOf(paramType))

	// 2. 创建函数定义
	def := agent.FunctionDefinitionParam{
		Name:        name,
		Description: description,
		Parameters:  paramSchema,
	}

	// 3. 创建适配器处理函数，处理类型转换
	wrapperHandler := func(args map[string]interface{}) (string, error) {
		// 将map转换为强类型结构体
		var input T
		data, err := json.Marshal(args)
		if err != nil {
			return "", fmt.Errorf("序列化参数错误: %w", err)
		}

		if err := json.Unmarshal(data, &input); err != nil {
			return "", fmt.Errorf("反序列化到结构体错误: %w", err)
		}

		// 调用实际处理函数
		result, err := handler(input)
		if err != nil {
			return "", err
		}

		// 将结果转换为JSON字符串
		outputData, err := json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("序列化结果错误: %w", err)
		}

		return string(outputData), nil
	}

	// 4. 注册工具到Agent
	return registry.agent.RegisterTool(def, wrapperHandler)
}

// RegisterSimpleTool 注册简单工具，只返回字符串
func RegisterSimpleTool[T any](
	registry *ToolRegistry,
	name string,
	description string,
	handler func(input T) (string, error),
) error {
	wrappedHandler := func(input T) (struct{ Message string }, error) {
		msg, err := handler(input)
		if err != nil {
			return struct{ Message string }{""}, err
		}
		return struct{ Message string }{msg}, nil
	}

	return RegisterTool(registry, name, description, wrappedHandler)
}

// RegisterNoParamTool 注册无参数工具，返回任意类型结果
func RegisterNoParamTool[R any](
	registry *ToolRegistry,
	name string,
	description string,
	handler func() (R, error),
) error {
	// 包装处理函数，接收空参数
	wrappedHandler := func(_ EmptyParams) (R, error) {
		return handler()
	}

	return RegisterTool(registry, name, description, wrappedHandler)
}

// RegisterNoParamSimpleTool 注册无参数简单工具，只返回字符串
func RegisterNoParamSimpleTool(
	registry *ToolRegistry,
	name string,
	description string,
	handler func() (string, error),
) error {
	// 包装处理函数，接收空参数
	wrappedHandler := func(_ EmptyParams) (string, error) {
		return handler()
	}

	return RegisterSimpleTool(registry, name, description, wrappedHandler)
}

// structToJSONSchema 将结构体转换为JSON Schema
func structToJSONSchema(t reflect.Type) map[string]interface{} {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []interface{}{},
	}

	properties := schema["properties"].(map[string]interface{})
	required := schema["required"].([]interface{})

	if t.Kind() != reflect.Struct {
		// 如果不是结构体，返回一个空的对象schema
		return schema
	}

	// 特殊处理EmptyParams类型
	if t == reflect.TypeOf(EmptyParams{}) {
		// Gemini要求当parameters.type为"object"时，properties字段不能为空
		// 为无参数工具添加一个虚拟属性，满足Gemini API要求
		properties["_dummy"] = map[string]interface{}{
			"type":        "string",
			"description": "此参数无需填写，仅用于满足API要求",
		}
		return schema
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 跳过未导出字段
		if !field.IsExported() {
			continue
		}

		// 获取字段名
		jsonTag := field.Tag.Get("json")
		name := field.Name
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "-" {
				name = parts[0]
			} else {
				// 如果json标签是"-"，跳过这个字段
				continue
			}
		}

		// 检查是否必填
		if !strings.Contains(jsonTag, "omitempty") {
			required = append(required, name)
		}

		// 添加属性
		fieldSchema := fieldToSchema(field)
		if fieldSchema != nil {
			properties[name] = fieldSchema
		}
	}

	schema["required"] = required
	return schema
}

// fieldToSchema 将字段转换为JSON Schema属性
func fieldToSchema(field reflect.StructField) map[string]interface{} {
	// 根据字段类型生成对应的JSON Schema
	prop := map[string]interface{}{}

	// 获取字段描述
	description := field.Tag.Get("description")
	if description != "" {
		prop["description"] = description
	}

	// 处理枚举值
	enum := field.Tag.Get("enum")
	if enum != "" {
		// 解析枚举值，格式为"value1,value2,value3"
		enumValues := parseEnumValues(enum, field.Type)
		if len(enumValues) > 0 {
			prop["enum"] = enumValues
		}
	}

	switch field.Type.Kind() {
	case reflect.String:
		prop["type"] = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		prop["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		prop["type"] = "number"
	case reflect.Bool:
		prop["type"] = "boolean"
	case reflect.Slice, reflect.Array:
		prop["type"] = "array"
		elemType := field.Type.Elem()
		if elemType.Kind() == reflect.Struct {
			// 对于结构体数组，生成嵌套的对象schema
			prop["items"] = structToJSONSchema(elemType)
		} else {
			// 对于基本类型数组，生成简单的类型描述
			elemProp := map[string]interface{}{}
			switch elemType.Kind() {
			case reflect.String:
				elemProp["type"] = "string"
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				elemProp["type"] = "integer"
			case reflect.Float32, reflect.Float64:
				elemProp["type"] = "number"
			case reflect.Bool:
				elemProp["type"] = "boolean"
			default:
				elemProp["type"] = "string"
			}
			prop["items"] = elemProp
		}
	case reflect.Map:
		prop["type"] = "object"
		// 对于map，我们不能准确描述它的属性，所以使用additionalProperties
		if field.Type.Key().Kind() == reflect.String {
			valueType := field.Type.Elem()
			if valueType.Kind() == reflect.Struct {
				prop["additionalProperties"] = structToJSONSchema(valueType)
			} else {
				valueProp := map[string]interface{}{}
				switch valueType.Kind() {
				case reflect.String:
					valueProp["type"] = "string"
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
					reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					valueProp["type"] = "integer"
				case reflect.Float32, reflect.Float64:
					valueProp["type"] = "number"
				case reflect.Bool:
					valueProp["type"] = "boolean"
				default:
					valueProp["type"] = "string"
				}
				prop["additionalProperties"] = valueProp
			}
		}
	case reflect.Struct:
		// 对于嵌套结构体，递归生成schema
		nestedSchema := structToJSONSchema(field.Type)
		for k, v := range nestedSchema {
			prop[k] = v
		}
	case reflect.Ptr:
		// 对于指针，获取指向的类型
		return fieldToSchema(reflect.StructField{
			Name: field.Name,
			Type: field.Type.Elem(),
			Tag:  field.Tag,
		})
	case reflect.Interface:
		// 对于接口，无法确定具体类型，使用通用的对象描述
		prop["type"] = "object"
	default:
		prop["type"] = "string"
	}

	return prop
}

// parseEnumValues 解析枚举值
func parseEnumValues(enumTag string, fieldType reflect.Type) []interface{} {
	values := strings.Split(enumTag, ",")
	result := make([]interface{}, 0, len(values))

	// 根据字段类型转换枚举值
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		switch fieldType.Kind() {
		case reflect.String:
			result = append(result, v)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// 尝试将字符串转换为整数
			if intVal, err := parseEnumInt(v); err == nil {
				result = append(result, intVal)
			}
		case reflect.Float32, reflect.Float64:
			// 尝试将字符串转换为浮点数
			if floatVal, err := parseFloat(v); err == nil {
				result = append(result, floatVal)
			}
		case reflect.Bool:
			// 尝试将字符串转换为布尔值
			if v == "true" {
				result = append(result, true)
			} else if v == "false" {
				result = append(result, false)
			}
		}
	}

	return result
}

// parseEnumInt 解析整数(用于枚举值)
func parseEnumInt(s string) (int, error) {
	i := 0
	negative := false

	if len(s) > 0 && s[0] == '-' {
		negative = true
		s = s[1:]
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("非数字字符: %c", c)
		}
		i = i*10 + int(c-'0')
	}

	if negative {
		i = -i
	}

	return i, nil
}

// parseFloat 解析浮点数
func parseFloat(s string) (float64, error) {
	var result float64
	var afterDecimal float64
	var divider float64 = 1
	var negative bool
	var decimalPart bool

	if len(s) > 0 && s[0] == '-' {
		negative = true
		s = s[1:]
	}

	for _, c := range s {
		if c == '.' {
			if decimalPart {
				return 0, fmt.Errorf("多个小数点")
			}
			decimalPart = true
			continue
		}

		if c < '0' || c > '9' {
			return 0, fmt.Errorf("非数字字符: %c", c)
		}

		digit := float64(c - '0')

		if decimalPart {
			divider *= 10
			afterDecimal = afterDecimal*10 + digit
		} else {
			result = result*10 + digit
		}
	}

	result += afterDecimal / divider

	if negative {
		result = -result
	}

	return result, nil
}
