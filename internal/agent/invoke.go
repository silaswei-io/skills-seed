package agent

import (
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
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
