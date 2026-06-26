package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
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

func TestAnalyzeProjectPassesStructuralContextToTemplate(t *testing.T) {
	loader := promptloader.New("codex", "zh-CN", "")
	ag := New("__skills_seed_missing_codex__", time.Second, loader, false, config.DefaultRetryConfig())

	_, err := ag.AnalyzeProject(context.Background(), &agent.AnalyzeProjectRequest{
		ProjectName:       "demo",
		RootPath:          t.TempDir(),
		Language:          "go",
		Structure:         "main.go",
		StructuralContext: "## Structural Context\n- handler symbol",
		MainFiles:         []string{"main.go"},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "__skills_seed_missing_codex__")
	require.NotContains(t, err.Error(), "StructuralContext")
	require.NotContains(t, err.Error(), "project-profile prompt")
}

func TestAnalyzeProjectRenderErrorIncludesTemplateReason(t *testing.T) {
	renderErr := fmt.Errorf("template: project-profile:18:7: missing StructuralContext")
	ag := &CodexAgent{
		commandPath:  "__skills_seed_missing_codex__",
		timeout:      time.Second,
		promptLoader: failingPromptRenderer{err: renderErr},
	}

	_, err := ag.AnalyzeProject(context.Background(), &agent.AnalyzeProjectRequest{
		ProjectName: "demo",
		RootPath:    t.TempDir(),
		Language:    "go",
		Structure:   "main.go",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "渲染 project-profile prompt 失败")
	require.Contains(t, err.Error(), "template:")
	require.ErrorIs(t, err, renderErr)
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

type failingPromptRenderer struct {
	err error
}

func (f failingPromptRenderer) Render(string, interface{}) (string, error) {
	return "", f.err
}

func (f failingPromptRenderer) RenderForRuntimeTask(string, interface{}, promptloader.RuntimeTask) (string, error) {
	return "", f.err
}
