package agent

import (
	"os"
	"strings"
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
	require.Contains(t, message, "本次非缓存 12.8k")
	require.Contains(t, message, "上下文处理 14.8k")
	require.Contains(t, message, "输入 12k")
	require.Contains(t, message, "输出 345")
	require.Contains(t, message, "缓存读 2k")
	require.Contains(t, message, "缓存写 500")
	require.Contains(t, message, "累计非缓存 1.2m")
	require.NotContains(t, message, "费用")
	require.NotContains(t, message, "$")
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
	require.Contains(t, message, "累计非缓存 1.2m")
	require.Contains(t, message, "输入 1.2m")
	require.Contains(t, message, "输出 34.5k")
	require.NotContains(t, message, "费用")
	require.NotContains(t, message, "$")
}

func TestScopedTokenUsageDefersConsoleOutputUntilFlush(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	output := captureTokenUsageStdout(t, func() {
		ctx := WithTokenUsageScope(t.Context(), "front")
		LogTokenUsageForContext(ctx, "claude", "AnalyzeCurrentCodebase", tokenusage.Usage{
			InputTokens:  1_000,
			OutputTokens: 200,
		})
	})
	require.Empty(t, output)

	output = captureTokenUsageStdout(t, func() {
		ctx := WithTokenUsageScope(t.Context(), "front")
		LogTokenUsageForContext(ctx, "claude", "AnalyzeCurrentCodebase", tokenusage.Usage{
			InputTokens:  1_000,
			OutputTokens: 200,
		})
		FlushTokenUsageScope(ctx)
	})

	require.Contains(t, output, "Token 消耗: 子项目 front claude/AnalyzeCurrentCodebase")
	require.Contains(t, output, "本次非缓存 1.2k")
	require.Equal(t, 1, strings.Count(output, "Token 消耗:"))
}

func TestScopedTokenUsageMessageIncludesProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	message := tokenUsageConsoleMessageWithScope("backend", "codex", "AnalyzeProject",
		tokenusage.Usage{
			InputTokens:  2_000,
			OutputTokens: 500,
		},
		tokenusage.Usage{
			InputTokens:  2_000,
			OutputTokens: 500,
			HasTokens:    true,
		},
	)

	require.Contains(t, message, "子项目 backend")
	require.Contains(t, message, "codex/AnalyzeProject")
}

func TestTokenUsageScopeCanDeferWithoutScopeLabel(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	output := captureTokenUsageStdout(t, func() {
		ctx := WithTokenUsageScope(t.Context(), "")
		LogTokenUsageForContext(ctx, "mock", "AnalyzeCurrentCodebase", tokenusage.Usage{
			InputTokens:  100,
			OutputTokens: 20,
		})
		FlushTokenUsageScope(ctx)
	})

	require.Contains(t, output, "Token 消耗: mock/AnalyzeCurrentCodebase")
	require.NotContains(t, output, "子项目")
}

func captureTokenUsageStdout(t *testing.T, fn func()) string {
	t.Helper()

	tempFile, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)

	originalStdout := os.Stdout
	os.Stdout = tempFile
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	require.NoError(t, tempFile.Close())
	data, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	return string(data)
}
