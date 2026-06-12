package state

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRepositoryTracksSkillsDirtyTargets(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(filepath.Join(t.TempDir(), ".skills-seed"))

	require.NoError(t, repo.MarkSkillsDirty(ctx, domain.SkillsDirtyTarget{Project: true, Workspace: true}))
	require.NoError(t, repo.MarkSkillsDirty(ctx, domain.SkillsDirtyTarget{Projects: []string{"hsmwebapi", "cluster-manage", "hsmwebapi"}}))

	state, err := repo.Get(ctx)
	require.NoError(t, err)
	require.True(t, state.SkillsDirty.Project)
	require.True(t, state.SkillsDirty.Workspace)
	require.Equal(t, []string{"cluster-manage", "hsmwebapi"}, state.SkillsDirty.Projects)

	require.NoError(t, repo.ClearSkillsDirty(ctx, domain.SkillsDirtyTarget{Projects: []string{"hsmwebapi"}}))
	state, err = repo.Get(ctx)
	require.NoError(t, err)
	require.True(t, state.SkillsDirty.Project)
	require.True(t, state.SkillsDirty.Workspace)
	require.Equal(t, []string{"cluster-manage"}, state.SkillsDirty.Projects)

	require.NoError(t, repo.ClearSkillsDirty(ctx, domain.SkillsDirtyTarget{Project: true, Workspace: true, Projects: []string{"cluster-manage"}}))
	state, err = repo.Get(ctx)
	require.NoError(t, err)
	require.False(t, state.SkillsDirty.Project)
	require.False(t, state.SkillsDirty.Workspace)
	require.Empty(t, state.SkillsDirty.Projects)
}
