package snapshotflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	snapshotstore "github.com/silaswei-io/skills-seed/internal/infra/storage/snapshot"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/stretchr/testify/require"
)

func TestBuildScopedClassifiesAddedModifiedAndDeletedFiles(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "src", "added.go"), []byte("package src\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "src", "modified.go"), []byte("package src\n// new\n"), 0o644))
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{
		"src/modified.go": "package src\n// old\n",
		"src/deleted.go":  "package src\n",
		"docs/keep.md":    "keep\n",
	}))

	result, err := BuildScoped(ctx, projectRoot, []domain.FileInfo{
		{Path: "src/added.go", Content: "must not be trusted"},
		{Path: "src/modified.go", Content: "must not be trusted"},
	}, []string{"src"})

	require.NoError(t, err)
	require.Equal(t, []domain.FileInfo{{Path: "src/added.go"}}, result.AddedFiles)
	require.Len(t, result.DiffFiles, 2)
	require.ElementsMatch(t, []string{"src/deleted.go", "src/modified.go"}, []string{
		result.DiffFiles[0].Path,
		result.DiffFiles[1].Path,
	})
	for _, file := range result.DiffFiles {
		require.FileExists(t, file.DiffPath)
	}
	require.Equal(t, "package src\n", result.CurrentFiles["src/added.go"])
}

func TestBuildScopedFiltersFilesUsedForDiff(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "visible.go"), []byte("package visible\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "ignored.go"), []byte("package ignored\n"), 0o644))

	result, err := BuildScopedWithOptions(ctx, projectRoot, []domain.FileInfo{
		{Path: "visible.go"},
		{Path: "ignored.go"},
	}, nil, Options{DiffAllowed: func(path string) bool { return path == "visible.go" }})

	require.NoError(t, err)
	require.Equal(t, []domain.FileInfo{{Path: "visible.go"}}, result.AddedFiles)
	require.Len(t, result.CurrentFiles, 2)
}

func TestBuildScopedRejectsMissingAndEscapingFiles(t *testing.T) {
	projectRoot := t.TempDir()

	_, err := Build(context.Background(), projectRoot, []domain.FileInfo{{Path: "missing.go"}})
	require.ErrorContains(t, err, "read current file")

	_, err = Build(context.Background(), projectRoot, []domain.FileInfo{{Path: "../outside.go"}})
	require.ErrorContains(t, err, "resolve current file")
}

func TestResultCommitScopedPreservesSnapshotsOutsideScope(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{
		"src/old.go":   "old\n",
		"docs/keep.md": "keep\n",
	}))
	result := &Result{
		Repository: repo,
		CurrentFiles: map[string]string{
			"src/new.go":   "new\n",
			"docs/drop.md": "must not enter scoped snapshot\n",
		},
	}

	require.NoError(t, result.CommitScoped([]string{"src"}))

	got, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"src/new.go":   "new\n",
		"docs/keep.md": "keep\n",
	}, got)
}

func TestResultCommitScopedHandlesNilAndFullReplacement(t *testing.T) {
	var nilResult *Result
	require.NoError(t, nilResult.CommitScoped(nil))
	require.NoError(t, (&Result{}).CommitScoped(nil))

	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{"old.go": "old\n"}))
	result := &Result{Repository: repo, CurrentFiles: map[string]string{"new.go": "new\n"}}

	require.NoError(t, result.CommitScoped(nil))
	got, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"new.go": "new\n"}, got)
}

func TestPathInScopeNormalizesPaths(t *testing.T) {
	require.True(t, pathInScope("/src/domain/file.go", []string{"/src/domain/"}))
	require.True(t, pathInScope("src/domain", []string{"src/domain"}))
	require.False(t, pathInScope("src/other.go", []string{"", "src/domain"}))
}

func TestSeedPathForUsesProjectDefault(t *testing.T) {
	require.Equal(t, filepath.Join("project", ".skills-seed"), seedPathFor(context.Background(), "project"))
}
