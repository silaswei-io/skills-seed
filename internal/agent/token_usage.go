package agent

import (
	"context"
	"strings"
	"sync"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
)

type tokenUsageScopeKey struct{}

type tokenUsageScope struct {
	label   string
	mu      sync.Mutex
	entries []tokenUsageEntry
}

type tokenUsageEntry struct {
	message string
	fields  []any
}

// WithTokenUsageScope 创建令牌消耗输出作用域，用于并发场景下延迟输出并保留归属信息
func WithTokenUsageScope(ctx context.Context, label string) context.Context {
	label = strings.TrimSpace(label)
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, tokenUsageScopeKey{}, &tokenUsageScope{label: label})
}

// FlushTokenUsageScope 输出并清空当前作用域中缓存的令牌消耗日志
func FlushTokenUsageScope(ctx context.Context) {
	scope := tokenUsageScopeFromContext(ctx)
	if scope == nil {
		return
	}
	for _, entry := range scope.drain() {
		logger.InfoAfterProgress(entry.message, entry.fields...)
	}
}

// LogTokenUsage 记录单次调用和累计令牌消耗；只有命令行返回精确结构化数值时才输出
func LogTokenUsage(agentName, operation string, usage tokenusage.Usage) {
	LogTokenUsageForContext(context.Background(), agentName, operation, usage)
}

// LogTokenUsageForContext 记录单次调用和累计令牌消耗；带作用域时控制台输出延迟到作用域刷新
func LogTokenUsageForContext(ctx context.Context, agentName, operation string, usage tokenusage.Usage) {
	usage = usage.Normalize()
	if !usage.Known() {
		return
	}

	total := tokenusage.Record(usage)
	fields := []any{
		"agent", agentName,
		"operation", operation,
	}
	fields = append(fields, tokenusage.Fields(usage, "")...)
	fields = append(fields, tokenusage.Fields(total, "cumulative_")...)
	scope := tokenUsageScopeFromContext(ctx)
	if scope != nil {
		fields = append(fields, "token_scope", scope.label)
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentTokenUsage"), fields...)
	message := tokenUsageConsoleMessageWithScope("", agentName, operation, usage, total)
	if scope != nil {
		message = tokenUsageConsoleMessageWithScope(scope.label, agentName, operation, usage, total)
		scope.append(tokenUsageEntry{
			message: message,
			fields:  append([]any(nil), fields...),
		})
		return
	}
	logger.InfoAfterProgress(message, fields...)
}

// LogTokenUsageSummary 输出累计令牌消耗汇总
func LogTokenUsageSummary() {
	total := tokenusage.Snapshot().Normalize()
	if !total.Known() {
		return
	}

	fields := tokenusage.Fields(total, "cumulative_")
	logger.Info(tokenUsageSummaryConsoleMessage(total), fields...)
}

func tokenUsageConsoleMessage(agentName, operation string, usage, total tokenusage.Usage) string {
	return tokenUsageConsoleMessageWithScope("", agentName, operation, usage, total)
}

func tokenUsageConsoleMessageWithScope(scope, agentName, operation string, usage, total tokenusage.Usage) string {
	usage = usage.Normalize()
	total = total.Normalize()

	params := map[string]interface{}{
		"Scope":               scope,
		"Agent":               agentName,
		"Operation":           operation,
		"Current":             tokenusage.FormatCount(usage.UncachedTokens()),
		"CurrentProcessed":    tokenusage.FormatCount(usage.TotalTokens),
		"Detail":              tokenUsageDetail(usage),
		"Cumulative":          tokenusage.FormatCount(total.UncachedTokens()),
		"CumulativeProcessed": tokenusage.FormatCount(total.TotalTokens),
		"Cost":                tokenUsageCost(usage, total),
	}
	if strings.TrimSpace(scope) != "" {
		return i18n.GetWithParams("AgentTokenUsageConsoleScoped", params)
	}
	return i18n.GetWithParams("AgentTokenUsageConsole", params)
}

func tokenUsageSummaryConsoleMessage(total tokenusage.Usage) string {
	total = total.Normalize()
	return i18n.GetWithParams("AgentTokenUsageSummaryConsole", map[string]interface{}{
		"Total":     tokenusage.FormatCount(total.UncachedTokens()),
		"Processed": tokenusage.FormatCount(total.TotalTokens),
		"Detail":    tokenUsageDetail(total),
		"Cost":      tokenUsageSummaryCost(total),
	})
}

func tokenUsageDetail(usage tokenusage.Usage) string {
	parts := []string{
		i18n.GetWithParams("AgentTokenUsageInputDetail", map[string]interface{}{
			"Value": tokenusage.FormatCount(usage.InputTokens),
		}),
		i18n.GetWithParams("AgentTokenUsageOutputDetail", map[string]interface{}{
			"Value": tokenusage.FormatCount(usage.OutputTokens),
		}),
	}
	if usage.CacheReadInputTokens > 0 {
		parts = append(parts, i18n.GetWithParams("AgentTokenUsageCacheReadDetail", map[string]interface{}{
			"Value": tokenusage.FormatCount(usage.CacheReadInputTokens),
		}))
	}
	if usage.CacheCreationInputTokens > 0 {
		parts = append(parts, i18n.GetWithParams("AgentTokenUsageCacheWriteDetail", map[string]interface{}{
			"Value": tokenusage.FormatCount(usage.CacheCreationInputTokens),
		}))
	}
	if usage.ReasoningTokens > 0 {
		parts = append(parts, i18n.GetWithParams("AgentTokenUsageReasoningDetail", map[string]interface{}{
			"Value": tokenusage.FormatCount(usage.ReasoningTokens),
		}))
	}
	return strings.Join(parts, i18n.Get("AgentTokenUsageDetailSeparator"))
}

func tokenUsageCost(usage, total tokenusage.Usage) string {
	if !usage.HasCost && !total.HasCost {
		return ""
	}
	return i18n.GetWithParams("AgentTokenUsageCostSuffix", map[string]interface{}{
		"CurrentCost":    tokenusage.FormatCostUSD(usage.CostUSD),
		"CumulativeCost": tokenusage.FormatCostUSD(total.CostUSD),
	})
}

func tokenUsageSummaryCost(total tokenusage.Usage) string {
	if !total.HasCost {
		return ""
	}
	return i18n.GetWithParams("AgentTokenUsageSummaryCostSuffix", map[string]interface{}{
		"Cost": tokenusage.FormatCostUSD(total.CostUSD),
	})
}

func tokenUsageScopeFromContext(ctx context.Context) *tokenUsageScope {
	if ctx == nil {
		return nil
	}
	scope, _ := ctx.Value(tokenUsageScopeKey{}).(*tokenUsageScope)
	return scope
}

func (s *tokenUsageScope) append(entry tokenUsageEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = append(s.entries, entry)
}

func (s *tokenUsageScope) drain() []tokenUsageEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries := append([]tokenUsageEntry(nil), s.entries...)
	s.entries = nil
	return entries
}
