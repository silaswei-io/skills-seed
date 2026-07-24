package promptio

import (
	"github.com/silaswei-io/skills-seed/internal/agent"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
)

// Renderer 是 Agent 依赖的最小提示词渲染能力，便于测试渲染错误链路。
type Renderer interface {
	Render(name string, data interface{}) (string, error)
	RenderForRuntimeTask(name string, data interface{}, task promptloader.RuntimeTask) (string, error)
}

// RuntimeTask 转换为提示词 loader 识别的 runtime 任务。
func RuntimeTask(task agent.RuntimeTask) promptloader.RuntimeTask {
	return promptloader.RuntimeTask{ID: task.ID, Slug: task.Slug}
}
