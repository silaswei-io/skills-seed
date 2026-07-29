package agent

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// RuntimeLabelFromEvidenceFocus 生成用于 runtime 文件名的短标签。
func RuntimeLabelFromEvidenceFocus(focusID, focusName string) string {
	if safe := runtimefiles.SafePart(focusID, ""); safe != "" {
		return "focus-" + safe
	}
	if safe := runtimefiles.SafePart(focusName, ""); safe != "" {
		return "focus-" + safe
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

// AnalyzeCurrentCodebaseBatchOperation 返回当前代码库批量分析的可读运行操作名。
func AnalyzeCurrentCodebaseBatchOperation(req *AnalyzeCurrentCodebaseBatchRequest) string {
	if req == nil || strings.TrimSpace(req.RuntimeLabel) == "" {
		return "AnalyzeCurrentCodebaseBatch"
	}
	return "AnalyzeCurrentCodebaseBatch/" + req.RuntimeLabel
}

// AnalyzeCurrentDeltaBatchOperation 返回当前代码库增量分析的可读运行操作名。
func AnalyzeCurrentDeltaBatchOperation(req *AnalyzeCurrentDeltaBatchRequest) string {
	if req == nil || strings.TrimSpace(req.RuntimeLabel) == "" {
		return "AnalyzeCurrentDeltaBatch"
	}
	return "AnalyzeCurrentDeltaBatch/" + req.RuntimeLabel
}
