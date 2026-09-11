package runtimeclean

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeLifecycle(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	runtimePath := filepath.Join(seedPath, "runtime")
	require.Equal(t, runtimePath, Path(seedPath))

	exists, err := Exists(seedPath)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, os.MkdirAll(runtimePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(runtimePath, "run.log"), []byte("log"), 0o644))
	exists, err = Exists(seedPath)
	require.NoError(t, err)
	require.True(t, exists)

	require.NoError(t, Clear(seedPath))
	require.NoDirExists(t, runtimePath)
	require.NoError(t, Clear(seedPath))
}

func TestExistsReturnsFalseForRuntimeFile(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	require.NoError(t, os.WriteFile(Path(seedPath), []byte("file"), 0o644))

	exists, err := Exists(seedPath)
	require.NoError(t, err)
	require.False(t, exists)
}
