package jsonrepair

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

var codeFenceStartRE = regexp.MustCompile("(?s)```(?:json|JSON)?\\s*\\n")

// ExtractJSON 从 AI 输出中提取 JSON。
func ExtractJSON(output string) (string, error) {
	trimmed := strings.TrimSpace(output)
	if len(trimmed) == 0 {
		logger.Error(i18n.Get("AgentEmptyAIOutput"),
			"output_length", len(output),
			"hint", i18n.Get("AgentEmptyAIOutputHint"))
		return "", fmt.Errorf("%s", i18n.Get("AgentEmptyAIResponse"))
	}

	if codeFenceStartRE.MatchString(output) {
		if jsonStr := extractJSONFromCodeBlock(output); jsonStr != "" {
			return jsonStr, nil
		}
	}

	start := findLikelyJSONObjectStart(output)
	if start == -1 {
		logger.Error(i18n.Get("AgentNoJSONFound"),
			"output_length", len(output),
			"hint", i18n.Get("AgentNoJSONFoundHint"))
		return "", fmt.Errorf("%s", i18n.Get("AgentNoJSONObjectFound"))
	}

	end := findMatchingBrace(output, start)
	if end == -1 {
		if repaired, repairErr := FixAIJSON(output[start:]); repairErr == nil {
			return repaired, nil
		}
		logger.Error(i18n.Get("AgentUnmatchedBraces"),
			"start", start,
			"output_length", len(output))
		return "", fmt.Errorf("%s", i18n.Get("AgentUnmatchedJSONBraces"))
	}

	jsonStr := strings.TrimSpace(output[start : end+1])
	repaired, err := FixAIJSON(jsonStr)
	if err != nil {
		logger.Error(i18n.Get("AgentInvalidJSON"),
			"error", err,
			"json_length", len(jsonStr))
		return "", fmt.Errorf("%s: %w", i18n.Get("AgentInvalidJSONError"), err)
	}

	return repaired, nil
}

func findLikelyJSONObjectStart(output string) int {
	targetKeys := map[string]bool{
		"patterns":     true,
		"issues":       true,
		"fixes":        true,
		"project_name": true,
		"include":      true,
		"name":         true,
	}
	for idx, ch := range output {
		if ch != '{' {
			continue
		}
		pos := idx + 1
		for pos < len(output) {
			switch output[pos] {
			case ' ', '\n', '\r', '\t':
				pos++
			default:
				goto keyStart
			}
		}
	keyStart:
		if pos >= len(output) || output[pos] != '"' {
			continue
		}
		pos++
		keyEnd := strings.IndexByte(output[pos:], '"')
		if keyEnd < 0 {
			continue
		}
		key := output[pos : pos+keyEnd]
		if targetKeys[key] {
			return idx
		}
	}
	return strings.Index(output, "{")
}

func extractJSONFromCodeBlock(output string) string {
	start := strings.Index(output, "```")
	for start != -1 {
		blockStart := start + 3
		for blockStart < len(output) && output[blockStart] != '\n' {
			blockStart++
		}
		if blockStart < len(output) {
			blockStart++
		}

		blockEnd := strings.Index(output[blockStart:], "```")
		if blockEnd == -1 {
			break
		}
		blockEnd += blockStart

		blockContent := strings.TrimSpace(output[blockStart:blockEnd])
		jsonStart := strings.Index(blockContent, "{")
		if jsonStart != -1 {
			jsonEnd := findMatchingBrace(blockContent, jsonStart)
			if jsonEnd != -1 {
				candidate := strings.TrimSpace(blockContent[jsonStart : jsonEnd+1])
				if repaired, err := FixAIJSON(candidate); err == nil {
					return repaired
				}
			} else if repaired, err := FixAIJSON(blockContent[jsonStart:]); err == nil {
				return repaired
			}
		}

		start = strings.Index(output[blockEnd+3:], "```")
		if start != -1 {
			start += blockEnd + 3
		}
	}
	return ""
}

func findMatchingBrace(s string, start int) int {
	braceCount := 0
	inString := false
	escapeNext := false

	for i := start; i < len(s); i++ {
		ch := s[i]
		if escapeNext {
			escapeNext = false
			continue
		}
		if ch == '\\' {
			escapeNext = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			braceCount++
		} else if ch == '}' {
			braceCount--
			if braceCount == 0 {
				return i
			}
		}
	}

	return -1
}

// TruncString 截断字符串用于日志输出（按 rune 截断，不破坏 UTF-8）。
func TruncString(s string, maxLen int) string {
	if len([]rune(s)) <= maxLen {
		return s
	}
	return string([]rune(s)[:maxLen]) + "..."
}
