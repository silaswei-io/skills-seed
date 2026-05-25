package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/stretchr/testify/require"
)

func TestClaudePrintArgs_ReadOnlyToolsAndUserPluginsDisabledByDefault(t *testing.T) {
	claudeHome := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeHome)
	writeClaudeJSON(t, filepath.Join(claudeHome, "plugins", "installed_plugins.json"), map[string]interface{}{
		"plugins": map[string]interface{}{
			"example-skills@anthropic-agent-skills": []map[string]string{{"scope": "user"}},
			"project-tool@demo-marketplace":         []map[string]string{{"scope": "project"}},
		},
	})
	writeClaudeJSON(t, filepath.Join(claudeHome, "settings.json"), map[string]interface{}{
		"enabledPlugins": map[string]interface{}{
			"manual-user-plugin@demo-marketplace": true,
			"builtin-tool@builtin":                true,
		},
	})

	args := claudePrintArgs(false)

	require.NotContains(t, args, "--setting-sources")
	settings := requireArgValue(t, args, "--settings")
	var settingsJSON struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	require.NoError(t, json.Unmarshal([]byte(settings), &settingsJSON))
	require.Equal(t, map[string]bool{
		"example-skills@anthropic-agent-skills": false,
		"manual-user-plugin@demo-marketplace":   false,
	}, settingsJSON.EnabledPlugins)

	require.Equal(t, []string{
		"--print",
		"--no-session-persistence",
		"--disable-slash-commands",
		"--output-format",
		"json",
		"--settings",
		settings,
		"--tools",
		"Read,Glob,Grep,LS",
	}, args)
}

func TestClaudePrintArgs_AllowsUserPluginsWhenConfigured(t *testing.T) {
	claudeHome := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeHome)
	writeClaudeJSON(t, filepath.Join(claudeHome, "plugins", "installed_plugins.json"), map[string]interface{}{
		"plugins": map[string]interface{}{
			"example-skills@anthropic-agent-skills": []map[string]string{{"scope": "user"}},
		},
	})

	args := claudePrintArgs(true)

	require.NotContains(t, args, "--setting-sources")
	require.NotContains(t, args, "--settings")
}

func TestAnalyzeCodeReturnsErrorWhenClaudeCommandMissing(t *testing.T) {
	loader := prompts.NewLoader("claude", "en-US", "")
	ag := New("__skills_seed_missing_claude__", time.Second, loader)

	_, err := ag.AnalyzeCode(context.Background(), &agent.AnalyzeRequest{
		Files: []domain.FileInfo{
			domain.NewFileInfo("main.go", "package main\n"),
		},
		Context: agent.ProjectContext{Name: "demo", Language: "go"},
	})

	require.Error(t, err)
}

func TestClaudeReplyPreview_TruncatesAt1000Chars(t *testing.T) {
	preview := claudeReplyPreview(strings.Repeat("a", 1001))

	require.Len(t, preview, 1003)
	require.True(t, strings.HasSuffix(preview, "..."))
}

func TestParseClaudeOutput_ExtractsResultAndTokenUsage(t *testing.T) {
	output, usage := parseClaudeOutput(`{
  "type": "result",
  "result": "{\"patterns\":[]}",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 5,
    "cache_creation_input_tokens": 2
  },
  "total_cost": 0.01
}`)

	require.Equal(t, `{"patterns":[]}`, output)
	require.True(t, usage.Known())
	require.EqualValues(t, 10, usage.InputTokens)
	require.EqualValues(t, 5, usage.OutputTokens)
	require.EqualValues(t, 17, usage.TotalTokens)
	require.True(t, usage.HasCost)
	require.InDelta(t, 0.01, usage.CostUSD, 0.000001)
}

func writeClaudeJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
}

func requireArgValue(t *testing.T, args []string, name string) string {
	t.Helper()
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("missing arg %s in %#v", name, args)
	return ""
}
