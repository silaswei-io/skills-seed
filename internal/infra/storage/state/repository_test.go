package state

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRepositoryTracksLearnedAndGeneratedState(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(filepath.Join(t.TempDir(), ".skills-seed"))

	require.NoError(t, repo.MarkLearned(ctx, domain.ModeProject))
	require.NoError(t, repo.MarkSkillsGenerated(ctx, domain.ModeProject))

	state, err := repo.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, state.Mode)
	require.True(t, state.ModeLocked)
	require.True(t, state.Learned)
	require.True(t, state.SkillsGenerated)
}
