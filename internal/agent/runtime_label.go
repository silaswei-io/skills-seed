package agent

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// RuntimeLabelFromAnalysisUnit 生成用于 runtime 文件名的短标签。
func RuntimeLabelFromAnalysisUnit(unitID, unitName string) string {
	if safe := runtimefiles.SafePart(unitID, ""); safe != "" {
		return "unit-" + safe
	}
	if safe := runtimefiles.SafePart(unitName, ""); safe != "" {
		return "unit-" + safe
	}
	return ""
}

// RuntimePromptInputPrefix 给 prompt 输入目录追加本次运行标签。
func RuntimePromptInputPrefix(base, label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return base
	}
	return base + "-" + label
}

// AnalyzeCurrentCodebaseOperation 返回当前代码库分析的可读运行操作名。
func AnalyzeCurrentCodebaseOperation(req *AnalyzeCurrentCodebaseRequest) string {
	if req == nil || strings.TrimSpace(req.RuntimeLabel) == "" {
		return "AnalyzeCurrentCodebase"
	}
	return "AnalyzeCurrentCodebase/" + req.RuntimeLabel
}
