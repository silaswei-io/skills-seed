package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
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

func TestClaudeSessionPrintArgsUsePersistentSession(t *testing.T) {
	outputSchema := `{"type":"object"}`

	startArgs := claudeSessionPrintArgs(false, outputSchema, false, "session-123", false)
	resumeArgs := claudeSessionPrintArgs(false, outputSchema, false, "session-123", true)

	require.NotContains(t, startArgs, "--no-session-persistence")
	require.Contains(t, startArgs, "--session-id")
	require.Equal(t, "session-123", requireArgValue(t, startArgs, "--session-id"))
	require.NotContains(t, resumeArgs, "--no-session-persistence")
	require.Contains(t, resumeArgs, "--resume")
	require.Equal(t, "session-123", requireArgValue(t, resumeArgs, "--resume"))
	require.Equal(t, "Read,Glob,Grep,LS", requireArgValue(t, resumeArgs, "--tools"))
}

func TestParseClaudeOutputExtractsStructuredOutput(t *testing.T) {
	output, outputErr := parseClaudeOutput(`{
  "type": "result",
  "result": "malformed free-form fallback",
  "structured_output": {"patterns": []}
}`)

	require.Nil(t, outputErr)
	require.Equal(t, `{"patterns": []}`, output)
}

func TestParseClaudeOutputUsesJSONResultWhenStructuredOutputMissing(t *testing.T) {
	rawOutput := `{"type":"result","subtype":"success","is_error":false,"result":"{\"focuses\":[{\"focus_id\":\"core-server-infrastructure\",\"patterns\":[]}]}"}`

	output, outputErr := parseClaudeOutput(rawOutput)

	require.Nil(t, outputErr)
	require.Equal(t, `{"focuses":[{"focus_id":"core-server-infrastructure","patterns":[]}]}`, output)
}

func TestParseClaudeOutputUsesFencedJSONResultWhenStructuredOutputMissing(t *testing.T) {
	rawOutput := "{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"```json\\n{\\\"focuses\\\":[{\\\"focus_id\\\":\\\"plugin-architecture\\\",\\\"patterns\\\":[]}]}\\n```\"}"

	output, outputErr := parseClaudeOutput(rawOutput)

	require.Nil(t, outputErr)
	require.Equal(t, `{"focuses":[{"focus_id":"plugin-architecture","patterns":[]}]}`, output)
}

func TestParseClaudeOutputExtractsJSONResultFromExplanatoryText(t *testing.T) {
	rawOutput := `{"type":"result","subtype":"success","is_error":false,"result":"I now have a complete picture. Let me compile the source-backed patterns.\n\n{\"focuses\":[{\"focus_id\":\"rbac-policy\",\"patterns\":[]}]}"}`

	output, outputErr := parseClaudeOutput(rawOutput)

	require.Nil(t, outputErr)
	require.Equal(t, `{"focuses":[{"focus_id":"rbac-policy","patterns":[]}]}`, output)
}

func TestParseClaudeOutputRepairsFencedJSONResultWhenStructuredOutputMissing(t *testing.T) {
	rawOutput := "{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"```json\\n{\\\"focuses\\\":[{\\\"focus_id\\\":\\\"plugin-architecture\\\",\\\"patterns\\\":[],}],}\\n```\"}"

	output, outputErr := parseClaudeOutput(rawOutput)

	require.Nil(t, outputErr)
	require.Equal(t, `{"focuses":[{"focus_id":"plugin-architecture","patterns":[]}]}`, output)
}

func TestParseClaudeOutputRejectsFreeFormResult(t *testing.T) {
	rawOutput := `{"type":"result","subtype":"success","is_error":false,"result":"malformed free-form fallback"}`

	output, outputErr := parseClaudeOutput(rawOutput)

	require.Empty(t, output)
	require.ErrorContains(t, outputErr, i18n.Get("AgentClaudeStructuredOutputMissing"))
}

func TestParseClaudeOutput_RejectsErrorEnvelope(t *testing.T) {
	output, outputErr := parseClaudeOutput(`{
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
