package agent

import "strings"

// Conversation 标识一次可续接的 Agent 会话。
// 它只在同一学习焦点的连续任务间传递，不进入持久化检查点。
type Conversation struct {
	Provider string
	ID       string
}

// Valid 返回会话是否可用于续接。
func (c Conversation) Valid() bool {
	return strings.TrimSpace(c.Provider) != "" && strings.TrimSpace(c.ID) != ""
}
