package commandstate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRepositoryRejectsUnsupportedSchemaOnLoad(t *testing.T) {
	repo := NewRepository(t.TempDir(), "learn-current")
	data, err := json.Marshal(&State{SchemaVersion: schemaVersion + 1})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(repo.Path()), 0o755))
	require.NoError(t, os.WriteFile(repo.Path(), data, 0o600))

	_, err = repo.Load(context.Background())

	require.ErrorIs(t, err, ErrUnsupportedSchemaVersion)
}

func TestRepositoryRejectsUnsupportedSchemaOnSave(t *testing.T) {
	repo := NewRepository(t.TempDir(), "learn-current")

	err := repo.Save(context.Background(), &State{SchemaVersion: schemaVersion + 1})

	require.ErrorIs(t, err, ErrUnsupportedSchemaVersion)
	require.NoFileExists(t, repo.Path())
}

func TestRepositoryRejectsNilState(t *testing.T) {
	repo := NewRepository(t.TempDir(), "learn-current")

	err := repo.Save(context.Background(), nil)

	require.EqualError(t, err, "command state is nil")
}

func TestRepositoryPersistsStableKnowledgeCommitCheckpoint(t *testing.T) {
	repo := NewRepository(t.TempDir(), "learn-current")
	state := NewState("learn-current", "demo", "go", "", []domain.FileAnalysisRecord{{Path: "main.go", Hash: "hash"}}, nil, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}})
	state.MarkPatternsCommitted(PatternCommitSummary{Found: 3, Saved: 2, Retired: 1})
	state.MarkSourceBaselineCommitted()
	state.MarkProjectionsCommitted()

	require.NoError(t, repo.Save(context.Background(), state))
	loaded, err := repo.Load(context.Background())

	require.NoError(t, err)
	checkpoint := loaded.KnowledgeCommitCheckpoint()
	require.NotNil(t, checkpoint)
	require.NotEmpty(t, checkpoint.ID)
	require.True(t, checkpoint.PatternsCommitted)
	require.Equal(t, PatternCommitSummary{Found: 3, Saved: 2, Retired: 1}, loaded.CommittedPatternSummary())
	require.True(t, checkpoint.SourceBaselineCommitted)
	require.True(t, checkpoint.ProjectionsCommitted)
	firstID := checkpoint.ID
	require.NoError(t, repo.Save(context.Background(), loaded))
	reloaded, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, firstID, reloaded.KnowledgeCommitCheckpoint().ID)
}
