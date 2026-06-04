package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/stretchr/testify/require"
)

func TestWorkDirForContextUsesSeedPathProjectRoot(t *testing.T) {
	projectRoot := t.TempDir()
	ctx := runtimecontext.WithSeedPath(context.Background(), filepath.Join(projectRoot, ".skills-seed"))

	workDir, err := WorkDirForContext(ctx)

	require.NoError(t, err)
	require.Equal(t, projectRoot, workDir)
}
