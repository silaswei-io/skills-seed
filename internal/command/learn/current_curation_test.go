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
	var persistedDecision agent.CuratePatternsResult
	require.NoError(t, json.Unmarshal(persisted.Curation.Decisions["candidate-hash"], &persistedDecision))
	require.Equal(t, *decision, persistedDecision)

	replayed, found, err := newCurrentCurationCheckpoint(repo, persisted, nil).Load(context.Background(), "candidate-hash")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, decision, replayed)
}

func TestCurrentCurationCheckpointKeepsIndependentCandidateBatches(t *testing.T) {
	state := commandstate.NewState("sync", "demo", "go", "", nil, nil)
	state.Curation = &commandstate.CurationCheckpoint{
		Decisions: map[string]json.RawMessage{"old-hash": json.RawMessage(`{"patterns":[],"dropped":[]}`)},
	}
	checkpoint := newCurrentCurationCheckpoint(commandstate.NewRepository(t.TempDir(), "sync"), state, nil)

	_, found, err := checkpoint.Load(context.Background(), "new-hash")

	require.NoError(t, err)
	require.False(t, found)
}

func TestCurrentCurationCheckpointPersistsMultipleBatches(t *testing.T) {
	repo := commandstate.NewRepository(t.TempDir(), "sync")
	state := commandstate.NewState("sync", "demo", "go", "", nil, nil)
	checkpoint := newCurrentCurationCheckpoint(repo, state, nil)
	first := &agent.CuratePatternsResult{Dropped: []agent.CuratedDrop{{ID: "a", Reason: "weak"}}}
	second := &agent.CuratePatternsResult{Dropped: []agent.CuratedDrop{{ID: "b", Reason: "weak"}}}

	require.NoError(t, checkpoint.Save(context.Background(), "first", first))
	require.NoError(t, checkpoint.Save(context.Background(), "second", second))
	persisted, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, persisted.Curation.Decisions, 2)

	replayed := newCurrentCurationCheckpoint(repo, persisted, nil)
	loadedFirst, found, err := replayed.Load(context.Background(), "first")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first, loadedFirst)
	loadedSecond, found, err := replayed.Load(context.Background(), "second")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, second, loadedSecond)
}
