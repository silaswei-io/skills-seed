package agent

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
)

// LogTokenUsage 记录单次调用和累计令牌消耗；只有命令行返回精确结构化数值时才输出
func LogTokenUsage(agentName, operation string, usage tokenusage.Usage) {
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
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentTokenUsage"), fields...)
	logger.Info(tokenUsageConsoleMessage(agentName, operation, usage, total), fields...)
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
	usage = usage.Normalize()
	total = total.Normalize()

	params := map[string]interface{}{
		"Agent":      agentName,
		"Operation":  operation,
		"Current":    tokenusage.FormatCount(usage.TotalTokens),
		"Detail":     tokenUsageDetail(usage),
		"Cumulative": tokenusage.FormatCount(total.TotalTokens),
		"Cost":       tokenUsageCost(usage, total),
	}
	return i18n.GetWithParams("AgentTokenUsageConsole", params)
}

func tokenUsageSummaryConsoleMessage(total tokenusage.Usage) string {
	total = total.Normalize()
	return i18n.GetWithParams("AgentTokenUsageSummaryConsole", map[string]interface{}{
		"Total":  tokenusage.FormatCount(total.TotalTokens),
		"Detail": tokenUsageDetail(total),
		"Cost":   tokenUsageSummaryCost(total),
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
