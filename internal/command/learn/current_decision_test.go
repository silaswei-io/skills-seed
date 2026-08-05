package learn

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/stretchr/testify/require"
)

func TestCurrentDecisionCheckpointPersistsDecision(t *testing.T) {
	repo := commandstate.NewRepository(t.TempDir(), "sync")
	state := commandstate.NewState("sync", "demo", "go", "", nil, nil, nil)
	decision := &patternnorm.Decision{Patterns: []patternnorm.DecisionPattern{{
		ID:        "candidate",
		Name:      "Candidate",
		Category:  "business",
		Rule:      "Reuse candidate.",
		SourceIDs: []string{"candidate"},
	}}}
	checkpoint := newCurrentDecisionCheckpoint(repo, state)

	require.NoError(t, checkpoint.Save(context.Background(), "candidate-hash", decision))

	loaded, found, err := checkpoint.Load(context.Background(), "candidate-hash")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, decision, loaded)

	persisted, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, "candidate-hash", persisted.Decision.CandidateHash)
	var persistedDecision patternnorm.Decision
	require.NoError(t, json.Unmarshal(persisted.Decision.Decision, &persistedDecision))
	require.Equal(t, *decision, persistedDecision)

	replayed, found, err := newCurrentDecisionCheckpoint(repo, persisted).Load(context.Background(), "candidate-hash")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, decision, replayed)
}

func TestCurrentDecisionCheckpointRejectsChangedCandidates(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	state := commandstate.NewState("sync", "demo", "go", "", nil, nil, nil)
	state.Decision = &commandstate.DecisionCheckpoint{
		CandidateHash: "old-hash",
		Decision:      json.RawMessage(`{"patterns":[],"dropped":[]}`),
	}
	checkpoint := newCurrentDecisionCheckpoint(commandstate.NewRepository(t.TempDir(), "sync"), state)

	_, _, err := checkpoint.Load(context.Background(), "new-hash")

	require.ErrorContains(t, err, i18n.Get("LearnCurrentDecisionCandidateChanged"))
}
