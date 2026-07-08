package agent

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// RuntimeTask 标识一次 agent 调用中共享的 runtime 文件名前缀。
type RuntimeTask struct {
	ID   string
	Slug string
}

// NewRuntimeTask 创建 prompt 与 agent 输出共用的 runtime 任务标识。
func NewRuntimeTask(slug string) RuntimeTask {
	return RuntimeTask{
		ID:   runtimefiles.NewID(),
		Slug: strings.TrimSpace(slug),
	}
}

// RuntimeSlug 合并模板名和业务标签，生成可读语义名。
func RuntimeSlug(name, label string) string {
	parts := []string{}
	if safe := runtimefiles.SafePart(name, ""); safe != "" {
		parts = append(parts, safe)
	}
	if safe := runtimefiles.SafePart(label, ""); safe != "" {
		parts = append(parts, safe)
	}
	return strings.Join(parts, "-")
}
