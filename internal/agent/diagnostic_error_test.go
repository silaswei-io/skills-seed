package agent

import (
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/require"
)

func TestDiagnosticErrorIncludesArchivePathsAndPreview(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	err := NewResultContractError("claude", "AnalyzeCurrentCodebaseBatch/batch-002", errors.New("输出中未找到有效 JSON"), "根据分析，现在让我输出最终结果：", AgentOutputArchive{
		ContentPath:  "/tmp/out.md",
		RawPath:      "/tmp/out.raw.txt",
		StderrPath:   "/tmp/out.stderr.txt",
		ManifestPath: "/tmp/out.manifest.json",
	})

	text := err.Error()
	require.Contains(t, text, "type=result_invalid")
	require.Contains(t, text, "agent=claude")
	require.Contains(t, text, "operation=AnalyzeCurrentCodebaseBatch/batch-002")
	require.Contains(t, text, "output_preview=根据分析")
	require.Contains(t, text, "raw=/tmp/out.raw.txt")
	require.Contains(t, text, "stderr=/tmp/out.stderr.txt")
	require.Contains(t, text, "manifest=/tmp/out.manifest.json")
}
