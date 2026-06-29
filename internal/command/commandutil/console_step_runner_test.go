package commandutil

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/require"
)

func TestConsoleStepRunnerKeepsDetailLabelOnFailure(t *testing.T) {
	runner := NewConsoleStepRunner(ConsoleStepRunnerOptions{TotalSteps: 2})

	err := runner.Run("分析当前代码库", func() error {
		runner.Detail("分析当前代码库", "分析当前代码库 · 单元 1/1 · 认证登录")
		return errors.New("解析结果失败")
	})

	require.Error(t, err)
	require.Equal(t, "分析当前代码库 · 单元 1/1 · 认证登录", runner.DisplayLabel("分析当前代码库"))
}

func TestConsoleStepRunnerUpdatesCallbacksAndCompletesBaseStep(t *testing.T) {
	var starts []string
	var updates []string
	var completes []string
	runner := NewConsoleStepRunner(ConsoleStepRunnerOptions{
		TotalSteps:     1,
		OnStepStart:    func(label string) { starts = append(starts, label) },
		OnStepUpdate:   func(label string) { updates = append(updates, label) },
		OnStepComplete: func(label string) { completes = append(completes, label) },
	})

	err := runner.Run("检测增量文件变化", func() error {
		runner.Detail("检测增量文件变化", "检测增量文件变化：扫描文件指纹")
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, []string{"检测增量文件变化"}, starts)
	require.Equal(t, []string{"检测增量文件变化：扫描文件指纹"}, updates)
	require.Equal(t, []string{"检测增量文件变化"}, completes)
}

func TestConsoleStepRunnerBindsAgentRetryProgressToCurrentDetail(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	var updates []string
	runner := NewConsoleStepRunner(ConsoleStepRunnerOptions{
		TotalSteps:   1,
		OnStepUpdate: func(label string) { updates = append(updates, label) },
	})
	ctx := runner.WithContext(context.Background())

	err := runner.Run("分析当前代码库", func() error {
		runner.Detail("分析当前代码库", "分析当前代码库 · 单元 1/1 · 认证登录")
		agent.ReportRetryForContext(ctx, agent.RetryInfo{
			Attempt:      1,
			MaxRetries:   3,
			WaitDuration: 15 * time.Second,
			CallDuration: 217 * time.Second,
			Reason:       "API Error: 529 overloaded_error",
		})
		agent.ReportRetryAttemptForContext(ctx, agent.RetryInfo{Attempt: 2})
		agent.ReportRetryRecoveredForContext(ctx, agent.RetryInfo{})
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"分析当前代码库 · 单元 1/1 · 认证登录",
		"分析当前代码库 · 单元 1/1 · 认证登录（API Error: 529 overloaded_error，本次调用 3m37s，15s 后重试）",
		"分析当前代码库 · 单元 1/1 · 认证登录（第2次尝试）",
		"分析当前代码库 · 单元 1/1 · 认证登录",
	}, updates)
}
