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
	"github.com/silaswei-io/skills-seed/internal/i18n"
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

	outputSchema := `{"type":"object"}`
	args := claudePrintArgs(false, outputSchema, false)

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
		"--json-schema",
		outputSchema,
		"--settings",
		settings,
		"--tools",
		"Read,Glob,Grep,LS",
	}, args)
}

func TestClaudePrintArgsPromptOnlyDisablesRepositoryTools(t *testing.T) {
	args := claudePrintArgs(false, `{"type":"object"}`, true)

	require.Contains(t, args, "--tools")
	require.Empty(t, requireArgValue(t, args, "--tools"))
}

func TestClaudePrintArgs_AllowsUserPluginsWhenConfigured(t *testing.T) {
	claudeHome := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeHome)
	writeClaudeJSON(t, filepath.Join(claudeHome, "plugins", "installed_plugins.json"), map[string]interface{}{
		"plugins": map[string]interface{}{
			"example-skills@anthropic-agent-skills": []map[string]string{{"scope": "user"}},
		},
	})

	args := claudePrintArgs(true, `{"type":"object"}`, false)

	require.NotContains(t, args, "--setting-sources")
	require.NotContains(t, args, "--settings")
}

func TestClaudeArgsForLogRedactsInlineSchema(t *testing.T) {
	args := claudePrintArgs(true, `{"type":"object"}`, false)

	logged := claudeArgsForLog(args)

	require.Equal(t, `{"type":"object"}`, requireArgValue(t, args, "--json-schema"))
	require.Equal(t, "<schema:17 bytes>", requireArgValue(t, logged, "--json-schema"))
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

func TestAnalyzeProjectReadsStructuredOutput(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0755))

	commandPath := writeFakeClaudeCommand(t, `{"type":"result","structured_output":{"project_name":"demo","common_utils":[{"name":"good"}]}}`)
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
	require.Equal(t, "good", result.CommonUtils[0].Name)
}

func TestSelectFilesStoresPromptAndOutputWithSharedRuntimeName(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0755))

	commandPath := writeFakeClaudeCommand(t, `{"type":"result","structured_output":{"include":["internal/auth/login.go"],"selected_paths":["internal/auth/login.go"],"reason":"auth"}}`)
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

