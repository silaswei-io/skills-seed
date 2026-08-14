package learn

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/stretchr/testify/require"
)

func TestCurrentDecisionCheckpointPersistsMatchingDecision(t *testing.T) {
	state := commandstate.NewState("learn_current", "demo", "go", "", nil, nil, nil)
	checkpoint := newCurrentDecisionCheckpoint(commandstate.NewRepository(t.TempDir(), "learn_current"), state)
	decision := &patternnorm.Decision{Patterns: []patternnorm.DecisionPattern{{ID: "candidate", SourceIDs: []string{"candidate"}}}}

	require.NoError(t, checkpoint.Save(context.Background(), "candidate-hash", decision))
	loaded, found, err := checkpoint.Load(context.Background(), "candidate-hash")

	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, decision, loaded)
}

func TestCurrentDecisionCheckpointRejectsChangedCandidates(t *testing.T) {
	state := commandstate.NewState("learn_current", "demo", "go", "", nil, nil, nil)
	checkpoint := newCurrentDecisionCheckpoint(commandstate.NewRepository(t.TempDir(), "learn_current"), state)
	require.NoError(t, checkpoint.Save(context.Background(), "first", &patternnorm.Decision{}))

	_, found, err := checkpoint.Load(context.Background(), "second")

	require.False(t, found)
	require.Error(t, err)
}
