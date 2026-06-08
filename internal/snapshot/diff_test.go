package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompareClassifiesFilesAndWritesDiffs(t *testing.T) {
	runtimeDir := t.TempDir()
	changes, err := CompareScoped(map[string]string{
		"added.go":     "package added\n",
		"modified.go":  "package main\n\nfunc newName() {}\n",
		"unchanged.go": "package same\n",
	}, map[string]string{
		"modified.go":  "package main\n\nfunc oldName() {}\n",
		"unchanged.go": "package same\n",
		"deleted.go":   "package deleted\n",
	}, runtimeDir, nil)

	require.NoError(t, err)
	require.Len(t, changes, 4)
	require.Equal(t, FileChange{Path: "added.go", Status: ChangeAdded}, changes[0])
	require.Equal(t, "modified.go", changes[1].Path)
	require.Equal(t, ChangeModified, changes[1].Status)
	require.NotEmpty(t, changes[1].DiffPath)
	require.Equal(t, FileChange{Path: "unchanged.go", Status: ChangeUnchanged}, changes[2])
	require.Equal(t, "deleted.go", changes[3].Path)
	require.Equal(t, ChangeDeleted, changes[3].Status)
	require.NotEmpty(t, changes[3].DiffPath)

	diffContent, err := os.ReadFile(changes[1].DiffPath)
	require.NoError(t, err)
	require.Contains(t, string(diffContent), "--- modified.go")
	require.Contains(t, string(diffContent), "+++ modified.go")
	require.Contains(t, string(diffContent), "-func oldName() {}")
	require.Contains(t, string(diffContent), "+func newName() {}")
	require.True(t, filepath.IsAbs(changes[1].DiffPath))

	deletedDiffContent, err := os.ReadFile(changes[3].DiffPath)
	require.NoError(t, err)
	require.Contains(t, string(deletedDiffContent), "-package deleted")
}

func TestWriteUnifiedDiffSanitizesOutputPath(t *testing.T) {
	runtimeDir := t.TempDir()

	diffPath, err := WriteUnifiedDiff(runtimeDir, "internal/service/app.go", "old\n", "new\n")

	require.NoError(t, err)
	require.Equal(t, filepath.Join(runtimeDir, "diffs", "internal", "service", "app.go.diff"), diffPath)
	require.FileExists(t, diffPath)
}
