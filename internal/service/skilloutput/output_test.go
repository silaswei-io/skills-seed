package skilloutput

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceReplacesExistingDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skill")
	require.NoError(t, os.MkdirAll(root, 0o755))
	oldPath := filepath.Join(root, "old.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("old\n"), 0o644))

	require.NoError(t, Replace(root, func(staging string) error {
		return os.WriteFile(filepath.Join(staging, "SKILL.md"), []byte("new\n"), 0o644)
	}))
	require.NoFileExists(t, oldPath)
	require.FileExists(t, filepath.Join(root, "SKILL.md"))
}

func TestRemoveDeletesConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("existing\n"), 0o644))

	require.NoError(t, Remove(root))
	require.NoDirExists(t, root)
	require.NoError(t, Remove(root))
}
