package fileanalysis

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
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

func TestConfiguredSelectionPolicyAppliesGitIgnoreByDefault(t *testing.T) {
	root := t.TempDir()
	initSelectionGitRepo(t, root)
	writeSelectionFile(t, root, ".gitignore", "ignored-dir/\n*.tmp\n")
	writeSelectionFile(t, root, "main.go", "package main\n")
	writeSelectionFile(t, root, "ignored-dir/generated.go", "package ignored\n")
	writeSelectionFile(t, root, "debug.tmp", "temporary\n")

	repo, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "zh-CN")
	require.NoError(t, err)

	selection, err := SelectFiles(SelectOptions{
		Root:   root,
		Policy: NewConfiguredSelectionPolicy(repo, root),
	})

	require.NoError(t, err)
	require.Equal(t, []string{"main.go"}, selection.Paths())
	require.Equal(t, 2, selection.SkippedCount(SkipReasonExcluded))
}

func TestConfiguredSelectionPolicyCanDisableGitIgnore(t *testing.T) {
	root := t.TempDir()
	initSelectionGitRepo(t, root)
	writeSelectionFile(t, root, ".gitignore", "ignored-dir/\n")
	writeSelectionFile(t, root, "main.go", "package main\n")
	writeSelectionFile(t, root, "ignored-dir/generated.go", "package ignored\n")

	repo, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := repo.Get()
	cfg.Exclude.GitIgnore = false
	require.NoError(t, repo.Update(cfg))

	selection, err := SelectFiles(SelectOptions{
		Root:   root,
		Policy: NewConfiguredSelectionPolicy(repo, root),
	})

	require.NoError(t, err)
	require.ElementsMatch(t, []string{"ignored-dir/generated.go", "main.go"}, selection.Paths())
}

func TestSelectFilesSkipsSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.go")
	require.NoError(t, os.WriteFile(outside, []byte("package outside\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "linked.go")))

	selection, err := SelectFiles(SelectOptions{Root: root, Policy: NewSelectionPolicy(nil)})

	require.NoError(t, err)
	require.Empty(t, selection.Paths())
	require.Equal(t, 1, selection.SkippedCount(SkipReasonUnreadable))
}

func initSelectionGitRepo(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
	require.NoError(t, cmd.Run())
}

func writeSelectionFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
