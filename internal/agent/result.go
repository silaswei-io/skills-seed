package agent

import "fmt"

// RequireResult 把 Agent 返回的 nil 结果统一视为契约错误。
func RequireResult[T any](result *T, operation string) error {
	if result != nil {
		return nil
	}
	return fmt.Errorf("agent %s returned nil result", operation)
}
