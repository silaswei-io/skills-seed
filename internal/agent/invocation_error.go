package agent

import (
	"context"
	"fmt"
	"time"
)

// NormalizeInvocationError 将 CommandContext 的进程信号还原成调用超时或取消语义。
func NormalizeInvocationError(runErr, contextErr error, timeout time.Duration) error {
	switch contextErr {
	case context.DeadlineExceeded:
		return fmt.Errorf("agent invocation timed out after %s: %w", timeout, contextErr)
	case context.Canceled:
		return fmt.Errorf("agent invocation canceled: %w", contextErr)
	default:
		return runErr
	}
}
