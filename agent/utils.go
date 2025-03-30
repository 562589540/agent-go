package agent

import (
	"encoding/json"
	"fmt"
)

func PrintJSON(prefix string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%s: 错误: %v\n", prefix, err)
		return
	}
	fmt.Printf("%s: %s\n", prefix, string(data))
}
