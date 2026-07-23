package learn

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/stretchr/testify/require"
)

func TestNewLearnCurrentProjectRunPreservesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	run := newLearnCurrentProjectRun(ctx, &container.Container{SeedPath: t.TempDir()}, learnCurrentProjectOptions{})
	cancel()

	require.ErrorIs(t, run.ctx.Err(), context.Canceled)
}
