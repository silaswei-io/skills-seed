package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts"
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
	args := claudePrintArgs(false, outputSchema, false, config.AgentRuntimeOptions{})

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
		"--strict-mcp-config",
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
	args := claudePrintArgs(false, `{"type":"object"}`, true, config.AgentRuntimeOptions{})

	require.Contains(t, args, "--tools")
	require.Empty(t, requireArgValue(t, args, "--tools"))
}

func TestClaudePrintArgsForConversationResumesWithoutDisablingPersistence(t *testing.T) {
	conversation := agent.Conversation{Provider: "provider", ID: "00000000-0000-4000-8000-000000000001"}
	args := claudePrintArgsForConversation(false, `{"type":"object"}`, false, config.AgentRuntimeOptions{}, conversation)

	require.NotContains(t, args, "--no-session-persistence")
	require.Equal(t, conversation.ID, requireArgValue(t, args, "--resume"))
}

func TestClaudeConversationReadsNewSessionID(t *testing.T) {
	conversation := claudeConversation(`{"type":"result","session_id":"00000000-0000-4000-8000-000000000001"}`, "provider", agent.Conversation{Provider: "provider"})

	require.Equal(t, agent.Conversation{Provider: "provider", ID: "00000000-0000-4000-8000-000000000001"}, conversation)
}

func TestClaudePrintArgsUsesConfiguredModel(t *testing.T) {
	args := claudePrintArgs(true, `{"type":"object"}`, false, config.AgentRuntimeOptions{Model: "sonnet"})

	require.Contains(t, args, "--model")
	require.Equal(t, "sonnet", requireArgValue(t, args, "--model"))
}

func TestClaudePrintArgsUsesConfiguredMaxTurns(t *testing.T) {
	args := claudePrintArgs(false, `{"type":"object"}`, false, config.AgentRuntimeOptions{MaxTurns: 24})

	require.Equal(t, "24", requireArgValue(t, args, "--max-turns"))
}

func TestClaudePrintArgs_AllowsUserPluginsWhenConfigured(t *testing.T) {
	claudeHome := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeHome)
	writeClaudeJSON(t, filepath.Join(claudeHome, "plugins", "installed_plugins.json"), map[string]interface{}{
		"plugins": map[string]interface{}{
			"example-skills@anthropic-agent-skills": []map[string]string{{"scope": "user"}},
		},
	})

	args := claudePrintArgs(true, `{"type":"object"}`, false, config.AgentRuntimeOptions{})

	require.NotContains(t, args, "--setting-sources")
	require.NotContains(t, args, "--settings")
}

func TestClaudeArgsForLogRedactsInlineSchema(t *testing.T) {
	args := claudePrintArgs(true, `{"type":"object"}`, false, config.AgentRuntimeOptions{})

	logged := claudeArgsForLog(args)

	require.Equal(t, `{"type":"object"}`, requireArgValue(t, args, "--json-schema"))
	require.Equal(t, "<schema:17 bytes>", requireArgValue(t, logged, "--json-schema"))
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

func TestAnalyzeCurrentDeltaBatchConstrainsRuntimeSchemaToInputFocus(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	commandPath := filepath.Join(dir, "claude")
	command := `#!/bin/sh
while [ "$#" -gt 0 ]; do
	if [ "$1" = "--json-schema" ]; then
		printf '%s' "$2" > "$CAPTURE_SCHEMA_PATH"
		break
	fi
	shift
done
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"structured_output":{"knowledge_changes":[{"focus_action":"no_change","focus_id":"api-contract-design","focus_name":"API contract design","pattern_action":"no_change","pattern_id":"","proposal":null,"anchors":[],"reason":"No reusable change."}],"profile_refresh_recommended":{"needed":false,"reason":""}}}'
`
	require.NoError(t, os.WriteFile(commandPath, []byte(command), 0o755))
	t.Setenv("CAPTURE_SCHEMA_PATH", schemaPath)

	ag := New(commandPath, 5*time.Second, promptloader.New("claude", "en", ""), false, config.DefaultRetryConfig(), config.AgentRuntimeOptions{})
	result, err := ag.AnalyzeCurrentDeltaBatch(context.Background(), &agent.AnalyzeCurrentDeltaBatchRequest{
		Focuses: []agent.AnalyzeCurrentDeltaFocus{{
			EvidenceFocus: domain.EvidenceFocus{ID: "api-contract-design", Name: "API contract design"},
		}},
	})

	require.NoError(t, err)
	require.Len(t, result.Changes, 1)
	data, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	changes := schema["properties"].(map[string]any)["knowledge_changes"].(map[string]any)
	require.Equal(t, float64(1), changes["minItems"])
	require.Equal(t, float64(1), changes["maxItems"])
	items := changes["items"].(map[string]any)
	focusID := items["properties"].(map[string]any)["focus_id"].(map[string]any)
	require.Equal(t, []any{"api-contract-design"}, focusID["enum"])
}

func TestAnalyzeCurrentCodebaseBatchConstrainsRuntimeSchemaToInputFocus(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	commandPath := filepath.Join(dir, "claude")
	command := `#!/bin/sh
while [ "$#" -gt 0 ]; do
	if [ "$1" = "--json-schema" ]; then
		printf '%s' "$2" > "$CAPTURE_SCHEMA_PATH"
		break
	fi
	shift
done
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"structured_output":{"focuses":[{"focus_id":"api-contract-design","focus_name":"API contract design","patterns":[],"profile_refresh_recommended":{"needed":false,"reason":""}}]}}'
`
	require.NoError(t, os.WriteFile(commandPath, []byte(command), 0o755))
	t.Setenv("CAPTURE_SCHEMA_PATH", schemaPath)

	ag := New(commandPath, 5*time.Second, promptloader.New("claude", "en", ""), false, config.DefaultRetryConfig(), config.AgentRuntimeOptions{})
	result, err := ag.AnalyzeCurrentCodebaseBatch(context.Background(), &agent.AnalyzeCurrentCodebaseBatchRequest{
		Focuses: []agent.AnalyzeCurrentEvidenceFocus{{
			EvidenceFocus: domain.EvidenceFocus{ID: "api-contract-design", Name: "API contract design"},
		}},
	})

	require.NoError(t, err)
	require.Len(t, result.Focuses, 1)
	data, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	focuses := schema["properties"].(map[string]any)["focuses"].(map[string]any)
	require.Equal(t, float64(1), focuses["minItems"])
	require.Equal(t, float64(1), focuses["maxItems"])
	items := focuses["items"].(map[string]any)
	focusID := items["properties"].(map[string]any)["focus_id"].(map[string]any)
	require.Equal(t, []any{"api-contract-design"}, focusID["enum"])
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
