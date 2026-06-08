package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

// retryReasonMaxLength 限制终端进度行中的重试原因长度，避免长错误撑开界面。
const retryReasonMaxLength = 220

// whitespacePattern 用于把多行或多空白的错误原因压缩成单行。
var whitespacePattern = regexp.MustCompile(`\s+`)

type retryReporterKey struct{}

type RetryProgressStatus string

const (
	RetryProgressStatusWaiting   RetryProgressStatus = "waiting"
	RetryProgressStatusAttempt   RetryProgressStatus = "attempt"
	RetryProgressStatusRecovered RetryProgressStatus = "recovered"
)

// RetryInfo 描述一次可重试的 Agent CLI 失败及下一次尝试前的状态。
type RetryInfo struct {
	AgentName    string
	Operation    string
	Status       RetryProgressStatus
	Attempt      int
	MaxRetries   int
	WaitDuration time.Duration
	CallDuration time.Duration
	Reason       string
}

// RetryReporter 接收重试事件，让命令进度 UI 可以实时更新。
type RetryReporter func(RetryInfo)

type RetryProgressBinder struct {
	mu           sync.Mutex
	currentLabel string
	retryShown   bool
	update       func(string)
}

func NewRetryProgressBinder(update func(string)) *RetryProgressBinder {
	return &RetryProgressBinder{update: update}
}

func (b *RetryProgressBinder) WithContext(ctx context.Context) context.Context {
	if b == nil {
		return ctx
	}
	existing := retryReporterFromContext(ctx)
	return WithRetryReporter(ctx, func(info RetryInfo) {
		if existing != nil {
			existing(info)
		}
		b.Report(info)
	})
}

func (b *RetryProgressBinder) StartStep(label string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.currentLabel = label
	b.retryShown = false
	b.mu.Unlock()
}

func (b *RetryProgressBinder) FinishStep(label string, succeeded bool) {
	if b == nil {
		return
	}
	var restore bool
	b.mu.Lock()
	if succeeded && b.retryShown {
		restore = true
		b.retryShown = false
	}
	b.currentLabel = ""
	b.mu.Unlock()
	if restore && b.update != nil {
		b.update(label)
	}
}

func (b *RetryProgressBinder) Report(info RetryInfo) {
	if b == nil {
		return
	}
	b.mu.Lock()
	baseLabel := b.currentLabel
	if baseLabel == "" {
		b.mu.Unlock()
		return
	}
	if info.Status == RetryProgressStatusRecovered {
		if !b.retryShown {
			b.mu.Unlock()
			return
		}
		b.retryShown = false
		b.mu.Unlock()
		if b.update != nil {
			b.update(baseLabel)
		}
		return
	}
	b.retryShown = true
	b.mu.Unlock()
	updatedLabel := RetryProgressLabel(baseLabel, info)
	if b.update != nil {
		b.update(updatedLabel)
	}
}

// WithRetryReporter 将重试事件报告器绑定到 ctx。
func WithRetryReporter(ctx context.Context, reporter RetryReporter) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if reporter == nil {
		return ctx
	}
	return context.WithValue(ctx, retryReporterKey{}, reporter)
}

// RetryReasonFromOutput 从 CLI 输出中提取适合终端展示的简短重试原因。
func RetryReasonFromOutput(stdout, stderr string) string {
	reason := retryReasonFromJSON(stdout)
	if reason == "" {
		reason = retryReasonFromJSON(stderr)
	}
	if reason == "" {
		reason = firstNonEmptyLine(stdout)
	}
	if reason == "" {
		reason = firstNonEmptyLine(stderr)
	}
	return truncateRetryReason(normalizeRetryReason(reason))
}

