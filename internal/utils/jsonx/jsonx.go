package jsonx

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	jsonrepair "github.com/silaswei-io/jsonrepair-go"
)

// ErrNoJSONCandidate 表示文本中没有可修复的 JSON 片段。
var ErrNoJSONCandidate = jsonrepair.ErrNoJSONCandidate

// UnmarshalStrict 解析可信 JSON 数据，并拒绝未知字段和尾随 JSON 值。
func UnmarshalStrict(data []byte, target any) error {
	return decodeStrict(string(data), target)
}

// UnmarshalFromTextStrict 从非纯 JSON 文本中抽取并修复 JSON，再按目标结构严格解析。
func UnmarshalFromTextStrict(text string, target any) error {
	if repaired, ok := RepairCandidate(text); ok {
		return unmarshalCandidate(repaired, target)
	}

	var lastErr error
	for _, candidate := range extractedCandidates(text) {
		if err := unmarshalCandidate(candidate, target); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return ErrNoJSONCandidate
}

// Candidates 返回文本中可修复为合法 JSON 的候选片段。
func Candidates(text string) []string {
	candidates := make([]string, 0, 1)
	if repaired, ok := RepairCandidate(text); ok {
		candidates = appendUnique(candidates, repaired)
	}
	for _, candidate := range extractedCandidates(text) {
		candidates = appendUnique(candidates, candidate)
	}
	return candidates
}

func extractedCandidates(text string) []string {
	candidates := make([]string, 0)
	for _, candidate := range jsonrepair.ExtractJSON(text) {
		if repaired, ok := RepairCandidate(candidate); ok {
			candidates = appendUnique(candidates, repaired)
		}
	}
	return candidates
}

// RepairCandidate 修复一个候选 JSON 片段；非对象或数组片段不作为结构化输出候选。
func RepairCandidate(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return "", false
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed, true
	}
	repaired, err := jsonrepair.Repair(trimmed)
	if err != nil || !json.Valid([]byte(repaired)) {
		return "", false
	}
	return repaired, true
}

// FormatIfJSON 将合法 JSON 格式化；非 JSON 原样返回并标记为 false。
func FormatIfJSON(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}
	var value any
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return "", false
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", false
	}
	return string(data), true
}

func unmarshalCandidate(candidate string, target any) error {
	if err := UnmarshalStrict([]byte(candidate), target); err == nil {
		return nil
	} else {
		nested := nestedOutputCandidates(candidate)
		if len(nested) == 0 {
			return err
		}
		var lastErr error
		for _, payload := range nested {
			if err := UnmarshalStrict([]byte(payload), target); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return lastErr
	}
}

func nestedOutputCandidates(candidate string) []string {
	var envelope struct {
		Result           any             `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal([]byte(candidate), &envelope); err != nil {
		return nil
	}

	var nested []string
	if len(envelope.StructuredOutput) > 0 && string(envelope.StructuredOutput) != "null" {
		nested = appendUnique(nested, string(envelope.StructuredOutput))
	}
	if result, ok := envelope.Result.(string); ok {
		for _, candidate := range Candidates(result) {
			nested = appendUnique(nested, candidate)
		}
	}
	return nested
}

func decodeStrict(text string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func appendUnique(candidates []string, candidate string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return candidates
	}
	for _, existing := range candidates {
		if existing == candidate {
			return candidates
		}
	}
	return append(candidates, candidate)
}
