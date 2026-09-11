package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestCommandMetadataAndNilContainer(t *testing.T) {
	cmd := Cmd(nil)
	require.Equal(t, "workspace", cmd.Use)
	require.Len(t, cmd.Commands(), 1)
	cmd.SetArgs([]string{"add", "."})
	require.Error(t, cmd.Execute())
}

func TestSelectWorkspaceProjects(t *testing.T) {
	detected := []config.WorkspaceProjectConfig{
		{ID: "api", Path: "services/api", Language: "go"},
		{ID: "web", Path: "apps/web", Language: "typescript"},
	}

	selected, err := selectWorkspaceProjects(detected, []string{".", "ignored"})
	require.Error(t, err)
	require.Nil(t, selected)

	selected, err = selectWorkspaceProjects(detected, []string{" . "})
	require.NoError(t, err)
	require.Equal(t, detected, selected)

	selected, err = selectWorkspaceProjects(detected, []string{"api", "./apps\\web", "api"})
	require.NoError(t, err)
	require.Equal(t, detected, selected)

	_, err = selectWorkspaceProjects(detected, nil)
	require.Error(t, err)
	_, err = selectWorkspaceProjects(detected, []string{"missing"})
	require.Error(t, err)
}

func TestProjectTargetHelpers(t *testing.T) {
	require.True(t, selectsAllDetectedProjects([]string{" ./ "}))
	require.False(t, selectsAllDetectedProjects([]string{".", "api"}))
	require.Empty(t, normalizeProjectTarget(" \t"))
	require.Equal(t, "services/api", normalizeProjectTarget(" ./services\\api/ "))
	require.Equal(t, "/", normalizeProjectTarget("/"))
}

func TestMergeWorkspaceProjectsReplacesByPathAndPreservesOrder(t *testing.T) {
	existing := []config.WorkspaceProjectConfig{
		{ID: "old-api", Path: "api"},
		{ID: "web", Path: "web"},
	}
	additions := []config.WorkspaceProjectConfig{
		{ID: "api", Path: "api", Language: "go"},
		{ID: "worker", Path: "worker"},
	}

	require.Equal(t, []config.WorkspaceProjectConfig{
		{ID: "api", Path: "api", Language: "go"},
		{ID: "web", Path: "web"},
		{ID: "worker", Path: "worker"},
	}, mergeWorkspaceProjects(existing, additions))
}

func TestRunAddWorkspaceProjectsValidatesPreconditions(t *testing.T) {
	root := t.TempDir()
	repo := newWorkspaceConfigRepository(t, root, domain.ModeProject)
	require.Error(t, runAddWorkspaceProjects(context.Background(), root, repo, []string{"api"}))
	require.Error(t, runAddWorkspaceProjects(context.Background(), root, repo, []string{"api"}, Dependencies{
		EnsureChildInitialized: func(string, config.WorkspaceProjectConfig, *config.Repository, string) error { return nil },
	}))

	repo = newWorkspaceConfigRepository(t, root, domain.ModeWorkspace)
	require.Error(t, runAddWorkspaceProjects(context.Background(), root, repo, []string{"api"}, Dependencies{
		EnsureChildInitialized: func(string, config.WorkspaceProjectConfig, *config.Repository, string) error { return nil },
	}))
}

func TestRunAddWorkspaceProjectsPersistsSelectedProjects(t *testing.T) {
	root := t.TempDir()
	createDetectedProject(t, root, "api")
	repo := newWorkspaceConfigRepository(t, root, domain.ModeWorkspace)
	called := 0
	deps := Dependencies{EnsureChildInitialized: func(workspaceRoot string, project config.WorkspaceProjectConfig, rootRepo *config.Repository, locale string) error {
		called++
		require.Equal(t, root, workspaceRoot)
		require.Equal(t, "api", project.ID)
		require.Same(t, repo, rootRepo)
		require.NotEmpty(t, locale)
		return nil
	}}

	require.NoError(t, runAddWorkspaceProjects(context.Background(), root, repo, []string{"api"}, deps))
	require.Equal(t, 1, called)
	require.Equal(t, []config.WorkspaceProjectConfig{{ID: "api", Path: "api", Type: "library", Language: "go"}}, repo.GetWorkspaceProjects())

	wantErr := errors.New("initialize child")
	deps.EnsureChildInitialized = func(string, config.WorkspaceProjectConfig, *config.Repository, string) error { return wantErr }
	require.ErrorIs(t, runAddWorkspaceProjects(context.Background(), root, repo, []string{"api"}, deps), wantErr)
}

func TestRunAddWorkspaceProjectsHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	createDetectedProject(t, root, "api")
	repo := newWorkspaceConfigRepository(t, root, domain.ModeWorkspace)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runAddWorkspaceProjects(ctx, root, repo, []string{"api"}, Dependencies{
		EnsureChildInitialized: func(string, config.WorkspaceProjectConfig, *config.Repository, string) error {
			t.Fatal("initializer must not run after cancellation")
			return nil
		},
	})
	require.ErrorIs(t, err, context.Canceled)
}

func newWorkspaceConfigRepository(t *testing.T, root, mode string) *config.Repository {
	t.Helper()
	repo, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "en-US")
	require.NoError(t, err)
	cfg := repo.Get()
	cfg.Project.RootPath = root
	cfg.Project.Mode = mode
	require.NoError(t, repo.Update(cfg))
	return repo
}

func createDetectedProject(t *testing.T, root, name string) {
	t.Helper()
	projectRoot := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/"+name+"\n"), 0o644))
}