// ReportRetryForContext 上报一次重试事件，并写入结构化诊断日志。
func ReportRetryForContext(ctx context.Context, info RetryInfo) {
	scopeLabel := ""
	if scope := tokenUsageScopeFromContext(ctx); scope != nil {
		scopeLabel = scope.label
	}
	info.Status = RetryProgressStatusWaiting
	info.Reason = truncateRetryReason(normalizeRetryReason(info.Reason))
	if info.Reason == "" {
		info.Reason = i18n.Get("AgentRetryReasonUnknown")
	}

	if reporter := retryReporterFromContext(ctx); reporter != nil {
		reporter(info)
	}
	fields := []any{
		"agent", info.AgentName,
		"operation", info.Operation,
		"attempt", info.Attempt,
		"max_retries", info.MaxRetries,
		"wait_seconds", info.WaitDuration.Seconds(),
		"reason", info.Reason,
		"token_scope", scopeLabel,
	}
	if info.CallDuration > 0 {
		fields = append(fields, "call_duration_seconds", info.CallDuration.Seconds())
	}
	logger.Diagnostic(i18n.Get("LoggerAgentRateLimitRetry"), fields...)
}

func ReportRetryAttemptForContext(ctx context.Context, info RetryInfo) {
	info.Status = RetryProgressStatusAttempt
	if reporter := retryReporterFromContext(ctx); reporter != nil {
		reporter(info)
	}
}

func ReportRetryRecoveredForContext(ctx context.Context, info RetryInfo) {
	info.Status = RetryProgressStatusRecovered
	if reporter := retryReporterFromContext(ctx); reporter != nil {
		reporter(info)
	}
}

func RetryProgressLabel(label string, info RetryInfo) string {
	switch info.Status {
	case RetryProgressStatusAttempt:
		return RetryAttemptProgressLabel(label, info)
	case RetryProgressStatusRecovered:
		return label
	}

	reason := truncateRetryReason(normalizeRetryReason(info.Reason))
	if reason == "" {
		reason = i18n.Get("AgentRetryReasonUnknown")
	}
	params := map[string]interface{}{
		"Label":      label,
		"Reason":     reason,
		"Attempt":    info.Attempt,
		"MaxRetries": info.MaxRetries,
		"Wait":       info.WaitDuration.Truncate(time.Second).String(),
	}
	if info.CallDuration > 0 {
		params["CallDuration"] = info.CallDuration.Truncate(time.Second).String()
		return i18n.GetWithParams("AgentRetryProgressNoteWithDuration", params)
	}
	return i18n.GetWithParams("AgentRetryProgressNote", params)
}

func RetryAttemptProgressLabel(label string, info RetryInfo) string {
	if info.Attempt <= 1 {
		return label
	}
	return i18n.GetWithParams("AgentRetryAttemptProgressNote", map[string]interface{}{
		"Label":   label,
		"Attempt": info.Attempt,
	})
}

func retryReporterFromContext(ctx context.Context) RetryReporter {
	if ctx == nil {
		return nil
	}
	reporter, _ := ctx.Value(retryReporterKey{}).(RetryReporter)
	return reporter
}

func retryReasonFromJSON(output string) string {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var value interface{}
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			continue
		}
		if reason := retryReasonFromValue(value); reason != "" {
			return reason
		}
	}
	return ""
}

func retryReasonFromValue(value interface{}) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, key := range []string{"result", "message", "error", "stderr", "detail"} {
			if reason := retryReasonFromValue(typed[key]); reason != "" {
				return reason
			}
		}
		return retryReasonFromErrorMap(typed)
	case []interface{}:
		for _, item := range typed {
			if reason := retryReasonFromValue(item); reason != "" {
				return reason
			}
		}
	case string:
		return typed
	}
	return ""
}

func retryReasonFromErrorMap(value map[string]interface{}) string {
	errorValue, hasError := value["error"].(map[string]interface{})
	if !hasError {
		return ""
	}
	message, _ := errorValue["message"].(string)
	errorType, _ := errorValue["type"].(string)
	code := fmt.Sprint(errorValue["code"])
	parts := make([]string, 0, 3)
	if code != "" && code != "<nil>" {
		parts = append(parts, code)
	}
	if errorType != "" {
		parts = append(parts, errorType)
	}
	if message != "" {
		parts = append(parts, message)
	}
	return strings.Join(parts, ": ")
}

func firstNonEmptyLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeRetryReason(reason string) string {
	reason = strings.TrimSpace(reason)
	reason = strings.Trim(reason, "\"")
	return whitespacePattern.ReplaceAllString(reason, " ")
}

func truncateRetryReason(reason string) string {
	runes := []rune(reason)
	if len(runes) <= retryReasonMaxLength {
		return reason
	}
	return string(runes[:retryReasonMaxLength]) + "..."
}
