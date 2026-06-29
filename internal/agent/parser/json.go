// Package parser 提供 AI 输出的 JSON 提取、修复和领域结果解析功能。
package parser

import "github.com/silaswei-io/skills-seed/internal/agent/parser/jsonrepair"

// ExtractJSON 从 AI 输出中提取 JSON。
func ExtractJSON(output string) (string, error) {
	return jsonrepair.ExtractJSON(output)
}

// FixAIJSON 修复 AI 常见的非标准 JSON 输出。
func FixAIJSON(jsonStr string) (string, error) {
	return jsonrepair.FixAIJSON(jsonStr)
}

// TruncString 截断字符串用于日志输出（按 rune 截断，不破坏 UTF-8）。
func TruncString(s string, maxLen int) string {
	return jsonrepair.TruncString(s, maxLen)
}
