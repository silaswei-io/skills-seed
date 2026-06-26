package filetree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestWalkReturnsRelativeFilesAndFiltersExcludedPaths(t *testing.T) {
	root := t.TempDir()
	writeWalkFile(t, root, "main.go", "package main\n")
	writeWalkFile(t, root, "docs/readme.md", "# docs\n")
	writeWalkFile(t, root, "node_modules/pkg/index.js", "console.log(1)\n")

	files, err := Walk(root, []string{"node_modules", "*.md"})

	require.NoError(t, err)
	require.Equal(t, []domain.FileInfo{
		domain.NewFileInfo("main.go", ""),
	}, files)
}

func TestWalkSkipsDotPrefixedExcludedDirectories(t *testing.T) {
	root := t.TempDir()
	writeWalkFile(t, root, ".skills-seed/runtime/input.txt", "runtime\n")
	writeWalkFile(t, root, "internal/service.go", "package internal\n")

	files, err := Walk(root, []string{".*"})

	require.NoError(t, err)
	require.Equal(t, []domain.FileInfo{
		domain.NewFileInfo("internal/service.go", ""),
	}, files)
}

func writeWalkFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
