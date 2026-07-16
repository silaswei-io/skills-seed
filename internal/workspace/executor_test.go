package workspace

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestEffectiveParallelism(t *testing.T) {
	require.Equal(t, 1, EffectiveParallelism(domain.ModeProject, 0, 5))
	require.Equal(t, 3, EffectiveParallelism(domain.ModeWorkspace, 0, 3))
	require.Equal(t, defaultWorkspaceParallelismCap, EffectiveParallelism(domain.ModeWorkspace, 0, 20))
	require.Equal(t, 9, EffectiveParallelism(domain.ModeWorkspace, 9, 20))
}

func TestRunProjectTasksReturnsCanceledContextWithoutRunningTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := RunProjectTasks(ctx, []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend"},
	}, 1, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
		called = true
		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
	require.False(t, called)
}

func TestResolveProjectRootRejectsPathOutsideWorkspaceRoot(t *testing.T) {
	require.NoError(t, i18n.Init(i18n.LocaleEnglish))
	workspaceRoot := t.TempDir()

	projectRoot, err := ResolveProjectRoot(workspaceRoot, config.WorkspaceProjectConfig{ID: "backend", Path: "backend"})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(workspaceRoot, "backend"), projectRoot)

	_, err = ResolveProjectRoot(workspaceRoot, config.WorkspaceProjectConfig{ID: "escaped", Path: "../escaped"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside workspace root")
}
