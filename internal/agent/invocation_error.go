package agent

import (
	"context"
	"errors"
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

// IsRetryableInvocationError 判断调用是否因 provider 执行超时而可重试。
// 上层主动取消代表用户意图，不应自动发起新调用。
func IsRetryableInvocationError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
