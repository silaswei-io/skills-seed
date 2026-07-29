package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexExecArgs_UseCurrentWorkDirMode(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())

	args := codexExecArgs(false, "/tmp/output-schema.json")

	require.Equal(t, []string{
		"--ask-for-approval", "never",
		"exec",
		"--skip-git-repo-check",
		"--ephemeral",
		"--ignore-rules",
		"--sandbox", "read-only",
		"--color", "never",
		"--json",
		"--output-schema", "/tmp/output-schema.json",
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

	args := codexExecArgs(false, "/tmp/output-schema.json")

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

	args := codexExecArgs(true, "/tmp/output-schema.json")

	require.NotContains(t, args, `plugins."superpowers@openai-curated".enabled=false`)
}

func TestCodexSessionArgsStayReadOnly(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())

	startArgs := codexSessionStartArgs(false, "/tmp/output-schema.json")
	resumeArgs := codexSessionResumeArgs(false, "/tmp/output-schema.json", "thread-123")

	for _, args := range [][]string{startArgs, resumeArgs} {
		require.Contains(t, args, "--ask-for-approval")
		require.Equal(t, "never", requireArgValue(t, args, "--ask-for-approval"))
		require.Contains(t, args, "--sandbox")
		require.Equal(t, "read-only", requireArgValue(t, args, "--sandbox"))
		require.Contains(t, args, "--json")
		require.Contains(t, args, "--output-schema")
	}
	require.Contains(t, resumeArgs, "resume")
	require.Contains(t, resumeArgs, "thread-123")
	require.NotContains(t, startArgs, "--ephemeral")
	require.NotContains(t, resumeArgs, "--ephemeral")
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

func TestExtractFinalContent_MergesDistinctAgentMessages(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","content":"{\"patterns\":["}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","content":"{\"id\":\"p1\"}]}"}}`

	content, err := extractFinalContent(output)

	require.NoError(t, err)
	require.Equal(t, "{\"patterns\":[\n{\"id\":\"p1\"}]}", content)
}

func TestExtractFinalContent_IgnoresCommandExecutionOutput(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","content":"final answer"}}
{"type":"item.completed","item":{"id":"item_2","type":"command_execution","aggregated_output":"not the final answer"}}`

	content, err := extractFinalContent(output)

	require.NoError(t, err)
	require.Equal(t, "final answer", content)
}

func TestExtractFinalContent_PrefersLastJSONMessageOverProgressMessages(t *testing.T) {
	output := `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"我会先读取源码证据。"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"继续补充邻近定义。"}}
{"type":"item.completed","item":{"id":"item_3","type":"agent_message","text":"{\"patterns\":[],\"profile_refresh_recommended\":{\"needed\":false}}"}}`

	content, err := extractFinalContent(output)

	require.NoError(t, err)
	require.Equal(t, `{"patterns":[],"profile_refresh_recommended":{"needed":false}}`, content)
}

func requireArgValue(t *testing.T, args []string, name string) string {
	t.Helper()
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("argument %s not found in %v", name, args)
	return ""
}
