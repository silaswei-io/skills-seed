package learning

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/stretchr/testify/require"
)

type supplementRuntime struct {
	fakeLearningRuntime
	seedPath string
}

func (r supplementRuntime) PromptRenderer() prompts.Renderer {
	return prompts.New("codex", "en-US", r.seedPath)
}

func TestSupplementUsesSelfContainedPromptAndIndependentSession(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "background.md"), []byte("A reservation holds capacity until cancellation."), 0o644))
	calls := 0
	rt := supplementRuntime{seedPath: seedPath, fakeLearningRuntime: fakeLearningRuntime{name: "fake", invoke: func(ctx context.Context, in agent.StructuredInvoke) (agent.StructuredResult, error) {
		calls++
		require.True(t, in.Runtime.PromptOnly)
		require.False(t, in.Conversation.Valid())
		require.Contains(t, in.Prompt, "func ExistingCapability()")
		require.Contains(t, in.Prompt, "existing-id")
		require.Contains(t, in.Prompt, "A reservation holds capacity until cancellation.")
		require.Contains(t, in.Prompt, "Check cancellation ownership.")
		require.LessOrEqual(t, len(in.Prompt), agent.SupplementInputLimit)
		return agent.StructuredResult{Output: `{"decisions":[]}`}, nil
	}}}
	ctx := agent.WithLearningSupplement(context.Background(), &agent.LearningSupplement{Evidence: `{"questions":[],"sources":{"existing.go":"func ExistingCapability() {}"},"calls":[]}`})
	_, err := ReviewKnowledge(ctx, rt, &agent.ReviewKnowledgeRequest{EvidenceFocus: domain.EvidenceFocus{ID: "existing-id"}, UserContext: "Check cancellation ownership.", Conversation: agent.Conversation{Provider: "fake", ID: "old-analysis"}})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	oversizedEvidence, err := json.Marshal(map[string]interface{}{"sources": map[string]string{"large.go": strings.Repeat("x", agent.SupplementInputLimit+1)}})
	require.NoError(t, err)
	oversized := agent.WithLearningSupplement(context.Background(), &agent.LearningSupplement{Evidence: string(oversizedEvidence)})
	_, err = ReviewKnowledge(oversized, rt, &agent.ReviewKnowledgeRequest{})
	require.ErrorIs(t, err, agent.ErrSupplementUnresolved)
	require.Equal(t, 1, calls, "oversized input must never reach AI")
}

func TestSupplementEvidenceKeepsAllSourcesWithoutJSONEscapeOverhead(t *testing.T) {
	source := strings.Repeat("func ExistingCapability() { return \"ready\" }\n", 3000)
	raw, err := json.Marshal(map[string]interface{}{
		"questions": []string{"check existing capability"},
		"sources":   map[string]string{"existing.go": source},
		"calls":     []string{"entry.go::Run -> existing.go::ExistingCapability"},
		"boundary":  "Do not infer omitted callers.",
	})
	require.NoError(t, err)
	formatted, err := formatSupplementEvidence(string(raw))
	require.NoError(t, err)
	require.Less(t, len(formatted), len(raw))
	require.Contains(t, formatted, source)
	require.Contains(t, formatted, "check existing capability")
	require.Contains(t, formatted, "entry.go::Run")
	require.Contains(t, formatted, "Do not infer omitted callers.")
}
