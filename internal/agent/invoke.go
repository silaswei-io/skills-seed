package agent

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/prompts"
)

// StructuredInvoke 描述一次已渲染提示词的结构化 Agent 调用。
type StructuredInvoke struct {
	Operation      string
	Prompt         string
	OutputContract string
	Options        aicontract.StructuredOutputOptions
	Conversation   Conversation
	Runtime        RuntimeTask
}

// StructuredResult 是结构化调用的输出和可选续接会话。
type StructuredResult struct {
	Output       string
	Conversation Conversation
}

// LearningRuntime 是 Claude/Codex 共享学习任务所需的最小运行时端口。
// Provider 只负责渲染器与进程调用；模板选择、契约与解析由 learning 包统一处理。
type LearningRuntime interface {
	Name() string
	PromptRenderer() prompts.Renderer
	InvokeStructured(ctx context.Context, in StructuredInvoke) (StructuredResult, error)
}
