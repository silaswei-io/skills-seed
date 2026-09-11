package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/require"
)

type retryPolicyStub struct {
	maxRetries int
	wait       time.Duration
}

func (p retryPolicyStub) EffectiveMaxRetries() int {
	return p.maxRetries
}

func (p retryPolicyStub) WaitDuration(int) time.Duration {
	return p.wait
}

func TestRetryReasonFromOutputExtractsClaudeAPIError(t *testing.T) {
	stdout := `{"type":"result","result":"API Error: 529 {\"error\":{\"message\":\"[1305][该模型当前访问量过大]\",\"type\":\"overloaded_error\"}}"}`

	reason := RetryReasonFromOutput(stdout, "")

	require.Contains(t, reason, "API Error: 529")
	require.Contains(t, reason, "overloaded_error")
	require.NotContains(t, reason, "\n")
}

func TestRetryReasonFromOutputExplainsStructuredOutputExhaustion(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	stdout := `{"type":"result","subtype":"error_max_structured_output_retries","is_error":true,"result":""}`

	reason := RetryReasonFromOutput(stdout, "")

	require.Equal(t, "结构化输出多次未通过 JSON Schema 校验（error_max_structured_output_retries）", reason)
}

func TestRetryReasonFromOutputPrefersProviderValidationDetails(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	stdout := `{"type":"result","subtype":"error_max_structured_output_retries","errors":["decisions[2].revision: required property missing"]}`

	reason := RetryReasonFromOutput(stdout, "")

	require.Equal(t, "decisions[2].revision: required property missing", reason)
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

func TestWithAdditionalRetryReporterPreservesExistingReporter(t *testing.T) {
	var reports []string
	ctx := WithRetryReporter(context.Background(), func(RetryInfo) {
		reports = append(reports, "existing")
	})
	ctx = WithAdditionalRetryReporter(ctx, func(RetryInfo) {
		reports = append(reports, "additional")
	})

	ReportRetryAttemptForContext(ctx, RetryInfo{Attempt: 2})

	require.Equal(t, []string{"existing", "additional"}, reports)
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

func TestRetryConsoleMessageShowsDiagnosticsPath(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	message := RetryConsoleMessage(RetryInfo{
		AgentName:       "claude",
		WaitDuration:    15 * time.Second,
		Reason:          "结构化输出多次未通过 JSON Schema 校验",
		DiagnosticsPath: "/tmp/runtime/attempt.manifest.json",
	})

	require.Contains(t, message, "完整诊断：/tmp/runtime/attempt.manifest.json")
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

func TestRunRetryingCallValidatesAndReturnsWithoutRetry(t *testing.T) {
	_, err := RunRetryingCall[string](context.Background(), RetryingCallOptions[string]{})
	require.EqualError(t, err, "agent retry call is nil")

	calls := 0
	got, err := RunRetryingCall(context.Background(), RetryingCallOptions[string]{
		Policy: retryPolicyStub{maxRetries: 3},
		Call: func(attempt int) (string, string, time.Duration, bool, error) {
			calls++
			require.Equal(t, 1, attempt)
			return "ok", "", time.Second, false, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "ok", got)
	require.Equal(t, 1, calls)

	wantErr := errors.New("permanent")
	got, err = RunRetryingCall(context.Background(), RetryingCallOptions[string]{
		Policy: retryPolicyStub{maxRetries: 3},
		Call: func(int) (string, string, time.Duration, bool, error) {
			return "partial", "", 0, false, wantErr
		},
	})
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, "partial", got)
}

func TestRunRetryingCallReportsWaitingAttemptAndRecovery(t *testing.T) {
	var events []RetryInfo
	ctx := WithRetryReporter(context.Background(), func(info RetryInfo) {
		events = append(events, info)
	})
	calls := 0
	got, err := RunRetryingCall(ctx, RetryingCallOptions[string]{
		AgentName: "codex",
		Operation: "analyze",
		Policy:    retryPolicyStub{maxRetries: 2},
		Call: func(attempt int) (string, string, time.Duration, bool, error) {
			calls++
			if attempt == 1 {
				return "failed-result", "HTTP 429\ntry later", 2 * time.Second, true, errors.New("busy")
			}
			return "recovered", "", 3 * time.Second, false, nil
		},
		RetryDetail: func(result string) string {
			return " /tmp/" + result + ".json "
		},
	})

	require.NoError(t, err)
	require.Equal(t, "recovered", got)
	require.Equal(t, 2, calls)
	require.Len(t, events, 3)
	require.Equal(t, RetryProgressStatusWaiting, events[0].Status)
	require.Equal(t, "HTTP 429", events[0].Reason)
	require.Equal(t, "/tmp/failed-result.json", events[0].DiagnosticsPath)
	require.Equal(t, RetryProgressStatusAttempt, events[1].Status)
	require.Equal(t, 2, events[1].Attempt)
	require.Equal(t, RetryProgressStatusRecovered, events[2].Status)
	require.Equal(t, 3*time.Second, events[2].CallDuration)
}

func TestRunRetryingCallStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := RunRetryingCall(ctx, RetryingCallOptions[string]{
		Policy: retryPolicyStub{maxRetries: 1, wait: time.Hour},
		Call: func(int) (string, string, time.Duration, bool, error) {
			return "partial", "rate limit", 0, true, errors.New("busy")
		},
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, got)
}

func TestIsRetryableOutputError(t *testing.T) {
	tests := []struct {
		name         string
		stdout       string
		stderr       string
		extraMarkers []string
		want         bool
	}{
		{name: "http status", stderr: "HTTP/1.1 503 unavailable", want: true},
		{name: "provider marker", stdout: "overloaded_error", want: true},
		{name: "english rate limit", stderr: "rate limit reached", want: true},
		{name: "chinese marker", stderr: "请求频率过高", want: true},
		{name: "extra marker", stdout: "capacity exhausted", extraMarkers: []string{"capacity exhausted"}, want: true},
		{name: "unrelated output", stdout: "port 503 is configured", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsRetryableOutputError(tt.stdout, tt.stderr, tt.extraMarkers...))
		})
	}
}

func TestRetryProgressBinderContextAndLifecycle(t *testing.T) {
	ctx := context.Background()
	require.Equal(t, ctx, (*RetryProgressBinder)(nil).WithContext(ctx))
	(*RetryProgressBinder)(nil).StartStep("ignored")
	(*RetryProgressBinder)(nil).FinishStep("ignored", true)
	(*RetryProgressBinder)(nil).Report(RetryInfo{})

	var reports []string
	ctx = WithRetryReporter(ctx, func(RetryInfo) {
		reports = append(reports, "existing")
	})
	var labels []string
	binder := NewRetryProgressBinder(func(label string) {
		labels = append(labels, label)
	})
	bound := binder.WithContext(ctx)
	binder.Report(RetryInfo{Status: RetryProgressStatusWaiting})
	binder.StartStep("分析")
	ReportRetryAttemptForContext(bound, RetryInfo{Attempt: 2})
	binder.FinishStep("完成", true)
	binder.Report(RetryInfo{Status: RetryProgressStatusRecovered})

	require.Equal(t, []string{"existing"}, reports)
	require.Equal(t, []string{"分析（第2次尝试）", "完成"}, labels)
}

func TestRetryHelpersCoverFallbacksAndBounds(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	var nilContext context.Context
	require.Nil(t, retryReporterFromContext(nilContext))
	require.Empty(t, retryDiagnosticsPath[string](nil, "result"))
	require.Equal(t, "detail", retryDiagnosticsPath(func(string) string { return " detail " }, "result"))
	require.Equal(t, "E42: overloaded: busy", retryReasonFromErrorMap(map[string]interface{}{
		"error": map[string]interface{}{"code": "E42", "type": "overloaded", "message": "busy"},
	}))
	require.Empty(t, retryReasonFromErrorMap(map[string]interface{}{}))
	require.Equal(t, "first", firstNonEmptyLine("\n  first  \nsecond"))
	require.Empty(t, firstNonEmptyLine(" \n\t"))

	reason := strings.Repeat("界", retryReasonMaxLength+10)
	truncated := truncateRetryReason(reason)
	require.Equal(t, retryReasonMaxLength+3, len([]rune(truncated)))
	require.True(t, strings.HasSuffix(truncated, "..."))

	var recovered RetryInfo
	ctx := WithRetryReporter(nilContext, func(info RetryInfo) { recovered = info })
	ReportRetryRecoveredForContext(ctx, RetryInfo{Attempt: 2})
	require.Equal(t, RetryProgressStatusRecovered, recovered.Status)
	require.Same(t, ctx, WithAdditionalRetryReporter(ctx, nil))
	require.NotNil(t, WithAdditionalRetryReporter(context.Background(), func(RetryInfo) {}))

	label := RetryProgressLabel("分析", RetryInfo{DiagnosticsPath: "/tmp/detail.json"})
	require.Contains(t, label, "/tmp/detail.json")
	require.Equal(t, "分析", RetryAttemptProgressLabel("分析", RetryInfo{Attempt: 1}))
	message := RetryConsoleMessage(RetryInfo{})
	require.NotEmpty(t, message)
}
