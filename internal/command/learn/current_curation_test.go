package learn

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/stretchr/testify/require"
)

func TestCurrentCurationCheckpointPersistsImportedDecision(t *testing.T) {
	repo := commandstate.NewRepository(t.TempDir(), "sync")
	state := commandstate.NewState("sync", "demo", "go", "", nil, nil)
	decision := &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{{
		ID:        "candidate",
		Name:      "Candidate",
		Category:  "business",
		Rule:      "Reuse candidate.",
		SourceIDs: []string{"candidate"},
	}}}
	checkpoint := newCurrentCurationCheckpoint(repo, state, decision)

	loaded, found, err := checkpoint.Load(context.Background(), "candidate-hash")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, decision, loaded)

	persisted, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, "candidate-hash", persisted.Curation.CandidateHash)
	var persistedDecision agent.CuratePatternsResult
	require.NoError(t, json.Unmarshal(persisted.Curation.Decision, &persistedDecision))
	require.Equal(t, *decision, persistedDecision)

	replayed, found, err := newCurrentCurationCheckpoint(repo, persisted, nil).Load(context.Background(), "candidate-hash")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, decision, replayed)
}

func TestCurrentCurationCheckpointRejectsChangedCandidates(t *testing.T) {
	state := commandstate.NewState("sync", "demo", "go", "", nil, nil)
	state.Curation = &commandstate.CurationCheckpoint{
		CandidateHash: "old-hash",
		Decision:      json.RawMessage(`{"patterns":[],"dropped":[]}`),
	}
	checkpoint := newCurrentCurationCheckpoint(commandstate.NewRepository(t.TempDir(), "sync"), state, nil)

	_, _, err := checkpoint.Load(context.Background(), "new-hash")

	require.ErrorContains(t, err, "curation candidate hash changed")
}
