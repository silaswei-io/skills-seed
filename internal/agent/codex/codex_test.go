package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/stretchr/testify/require"
)

func TestCodexExecArgs_UseCurrentWorkDirMode(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())

	args := codexExecArgs(false, "/tmp/output-schema.json", config.AgentRuntimeOptions{})

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

func TestCodexExecArgsUsesConfiguredModel(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())

	args := codexExecArgs(false, "/tmp/output-schema.json", config.AgentRuntimeOptions{Model: "gpt-5-mini"})

	require.Contains(t, args, "--model")
	require.Equal(t, "gpt-5-mini", requireArgValue(t, args, "--model"))
}

func TestCodexExecArgsForConversationResumesPersistedSession(t *testing.T) {
	conversation := agent.Conversation{Provider: "provider", ID: "thread-1"}
	args := codexExecArgsForConversation(false, "/tmp/output-schema.json", config.AgentRuntimeOptions{}, conversation)

	require.Contains(t, args, "resume")
	require.Contains(t, args, conversation.ID)
	require.NotContains(t, args, "--ephemeral")
}

func TestCodexConversationReadsThreadStartedEvent(t *testing.T) {
	conversation := codexConversation("{\"type\":\"thread.started\",\"thread_id\":\"thread-1\"}", "provider", agent.Conversation{Provider: "provider"})

	require.Equal(t, agent.Conversation{Provider: "provider", ID: "thread-1"}, conversation)
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

	args := codexExecArgs(false, "/tmp/output-schema.json", config.AgentRuntimeOptions{})

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

	args := codexExecArgs(true, "/tmp/output-schema.json", config.AgentRuntimeOptions{})

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

func TestReviewKnowledgeRetriesInvalidStructuredResult(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	commandPath := filepath.Join(projectRoot, "codex")
	attemptPath := filepath.Join(projectRoot, "attempts")
	command := `#!/bin/sh
attempt=0
if [ -f "$CODEX_REVIEW_ATTEMPT_PATH" ]; then
	attempt=$(sed -n '1p' "$CODEX_REVIEW_ATTEMPT_PATH")
fi
attempt=$((attempt + 1))
printf '%s' "$attempt" > "$CODEX_REVIEW_ATTEMPT_PATH"
printf '%s\n' '{"type":"thread.started","thread_id":"thread-test"}'
if [ "$attempt" -eq 1 ]; then
	printf '%s\n' '{"type":"item.completed","item":{"id":"result","type":"agent_message","text":"{\"decisions\":[{\"candidate_id\":\"app-boot-auth-chain-init-guard\",\"reason_code\":\"accepted\",\"reason\":\"Evidence supports the candidate.\",\"business_method\":null,\"revision\":null}]}"}}'
	exit 0
fi
printf '%s\n' '{"type":"item.completed","item":{"id":"result","type":"agent_message","text":"{\"decisions\":[{\"candidate_id\":\"app-boot-auth-chain-init-guard\",\"verdict\":\"accept\",\"reason_code\":\"accepted\",\"reason\":\"Evidence supports the candidate.\",\"business_method\":null,\"revision\":null}]}"}}'
`
	require.NoError(t, os.WriteFile(commandPath, []byte(command), 0o755))
	t.Setenv("CODEX_REVIEW_ATTEMPT_PATH", attemptPath)
	t.Setenv("CODEX_HOME", t.TempDir())

	ag := New(commandPath, 5*time.Second, promptloader.New("codex", "en", ""), false, config.RetryConfig{MaxRetries: 1, InitialInterval: -1, MaxInterval: -1}, config.AgentRuntimeOptions{})
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)
	result, err := ag.ReviewKnowledge(ctx, &agent.ReviewKnowledgeRequest{
		ProjectName:  "kmc-admin-web",
		RootPath:     ".",
		Language:     "TypeScript",
		RuntimeLabel: "batch-001",
		EvidenceFocus: domain.EvidenceFocus{
			ID:   "login-auth-session-access-control",
			Name: "Login authentication and session access control",
		},
		Candidates: []domain.Pattern{{ID: "app-boot-auth-chain-init-guard"}},
	})

	require.NoError(t, err)
	attemptData, err := os.ReadFile(attemptPath)
	require.NoError(t, err)
	require.Equal(t, "2", string(attemptData))
	require.Len(t, result.Decisions, 1)
	require.Equal(t, "accept", result.Decisions[0].Verdict)
	paths, err := filepath.Glob(filepath.Join(seedPath, "runtime", "agent-outputs", "*-codex-learning-knowledge-review-batch-001*.manifest.json"))
	require.NoError(t, err)
	require.Len(t, paths, 2)
}

func TestAnalyzeCurrentDeltaBatchConstrainsRuntimeSchemaToInputFocus(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	commandPath := filepath.Join(dir, "codex")
	command := `#!/bin/sh
while [ "$#" -gt 0 ]; do
	if [ "$1" = "--output-schema" ]; then
		cp "$2" "$CAPTURE_SCHEMA_PATH"
		break
	fi
	shift
done
printf '%s\n' '{"type":"thread.started","thread_id":"thread-test"}'
printf '%s\n' '{"type":"item.completed","item":{"id":"result","type":"agent_message","text":"{\"knowledge_changes\":[{\"focus_action\":\"no_change\",\"focus_id\":\"api-contract-design\",\"focus_name\":\"API contract design\",\"pattern_action\":\"no_change\",\"pattern_id\":\"\",\"proposal\":null,\"anchors\":[],\"reason\":\"No reusable change.\"}],\"profile_refresh_recommended\":{\"needed\":false,\"reason\":\"\"}}"}}'
`
	require.NoError(t, os.WriteFile(commandPath, []byte(command), 0o755))
	t.Setenv("CAPTURE_SCHEMA_PATH", schemaPath)
	t.Setenv("CODEX_HOME", t.TempDir())

	ag := New(commandPath, 5*time.Second, promptloader.New("codex", "en", ""), false, config.DefaultRetryConfig(), config.AgentRuntimeOptions{})
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
	commandPath := filepath.Join(dir, "codex")
	command := `#!/bin/sh
while [ "$#" -gt 0 ]; do
	if [ "$1" = "--output-schema" ]; then
		cp "$2" "$CAPTURE_SCHEMA_PATH"
		break
	fi
	shift
done
printf '%s\n' '{"type":"thread.started","thread_id":"thread-test"}'
printf '%s\n' '{"type":"item.completed","item":{"id":"result","type":"agent_message","text":"{\"focuses\":[{\"focus_id\":\"api-contract-design\",\"focus_name\":\"API contract design\",\"patterns\":[],\"profile_refresh_recommended\":{\"needed\":false,\"reason\":\"\"}}]}"}}'
`
	require.NoError(t, os.WriteFile(commandPath, []byte(command), 0o755))
	t.Setenv("CAPTURE_SCHEMA_PATH", schemaPath)
	t.Setenv("CODEX_HOME", t.TempDir())

	ag := New(commandPath, 5*time.Second, promptloader.New("codex", "en", ""), false, config.DefaultRetryConfig(), config.AgentRuntimeOptions{})
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
