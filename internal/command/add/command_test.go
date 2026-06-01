package add

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestRunAddWorkspaceProjectsDetectsAndInitializesNewChild(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	initWorkspaceRoot(t, workspaceRoot)

	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))
	initGitDir(t, childRoot)

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.NoError(t, runAddWorkspaceProjects(context.Background(), workspaceRoot, rootConfig, []string{"."}))

	reloadedRoot, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Len(t, reloadedRoot.GetWorkspaceConfig().Projects, 1)
	require.Equal(t, "backend", reloadedRoot.GetWorkspaceConfig().Projects[0].ID)
	require.Equal(t, "backend", reloadedRoot.GetWorkspaceConfig().Projects[0].Path)
	require.FileExists(t, filepath.Join(childRoot, ".skills-seed", "config.yaml"))

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, childConfig.GetProjectConfig().Mode)
	require.Equal(t, "backend", childConfig.GetProjectConfig().Name)
	require.Equal(t, "go", childConfig.GetProjectConfig().Language)
}

func TestRunAddWorkspaceProjectsDotDetectsAllChildren(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	initWorkspaceRoot(t, workspaceRoot)

	for _, name := range []string{"backend", "frontend"} {
		childRoot := filepath.Join(workspaceRoot, name)
		require.NoError(t, os.MkdirAll(childRoot, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module "+name+"\n"), 0644))
		initGitDir(t, childRoot)
	}

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.NoError(t, runAddWorkspaceProjects(context.Background(), workspaceRoot, rootConfig, []string{"."}))

	reloadedRoot, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Len(t, reloadedRoot.GetWorkspaceConfig().Projects, 2)
	require.FileExists(t, filepath.Join(workspaceRoot, "backend", ".skills-seed", "config.yaml"))
	require.FileExists(t, filepath.Join(workspaceRoot, "frontend", ".skills-seed", "config.yaml"))
}

func TestRunAddWorkspaceProjectsCanTargetSpecificChild(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	initWorkspaceRoot(t, workspaceRoot)

	for _, name := range []string{"backend", "frontend"} {
		childRoot := filepath.Join(workspaceRoot, name)
		require.NoError(t, os.MkdirAll(childRoot, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module "+name+"\n"), 0644))
		initGitDir(t, childRoot)
	}

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.NoError(t, runAddWorkspaceProjects(context.Background(), workspaceRoot, rootConfig, []string{"frontend"}))

	reloadedRoot, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Len(t, reloadedRoot.GetWorkspaceConfig().Projects, 1)
	require.Equal(t, "frontend", reloadedRoot.GetWorkspaceConfig().Projects[0].ID)
	require.FileExists(t, filepath.Join(workspaceRoot, "frontend", ".skills-seed", "config.yaml"))
	require.NoFileExists(t, filepath.Join(workspaceRoot, "backend", ".skills-seed", "config.yaml"))
}

func TestSelectWorkspaceProjectsNormalizesPathTargets(t *testing.T) {
	detected := []config.WorkspaceProjectConfig{
		{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
	}

	for _, target := range []string{"./frontend", "frontend/", `frontend\`} {
		t.Run(target, func(t *testing.T) {
			selected, err := selectWorkspaceProjects(detected, []string{target})
			require.NoError(t, err)
			require.Len(t, selected, 1)
			require.Equal(t, "frontend", selected[0].Path)
		})
	}
}

func TestRunAddWorkspaceProjectsDoesNotSyncConfigWhenChildInitializationFails(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	initWorkspaceRoot(t, workspaceRoot)

	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)

	err = runAddWorkspaceProjects(context.Background(), workspaceRoot, rootConfig, []string{"."})

	require.Error(t, err)
	reloadedRoot, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Empty(t, reloadedRoot.GetWorkspaceConfig().Projects)
	require.NoDirExists(t, filepath.Join(childRoot, ".skills-seed"))
}

func initWorkspaceRoot(t *testing.T, workspaceRoot string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(workspaceRoot, ".skills-seed"), 0755))
	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := rootConfig.Get()
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.Locale = "zh-CN"
	cfg.Project.Name = filepath.Base(workspaceRoot)
	cfg.Project.RootPath = workspaceRoot
	cfg.Workspace.Projects = nil
	require.NoError(t, rootConfig.Update(cfg))
}

func initGitDir(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0755))
}
