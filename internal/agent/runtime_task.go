package agent

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// RuntimeTask 标识一次 agent 调用中共享的 runtime 文件名前缀。
type RuntimeTask struct {
	ID         string
	Slug       string
	PromptOnly bool
}

// NewRepositoryReadRuntimeTask 创建允许读取 runtime 输入和仓库源码的只读任务。
func NewRepositoryReadRuntimeTask(slug string) RuntimeTask {
	return NewRuntimeTask(slug)
}

// NewPromptOnlyRuntimeTask 创建不需要仓库读取工具的自包含任务。
func NewPromptOnlyRuntimeTask(slug string) RuntimeTask {
	task := NewRuntimeTask(slug)
	task.PromptOnly = true
	return task
}

// NewRuntimeTask 创建 prompt 与 agent 输出共用的 runtime 任务标识。
func NewRuntimeTask(slug string) RuntimeTask {
	return RuntimeTask{
		ID:   runtimefiles.NewID(),
		Slug: strings.TrimSpace(slug),
	}
}

// FirstRuntimeTask 返回可选任务列表中的首个任务。
func FirstRuntimeTask(tasks []RuntimeTask) RuntimeTask {
	if len(tasks) == 0 {
		return RuntimeTask{}
	}
	return tasks[0]
}

// RuntimeSlug 合并模板名和业务标签，生成可读语义名。
func RuntimeSlug(name, label string) string {
	parts := []string{}
	if safe := runtimefiles.SafePart(name, ""); safe != "" {
		parts = append(parts, safe)
	}
	if safe := runtimefiles.SafePart(label, ""); safe != "" {
		if trimmed := trimRuntimeSlugOverlap(parts, safe); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "-")
}

func trimRuntimeSlugOverlap(parts []string, label string) string {
	if len(parts) == 0 || label == "" {
		return label
	}
	lastPart := parts[len(parts)-1]
	lastSegments := strings.Split(lastPart, "-")
	labelSegments := strings.Split(label, "-")
	if len(lastSegments) == 0 || len(labelSegments) == 0 {
		return label
	}
	if lastSegments[len(lastSegments)-1] != labelSegments[0] {
		return label
	}
	return strings.Join(labelSegments[1:], "-")
}
