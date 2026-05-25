package tokenusage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractClaudeJSONUsage(t *testing.T) {
	output := `{
  "type": "result",
  "result": "{\"ok\":true}",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 5,
    "cache_creation_input_tokens": 2,
    "cache_read_input_tokens": 3
  },
  "total_cost": 0.0123
}`

	usage := Extract(output)

	require.True(t, usage.Known())
	require.EqualValues(t, 10, usage.InputTokens)
	require.EqualValues(t, 5, usage.OutputTokens)
	require.EqualValues(t, 20, usage.TotalTokens)
	require.EqualValues(t, 2, usage.CacheCreationInputTokens)
	require.EqualValues(t, 3, usage.CacheReadInputTokens)
	require.True(t, usage.HasCost)
	require.InDelta(t, 0.0123, usage.CostUSD, 0.000001)
}

func TestExtractCodexJSONLUsage(t *testing.T) {
	output := `{"type":"thread.started"}
{"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":25,"input_tokens_details":{"cached_tokens":40},"output_tokens_details":{"reasoning_tokens":7}}}}`

	usage := Extract(output)

	require.True(t, usage.Known())
	require.EqualValues(t, 100, usage.InputTokens)
	require.EqualValues(t, 25, usage.OutputTokens)
	require.EqualValues(t, 125, usage.TotalTokens)
	require.EqualValues(t, 40, usage.CacheReadInputTokens)
	require.EqualValues(t, 7, usage.ReasoningTokens)
}

func TestExtractCodexTokenCountEventAfterLargeJSONLLine(t *testing.T) {
	output := strings.Join([]string{
		`{"type":"thread.started"}`,
		`{"type":"item.completed","item":{"type":"agent_message","content":"` + strings.Repeat("x", 70*1024) + `"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":15854,"cached_input_tokens":10624,"output_tokens":454,"reasoning_output_tokens":199,"total_tokens":16308},"last_token_usage":{"input_tokens":15854,"cached_input_tokens":10624,"output_tokens":454,"reasoning_output_tokens":199,"total_tokens":16308}}}}`,
	}, "\n")

	usage := Extract(output)

	require.True(t, usage.Known())
	require.EqualValues(t, 15854, usage.InputTokens)
	require.EqualValues(t, 454, usage.OutputTokens)
	require.EqualValues(t, 16308, usage.TotalTokens)
	require.EqualValues(t, 10624, usage.CacheReadInputTokens)
	require.EqualValues(t, 199, usage.ReasoningTokens)
}

func TestRecordAggregatesKnownUsage(t *testing.T) {
	Reset()

	first := Record(Usage{InputTokens: 10, OutputTokens: 5})
	second := Record(Usage{InputTokens: 3, OutputTokens: 2, CostUSD: 0.01, HasCost: true})

	require.EqualValues(t, 15, first.TotalTokens)
	require.EqualValues(t, 20, second.TotalTokens)
	require.EqualValues(t, 13, second.InputTokens)
	require.EqualValues(t, 7, second.OutputTokens)
	require.True(t, second.HasCost)
	require.InDelta(t, 0.01, second.CostUSD, 0.000001)
}

func TestExtractIgnoresNonUsageTotals(t *testing.T) {
	usage := Extract(`{"type":"progress","total":4,"input":1,"output":2}`)

	require.False(t, usage.Known())
}

func TestFormatCount(t *testing.T) {
	require.Equal(t, "999", FormatCount(999))
	require.Equal(t, "1k", FormatCount(1_000))
	require.Equal(t, "12.3k", FormatCount(12_345))
	require.Equal(t, "1.2m", FormatCount(1_234_567))
	require.Equal(t, "-1.5k", FormatCount(-1_500))
}

func TestFormatCostUSD(t *testing.T) {
	require.Equal(t, "$0", FormatCostUSD(0))
	require.Equal(t, "$0.0012", FormatCostUSD(0.001234))
	require.Equal(t, "$1.23", FormatCostUSD(1.234))
}
