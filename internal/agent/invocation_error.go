package agent

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// NormalizeInvocationError 将进程信号还原成超时或取消语义；elapsed 是实际调用耗时，包含父上下文提前到期。
func NormalizeInvocationError(runErr, contextErr error, elapsed time.Duration) error {
	switch contextErr {
	case context.DeadlineExceeded:
		return fmt.Errorf("agent invocation timed out after %s: %w", elapsed, contextErr)
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
