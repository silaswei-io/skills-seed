package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRepositoryReadsHistoryChangesAndStagedFiles(t *testing.T) {
	root := newGitRepository(t)
	repo := NewRepository(root)
	writeGitFile(t, root, "first.go", "package first\n")
	runRepositoryGit(t, root, "add", "first.go")
	runRepositoryGit(t, root, "commit", "-q", "-m", "first commit")
	writeGitFile(t, root, "second.go", "package second\n")
	runRepositoryGit(t, root, "add", "second.go")
	runRepositoryGit(t, root, "commit", "-q", "-m", "second | commit")

	commits, err := repo.GetCommits(context.Background(), 2, "")
	require.NoError(t, err)
	require.Len(t, commits, 2)
	require.Equal(t, "first commit", commits[0].Message)
	require.Equal(t, "second | commit", commits[1].Message)

	changed, err := repo.GetChangedFiles(context.Background(), commits[1].Hash)
	require.NoError(t, err)
	require.Equal(t, []string{"second.go"}, changed)

	writeGitFile(t, root, "first.go", "package first\n// modified\n")
	writeGitFile(t, root, "added.go", "package added\n")
	require.NoError(t, os.Remove(filepath.Join(root, "second.go")))
	runRepositoryGit(t, root, "add", "-A")
	staged, err := repo.GetStagedFiles(context.Background())
	require.NoError(t, err)
	require.ElementsMatch(t, []domain.FileInfo{
		{Path: "added.go", Language: "go", Status: domain.StatusAdded},
		{Path: "first.go", Language: "go", Status: domain.StatusModified},
		{Path: "second.go", Language: "go", Status: domain.StatusDeleted},
	}, staged)
}

func TestRepositoryBranchAndStashOperations(t *testing.T) {
	root := newGitRepository(t)
	repo := NewRepository(root)
	writeGitFile(t, root, "README.md", "initial\n")
	runRepositoryGit(t, root, "add", "README.md")
	runRepositoryGit(t, root, "commit", "-q", "-m", "initial")

	branch, err := repo.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	require.Equal(t, "main", branch)
	projectRoot, err := repo.GetProjectRoot(context.Background())
	require.NoError(t, err)
	require.Equal(t, root, projectRoot)

	require.NoError(t, repo.CreateBranch(context.Background(), "feature"))
	branch, err = repo.GetCurrentBranch(context.Background())
	require.NoError(t, err)
	require.Equal(t, "feature", branch)
	require.NoError(t, repo.Checkout(context.Background(), "main"))

	writeGitFile(t, root, "README.md", "changed\n")
	require.NoError(t, repo.Stash(context.Background(), "test stash"))
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)
	require.Equal(t, "initial\n", string(data))
}

func TestRepositoryReportsGitFailures(t *testing.T) {
	repo := NewRepository(filepath.Join(t.TempDir(), "missing"))
	ctx := context.Background()
	_, err := repo.GetCommits(ctx, 1, "")
	require.Error(t, err)
	_, err = repo.GetChangedFiles(ctx, "missing")
	require.Error(t, err)
	_, err = repo.GetStagedFiles(ctx)
	require.Error(t, err)
	_, err = repo.GetCurrentBranch(ctx)
	require.Error(t, err)
	require.Error(t, repo.Stash(ctx, "message"))
	require.Error(t, repo.CreateBranch(ctx, "branch"))
	require.Error(t, repo.Checkout(ctx, "branch"))
}

func TestRepositoryHelpers(t *testing.T) {
	repo := NewRepository("root")
	require.Equal(t, domain.StatusAdded, repo.parseStatus(" A "))
	require.Equal(t, domain.StatusDeleted, repo.parseStatus("D"))
	require.Equal(t, domain.StatusModified, repo.parseStatus("R100"))
	require.Equal(t, []string{"one", "two"}, nonEmptyLines([]byte(" one \n\n two\n")))
}

func newGitRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runRepositoryGit(t, root, "init", "-q", "-b", "main")
	runRepositoryGit(t, root, "config", "user.name", "Unit Test")
	runRepositoryGit(t, root, "config", "user.email", "unit@example.com")
	return root
}

func writeGitFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func runRepositoryGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}
