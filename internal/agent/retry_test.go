package agent

import (
	"context"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/require"
)

func TestRetryReasonFromOutputExtractsClaudeAPIError(t *testing.T) {
	stdout := `{"type":"result","result":"API Error: 529 {\"error\":{\"message\":\"[1305][该模型当前访问量过大]\",\"type\":\"overloaded_error\"}}"}`

	reason := RetryReasonFromOutput(stdout, "")

	require.Contains(t, reason, "API Error: 529")
	require.Contains(t, reason, "overloaded_error")
	require.NotContains(t, reason, "\n")
}

func TestHTTPStatusRetryableRegexRequiresHTTPContext(t *testing.T) {
	require.False(t, HTTPStatusRetryableRegex.MatchString("line 429 in generated output"))
	require.False(t, HTTPStatusRetryableRegex.MatchString("port 503 is used by the test server"))

	require.True(t, HTTPStatusRetryableRegex.MatchString("HTTP 429 too many requests"))
	require.True(t, HTTPStatusRetryableRegex.MatchString("status: 503 service unavailable"))
	require.True(t, HTTPStatusRetryableRegex.MatchString("HTTP/1.1 529 overloaded"))
}

func TestReportRetryForContextInvokesReporterWithNormalizedReason(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	var got RetryInfo
	ctx := WithRetryReporter(context.Background(), func(info RetryInfo) {
		got = info
	})

	ReportRetryForContext(ctx, RetryInfo{
		AgentName:    "claude",
		Operation:    "AnalyzeCurrentCodebase",
		Attempt:      1,
		MaxRetries:   3,
		WaitDuration: 15 * time.Second,
		CallDuration: 217 * time.Second,
		Reason:       "API Error: 529\n overloaded_error",
	})

	require.Equal(t, "claude", got.AgentName)
	require.Equal(t, "AnalyzeCurrentCodebase", got.Operation)
	require.Equal(t, 1, got.Attempt)
	require.Equal(t, 3, got.MaxRetries)
	require.Equal(t, 15*time.Second, got.WaitDuration)
	require.Equal(t, 217*time.Second, got.CallDuration)
	require.Equal(t, "API Error: 529 overloaded_error", got.Reason)
}

func TestRetryProgressLabelShowsErrorDurationAndWait(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	label := RetryProgressLabel("分析当前代码库", RetryInfo{
		Attempt:      1,
		MaxRetries:   3,
		WaitDuration: 15 * time.Second,
		CallDuration: 217 * time.Second,
		Reason:       "API Error: 529 overloaded_error",
	})

	require.Contains(t, label, "分析当前代码库")
	require.Contains(t, label, "API Error: 529 overloaded_error")
	require.NotContains(t, label, "错误原因")
	require.Contains(t, label, "本次调用 3m37s")
	require.Contains(t, label, "15s 后重试")
}

func TestRetryProgressLabelShowsRetryAttemptSeparately(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	label := RetryProgressLabel("分析当前代码库", RetryInfo{
		Status:     RetryProgressStatusAttempt,
		Attempt:    2,
		MaxRetries: 3,
	})

	require.Equal(t, "分析当前代码库（第2次尝试）", label)
}

func TestRetryConsoleMessageShowsRetryReason(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	message := RetryConsoleMessage(RetryInfo{
		AgentName:    "claude",
		Attempt:      1,
		MaxRetries:   3,
		WaitDuration: 15 * time.Second,
		CallDuration: 217 * time.Second,
		Reason:       "API Error: 529\n overloaded_error",
	})

	require.Contains(t, message, "claude 遇到可重试错误")
	require.Contains(t, message, "本次调用 3m37s")
	require.Contains(t, message, "15s 后重试")
	require.Contains(t, message, "API Error: 529 overloaded_error")
	require.NotContains(t, message, "\n")
}

func TestRetryProgressBinderRestoresBaseLabelAfterRecoveredRetry(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	var labels []string
	binder := NewRetryProgressBinder(func(label string) {
		labels = append(labels, label)
	})
	binder.StartStep("分析当前代码库")
	binder.Report(RetryInfo{
		Status:       RetryProgressStatusWaiting,
		Attempt:      1,
		MaxRetries:   3,
		WaitDuration: 15 * time.Second,
		CallDuration: 217 * time.Second,
		Reason:       "API Error: 529 overloaded_error",
	})
	binder.Report(RetryInfo{
		Status:  RetryProgressStatusAttempt,
		Attempt: 2,
	})
	binder.Report(RetryInfo{Status: RetryProgressStatusRecovered})

	require.Equal(t, []string{
		"分析当前代码库（API Error: 529 overloaded_error，本次调用 3m37s，15s 后重试）",
		"分析当前代码库（第2次尝试）",
		"分析当前代码库",
	}, labels)
}
