package autofix

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixWithPatchCreatesUnifiedDiff(t *testing.T) {
	require.NoError(t, i18n.Init(i18n.LocaleEnglish))
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644))

	gitRepo := &mocks.MockGitRepository{
		ProjectRootFn: func(ctx context.Context) (string, error) {
			return root, nil
		},
	}
	svc := NewAutofixService("patch", filepath.Join(root, ".skills-seed"), gitRepo)

	result, err := svc.FixIssues(context.Background(), []domain.Issue{{File: "main.go"}}, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
	})
	require.NoError(t, err)
	require.True(t, result.Success)

	content, err := os.ReadFile(result.OutputPath)
	require.NoError(t, err)
	patch := string(content)
	assert.Contains(t, patch, "# Auto-generated patch by skills-seed")
	assert.Contains(t, patch, "# Issues fixed: 1")
	assert.Contains(t, patch, "diff --git a/main.go b/main.go")
	assert.Contains(t, patch, "@@ -1,1 +1,3 @@")
	assert.Contains(t, patch, "-package main")
	assert.Contains(t, patch, "+func main() {}")
	assert.NotContains(t, result.Message, "{{.Path}}")
}

func TestFixWithBackupUsesProjectRootAndPreservesPath(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "app.go"), []byte("package internal\n"), 0644))

	gitRepo := &mocks.MockGitRepository{
		ProjectRootFn: func(ctx context.Context) (string, error) {
			return root, nil
		},
	}
	svc := NewAutofixService("backup", filepath.Join(root, ".skills-seed"), gitRepo)

	result, err := svc.FixIssues(context.Background(), []domain.Issue{{File: "internal/app.go"}}, map[string]string{
		"internal/app.go": "package internal\n\nfunc App() {}\n",
	})
	require.NoError(t, err)
	require.True(t, result.Success)

	updated, err := os.ReadFile(filepath.Join(root, "internal", "app.go"))
	require.NoError(t, err)
	assert.Contains(t, string(updated), "func App() {}")

	backup, err := os.ReadFile(filepath.Join(result.OutputPath, "internal", "app.go"))
	require.NoError(t, err)
	assert.Equal(t, "package internal\n", string(backup))
}

func TestFixAllowsAbsolutePathInsideProjectRoot(t *testing.T) {
	root := t.TempDir()
	appPath := filepath.Join(root, "app.go")
	require.NoError(t, os.WriteFile(appPath, []byte("package main\n"), 0644))

	gitRepo := &mocks.MockGitRepository{
		ProjectRootFn: func(ctx context.Context) (string, error) {
			return root, nil
		},
	}
	svc := NewAutofixService("backup", filepath.Join(root, ".skills-seed"), gitRepo)

	result, err := svc.FixIssues(context.Background(), []domain.Issue{{File: appPath}}, map[string]string{
		appPath: "package main\n\nfunc main() {}\n",
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	updated, readErr := os.ReadFile(appPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(updated), "func main() {}")
}

func TestFixRejectsPathsOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	require.NoError(t, os.MkdirAll(root, 0755))
	outsidePath := filepath.Join(parent, "outside.go")
	require.NoError(t, os.WriteFile(outsidePath, []byte("package outside\n"), 0644))

	gitRepo := &mocks.MockGitRepository{
		ProjectRootFn: func(ctx context.Context) (string, error) {
			return root, nil
		},
	}
	svc := NewAutofixService("backup", filepath.Join(root, ".skills-seed"), gitRepo)

	_, err := svc.FixIssues(context.Background(), []domain.Issue{{File: "../outside.go"}}, map[string]string{
		"../outside.go": "package outside\n\nfunc Escaped() {}\n",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside project root")

	content, readErr := os.ReadFile(outsidePath)
	require.NoError(t, readErr)
	assert.Equal(t, "package outside\n", string(content))
}
