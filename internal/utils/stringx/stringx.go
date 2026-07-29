package stringx

import "strings"

// FirstNonEmpty 返回第一个 trim 后非空的字符串，保留原始值。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// FirstNonBlank 返回第一个 trim 后非空的字符串，并返回 trim 后的值。
func FirstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

// EmptyIfNil 返回非 nil 字符串切片，便于输出稳定 JSON/模板数据。
func EmptyIfNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// UniqueNonBlank 返回去重后的非空字符串，保持首次出现顺序。
func UniqueNonBlank(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// UniqueNonEmpty 返回去重后的非空字符串，保持原始值和首次出现顺序。
func UniqueNonEmpty(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// NormalizeStructureSummary 规范化源码结构摘要文本，便于写入 prompt 输入。
func NormalizeStructureSummary(structure string) string {
	structure = strings.ReplaceAll(structure, "\u00a0", " ")
	structure = strings.ReplaceAll(structure, "&nbsp;", " ")
	structure = strings.ReplaceAll(structure, "\r\n", "\n")
	structure = strings.ReplaceAll(structure, "\r", "\n")

	lines := strings.Split(structure, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
