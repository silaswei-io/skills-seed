package fileanalysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectFilesUsesOnePolicyForExcludeDocumentsAndSourceFiles(t *testing.T) {
	root := t.TempDir()
	writeSelectionFile(t, root, "main.go", "package main\n")
	writeSelectionFile(t, root, "docs/examples/demo.go", "package examples\n")
	writeSelectionFile(t, root, "README.MD", "# readme\n")
	writeSelectionFile(t, root, "assets/logo.svg", "<svg></svg>\n")
	writeSelectionFile(t, root, "debug.log", "debug\n")

	selection, err := SelectFiles(SelectOptions{
		Root:   root,
		Policy: NewSelectionPolicy([]string{"*.log"}),
	})

	require.NoError(t, err)
	require.ElementsMatch(t, []string{"docs/examples/demo.go", "main.go"}, selection.Paths())
	require.Equal(t, 1, selection.SkippedCount(SkipReasonDocument))
	require.Equal(t, 1, selection.SkippedCount(SkipReasonNonSource))
	require.Equal(t, 1, selection.SkippedCount(SkipReasonExcluded))
}

func TestSelectFilesKeepsOnlyFocusedAnalyzableFiles(t *testing.T) {
	root := t.TempDir()
	writeSelectionFile(t, root, "cmd/server/main.go", "package main\n")
	writeSelectionFile(t, root, "internal/app.go", "package internal\n")
	writeSelectionFile(t, root, "cmd/server/README.MD", "# server\n")

	selection, err := SelectFiles(SelectOptions{
		Root:          root,
		Policy:        NewSelectionPolicy(nil),
		FocusAbsPaths: []string{filepath.Join(root, "cmd", "server")},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"cmd/server/main.go"}, selection.Paths())
	require.Equal(t, 1, selection.SkippedCount(SkipReasonDocument))
}

func writeSelectionFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
