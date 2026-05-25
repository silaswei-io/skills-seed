package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexExecArgs_UseCurrentWorkDirMode(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())

	args := codexExecArgs(false)

	require.Equal(t, []string{
		"--ask-for-approval", "never",
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"--ignore-rules",
		"--sandbox", "read-only",
		"--color", "never",
		"--json",
		"-",
	}, args)
}

func TestCodexExecArgs_DisablesUserPluginsByDefault(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(`
[plugins."superpowers@openai-curated"]
enabled = true

[plugins.local]
enabled = true
`), 0o644))

	args := codexExecArgs(false)

	require.Contains(t, args, `plugins."superpowers@openai-curated".enabled=false`)
	require.Contains(t, args, `plugins."local".enabled=false`)
}

func TestCodexExecArgs_AllowsUserPluginsWhenConfigured(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(`
[plugins."superpowers@openai-curated"]
enabled = true
`), 0o644))

	args := codexExecArgs(true)

	require.NotContains(t, args, `plugins."superpowers@openai-curated".enabled=false`)
}

func TestExtractFinalContent_NoFinalMessage(t *testing.T) {
	_, err := extractFinalContent(`{"msg_type":"task_started"}`)
	require.Error(t, err)
}

func TestExtractFinalContent_CodexItemCompletedAgentMessage(t *testing.T) {
	output := `{"type":"thread.started","thread_id":"thread_1"}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","content":"{\"patterns\":[]}"}}`

	content, err := extractFinalContent(output)

	require.NoError(t, err)
	require.Equal(t, `{"patterns":[]}`, content)
}

func TestExtractFinalContent_IgnoresCommandExecutionOutput(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","content":"final answer"}}
{"type":"item.completed","item":{"id":"item_2","type":"command_execution","aggregated_output":"not the final answer"}}`

	content, err := extractFinalContent(output)

	require.NoError(t, err)
	require.Equal(t, "final answer", content)
}

func TestCodexReplyPreview_TruncatesAt1000Chars(t *testing.T) {
	preview := codexReplyPreview(strings.Repeat("a", 1001))

	require.Len(t, preview, 1003)
	require.True(t, strings.HasSuffix(preview, "..."))
}
