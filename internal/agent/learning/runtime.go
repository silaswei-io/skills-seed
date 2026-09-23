package learning

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/prompts"
)

// Runtime 是 Claude/Codex 共享学习任务所需的最小运行时端口。
// Provider 只负责渲染器与进程调用；模板选择、契约与解析由本包统一处理。
// 放在 learning 包避免 agent ↔ prompts 在测试中形成导入环。
type Runtime interface {
	Name() string
	PromptRenderer() prompts.Renderer
	InvokeStructured(ctx context.Context, in agent.StructuredInvoke) (agent.StructuredResult, error)
}
