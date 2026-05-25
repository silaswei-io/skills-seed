package agent

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	"github.com/stretchr/testify/require"
)

func TestTokenUsageConsoleMessageUsesCompactUnits(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	message := tokenUsageConsoleMessage("codex", "AnalyzeProject",
		tokenusage.Usage{
			InputTokens:              12_000,
			OutputTokens:             345,
			CacheReadInputTokens:     2_000,
			CacheCreationInputTokens: 500,
		},
		tokenusage.Usage{
			InputTokens:  1_200_000,
			OutputTokens: 34_500,
			HasTokens:    true,
		},
	)

	require.Contains(t, message, "codex/AnalyzeProject")
	require.Contains(t, message, "本次 14.8k")
	require.Contains(t, message, "输入 12k")
	require.Contains(t, message, "输出 345")
	require.Contains(t, message, "缓存读 2k")
	require.Contains(t, message, "缓存写 500")
	require.Contains(t, message, "累计 1.2m")
}

func TestTokenUsageSummaryConsoleMessageUsesCompactUnits(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	message := tokenUsageSummaryConsoleMessage(tokenusage.Usage{
		InputTokens:  1_200_000,
		OutputTokens: 34_500,
		CostUSD:      1.234,
		HasCost:      true,
	})

	require.Contains(t, message, "Token 消耗汇总")
	require.Contains(t, message, "累计 1.2m")
	require.Contains(t, message, "输入 1.2m")
	require.Contains(t, message, "输出 34.5k")
	require.Contains(t, message, "费用 $1.23")
}
