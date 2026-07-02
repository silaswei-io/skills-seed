package jsonrepair

import (
	"encoding/json"
	"fmt"
	"strings"

	jsonrepairgo "github.com/silaswei-io/jsonrepair-go"
)

// FixAIJSON 修复 AI 常见的非标准 JSON 输出，并返回可被 encoding/json 解析的 JSON。
func FixAIJSON(jsonStr string) (string, error) {
	current := strings.TrimSpace(jsonStr)
	if current == "" {
		return "", fmt.Errorf("empty JSON")
	}
	if err := validateJSON(current); err == nil {
		return current, nil
	}

	repaired, err := jsonrepairgo.Repair(current)
	if err != nil {
		return "", err
	}
	repaired = strings.TrimSpace(repaired)
	if repaired == "" {
		return "", fmt.Errorf("empty repaired JSON")
	}
	if err := validateJSON(repaired); err != nil {
		return "", err
	}
	return repaired, nil
}

func validateJSON(jsonStr string) error {
	var js interface{}
	return json.Unmarshal([]byte(jsonStr), &js)
}
