package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
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
	loader := promptloader.New("claude", "en-US", "")
	ag := New("__skills_seed_missing_claude__", time.Second, loader, false, config.DefaultRetryConfig())

	_, err := ag.AnalyzeCode(context.Background(), &agent.AnalyzeRequest{
		Files: []domain.FileInfo{
			domain.NewFileInfo("main.go", "package main\n"),
		},
		Context: agent.ProjectContext{Name: "demo", Language: "go"},
	})

	require.Error(t, err)
}

func TestAnalyzeProjectPassesStructuralContextToTemplate(t *testing.T) {
	loader := promptloader.New("claude", "zh-CN", "")
	ag := New("__skills_seed_missing_claude__", time.Second, loader, false, config.DefaultRetryConfig())

	_, err := ag.AnalyzeProject(context.Background(), &agent.AnalyzeProjectRequest{
		ProjectName:       "demo",
		RootPath:          t.TempDir(),
		Language:          "go",
		Structure:         "main.go",
		StructuralContext: "## Structural Context\n- handler symbol",
		MainFiles:         []string{"main.go"},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "__skills_seed_missing_claude__")
	require.NotContains(t, err.Error(), "StructuralContext")
	require.NotContains(t, err.Error(), "project-profile prompt")
}

func TestAnalyzeProjectRenderErrorIncludesTemplateReason(t *testing.T) {
	renderErr := fmt.Errorf("template: project-profile:18:7: missing StructuralContext")
	ag := &ClaudeAgent{
		commandPath:  "__skills_seed_missing_claude__",
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

func TestAnalyzeProjectRepairsMalformedModelJSON(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0755))

	commandPath := writeFakeClaudeCommand(t, `{"type":"result","result":"{project_name:'demo', common_utils:[{name:'bad',}],}"}`)
	loader := promptloader.New("claude", "zh-CN", "")
	ag := New(commandPath, 5*time.Second, loader, false, config.DefaultRetryConfig())
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	result, err := ag.AnalyzeProject(ctx, &agent.AnalyzeProjectRequest{
		ProjectName: "demo",
		RootPath:    projectRoot,
		Language:    "go",
		Structure:   "main.go",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "demo", result.ProjectName)
	require.Len(t, result.CommonUtils, 1)
	require.Equal(t, "bad", result.CommonUtils[0].Name)
}

func TestSelectFilesStoresPromptAndOutputWithSharedRuntimeName(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0755))

	commandPath := writeFakeClaudeCommand(t, `{"type":"result","result":"{\"include\":[\"internal/auth/login.go\"],\"selected_paths\":[\"internal/auth/login.go\"],\"reason\":\"auth\"}"}`)
	loader := promptloader.New("claude", "zh-CN", seedPath)
	ag := New(commandPath, 5*time.Second, loader, false, config.DefaultRetryConfig())
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	_, err := ag.SelectFiles(ctx, &agent.SelectFilesRequest{
		FileTree:     "internal/auth/login.go",
		Candidates:   []agent.FileSelectionCandidate{{Path: "internal/auth/login.go", Changed: true}},
		CandidateNum: 1,
	})
	require.NoError(t, err)

	promptEntries, err := os.ReadDir(filepath.Join(seedPath, "runtime", "rendered-prompts"))
	require.NoError(t, err)
	outputEntries, err := os.ReadDir(filepath.Join(seedPath, "runtime", "agent-outputs"))
	require.NoError(t, err)

	var promptName, outputName string
	for _, entry := range promptEntries {
		if strings.HasSuffix(entry.Name(), ".md") {
			promptName = entry.Name()
		}
	}
	for _, entry := range outputEntries {
		if strings.HasSuffix(entry.Name(), ".md") {
			outputName = entry.Name()
		}
	}
	require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-file-select\.md$`, promptName)
	require.Equal(t, strings.TrimSuffix(promptName, "-file-select.md")+"-claude-file-select.md", outputName)
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

func writeFakeClaudeCommand(t *testing.T, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s' " + shellQuote(output) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	return path
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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

type failingPromptRenderer struct {
	err error
}

func (f failingPromptRenderer) Render(string, interface{}) (string, error) {
	return "", f.err
}

func (f failingPromptRenderer) RenderForRuntimeTask(string, interface{}, promptloader.RuntimeTask) (string, error) {
	return "", f.err
}
