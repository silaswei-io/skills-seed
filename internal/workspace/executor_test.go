package workspace

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
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