func TestFileSelectionAndAnalysisPlanningKeepRepositoryReadTools(t *testing.T) {
	tests := []struct {
		name   string
		output string
		invoke func(context.Context, *ClaudeAgent, string) error
	}{
		{
			name:   "file-selection",
			output: `{"type":"result","structured_output":{"selected_paths":["internal/auth/login.go"],"reason":"auth"}}`,
			invoke: func(ctx context.Context, ag *ClaudeAgent, projectRoot string) error {
				_, err := ag.SelectFiles(ctx, &agent.SelectFilesRequest{
					FileTree:          "internal/auth/login.go",
					Candidates:        []agent.FileSelectionCandidate{{Path: "internal/auth/login.go", Changed: true}},
					StructuralContext: "internal/auth/login.go defines the login entry",
					CandidateNum:      1,
				})
				return err
			},
		},
		{
			name:   "analysis-planning",
			output: `{"type":"result","structured_output":{"units":[{"id":"auth","name":"Auth","entry_paths":["internal/auth/login.go"]}]}}`,
			invoke: func(ctx context.Context, ag *ClaudeAgent, projectRoot string) error {
				_, err := ag.PlanAnalysisUnits(ctx, &agent.PlanAnalysisUnitsRequest{
					ProjectName:       "demo",
					RootPath:          projectRoot,
					Language:          "go",
					FocusPaths:        []string{"internal/auth/login.go"},
					StructuralContext: "internal/auth/login.go defines the login entry",
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			seedPath := filepath.Join(projectRoot, ".skills-seed")
			require.NoError(t, os.MkdirAll(seedPath, 0755))
			commandPath, argsPath := writeFakeClaudeCommandCapturingArgs(t, tt.output)
			ag := New(commandPath, 5*time.Second, promptloader.New("claude", "zh-CN", seedPath), false, config.DefaultRetryConfig())
			ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

			require.NoError(t, tt.invoke(ctx, ag, projectRoot))

			argsData, err := os.ReadFile(argsPath)
			require.NoError(t, err)
			args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
			require.Equal(t, "Read,Glob,Grep,LS", requireArgValue(t, args, "--tools"))
		})
	}
}

func TestSelectFilesRetryDoesNotPrintFinalFailureWarning(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	successOutput := `{"type":"result","structured_output":{"include":["internal/auth/login.go"],"selected_paths":["internal/auth/login.go"],"reason":"auth"}}`
	commandPath := writeFakeClaudeRetryCommand(t, "API Error: 529 overloaded_error", successOutput)
	loader := promptloader.New("claude", "zh-CN", seedPath)
	ag := New(commandPath, 5*time.Second, loader, false, config.RetryConfig{
		MaxRetries:      1,
		InitialInterval: 1,
		MaxInterval:     1,
	})
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	var result *agent.SelectFilesResult
	output := captureClaudeStdout(t, func() {
		var err error
		result, err = ag.SelectFiles(ctx, &agent.SelectFilesRequest{
			FileTree:     "internal/auth/login.go",
			Candidates:   []agent.FileSelectionCandidate{{Path: "internal/auth/login.go", Changed: true}},
			CandidateNum: 1,
		})
		require.NoError(t, err)
	})

	require.Equal(t, []string{"internal/auth/login.go"}, result.SelectedPaths)
	require.Contains(t, output, "claude 遇到可重试错误")
	require.Contains(t, output, "API Error: 529 overloaded_error")
	require.NotContains(t, output, "Claude CLI 调用失败")
}

func TestParseClaudeOutput_ExtractsStructuredOutputAndTokenUsage(t *testing.T) {
	output, usage, outputErr := parseClaudeOutput(`{
  "type": "result",
  "result": "malformed free-form fallback",
  "structured_output": {"patterns": []},
  "usage": {
    "input_tokens": 10,
    "output_tokens": 5,
    "cache_creation_input_tokens": 2
  },
  "total_cost": 0.01
}`)

	require.Nil(t, outputErr)
	require.Equal(t, `{"patterns": []}`, output)
	require.True(t, usage.Known())
	require.EqualValues(t, 10, usage.InputTokens)
	require.EqualValues(t, 5, usage.OutputTokens)
	require.EqualValues(t, 17, usage.TotalTokens)
	require.True(t, usage.HasCost)
	require.InDelta(t, 0.01, usage.CostUSD, 0.000001)
}

func TestParseClaudeOutput_DoesNotUseFreeFormResult(t *testing.T) {
	rawOutput := `{"type":"result","result":"{\"patterns\":[]}"}`

	output, _, outputErr := parseClaudeOutput(rawOutput)

	require.Empty(t, output)
	require.ErrorContains(t, outputErr, "缺少 structured_output")
}

func TestParseClaudeOutput_RejectsErrorEnvelope(t *testing.T) {
	output, _, outputErr := parseClaudeOutput(`{
  "type": "result",
  "subtype": "error_max_structured_output_retries",
  "is_error": true,
  "errors": ["Structured output validation failed"]
}`)

	require.Empty(t, output)
	require.True(t, outputErr.invocation)
	require.ErrorContains(t, outputErr, "Structured output validation failed")
}

func TestStructuredOutputRetryExhaustionIsRetryable(t *testing.T) {
	require.True(t, isRetryableError(`{"type":"result","subtype":"error_max_structured_output_retries","is_error":true}`, ""))
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

func writeFakeClaudeCommandCapturingArgs(t *testing.T, output string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	argsPath := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\ncat >/dev/null\nprintf '%s' " + shellQuote(output) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	return path, argsPath
}

func writeFakeClaudeRetryCommand(t *testing.T, firstOutput, successOutput string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude")
	countPath := filepath.Join(dir, "count")
	script := "#!/bin/sh\n" +
		"count=0\n" +
		"if [ -f " + shellQuote(countPath) + " ]; then count=$(cat " + shellQuote(countPath) + "); fi\n" +
		"count=$((count + 1))\n" +
		"printf '%s' \"$count\" > " + shellQuote(countPath) + "\n" +
		"cat >/dev/null\n" +
		"if [ \"$count\" = \"1\" ]; then\n" +
		"  printf '%s' " + shellQuote(firstOutput) + "\n" +
		"  exit 1\n" +
		"fi\n" +
		"printf '%s' " + shellQuote(successOutput) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	return path
}

func captureClaudeStdout(t *testing.T, fn func()) string {
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
