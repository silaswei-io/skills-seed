package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestInitializeWorkspaceDoesNotInitializeChildProjects(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))
	initGitDir(t, childRoot)

	require.NoError(t, initializeSkillAt(workspaceRoot, "zh-CN", domain.ModeWorkspace))

	require.FileExists(t, filepath.Join(workspaceRoot, ".skills-seed", "config.yaml"))
	require.NoFileExists(t, filepath.Join(childRoot, ".skills-seed", "config.yaml"))

	configRepo, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeWorkspace, configRepo.GetProjectConfig().Mode)
	require.Len(t, configRepo.GetWorkspaceConfig().Projects, 1)
	require.Equal(t, "backend", configRepo.GetWorkspaceConfig().Projects[0].Path)
}

func TestInitializeProjectWithAgentSetsProvider(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
		agentProvider:   "codex",
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, configRepo.GetProjectConfig().Mode)
	require.Equal(t, "codex", configRepo.GetAgentConfig().Provider)
}

func TestInitializeProjectSummaryUsesRelativeSeedPathAndDocumentationLink(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)

	output := captureInitStdout(t, func() {
		require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
			initLogger:      false,
			showUserSummary: true,
		}))
	})

	require.Contains(t, output, "初始化成功: .skills-seed")
	require.Contains(t, output, "文档参考: https://github.com/silaswei-io/skills-seed/blob/"+metadata.ProgramVersion+"/README.md")
	require.NotContains(t, output, projectRoot)
	require.NotContains(t, output, "可选后续步骤")
	require.NotContains(t, output, "skills-seed learn current")
}

func TestInitializeWorkspaceWithChildrenInitializesChildProjects(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))
	initGitDir(t, childRoot)

	require.NoError(t, initializeSkillWithOptions(workspaceRoot, "zh-CN", domain.ModeWorkspace, initializeSkillOptions{
		initLogger:            true,
		showUserSummary:       true,
		initWorkspaceChildren: true,
		agentProvider:         "codex",
	}))

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeWorkspace, rootConfig.GetProjectConfig().Mode)
	require.Equal(t, "codex", rootConfig.GetAgentConfig().Provider)

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, childConfig.GetProjectConfig().Mode)
	require.Equal(t, "backend", childConfig.GetProjectConfig().Name)
	require.Equal(t, "go", childConfig.GetProjectConfig().Language)
	require.Equal(t, childRoot, childConfig.GetProjectConfig().RootPath)
	require.Equal(t, "codex", childConfig.GetAgentConfig().Provider)
}

func TestInitializeWorkspaceWithChildrenRemovesRootSeedWhenChildInitializationFails(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))

	err := initializeSkillWithOptions(workspaceRoot, "zh-CN", domain.ModeWorkspace, initializeSkillOptions{
		initLogger:            true,
		showUserSummary:       true,
		initWorkspaceChildren: true,
	})

	require.Error(t, err)
	require.NoDirExists(t, filepath.Join(workspaceRoot, ".skills-seed"))
	require.NoDirExists(t, filepath.Join(childRoot, ".skills-seed"))
}

func TestInitializeWorkspaceChildrenCreatesProjectModeSeeds(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	initGitDir(t, childRoot)

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := rootConfig.Get()
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.Locale = "zh-CN"
	cfg.Project.RootPath = workspaceRoot
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
	}
	require.NoError(t, rootConfig.Update(cfg))

	require.NoError(t, initializeWorkspaceChildrenAt(workspaceRoot, "zh-CN"))

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, childConfig.GetProjectConfig().Mode)
	require.Equal(t, "backend", childConfig.GetProjectConfig().Name)
	require.Equal(t, "go", childConfig.GetProjectConfig().Language)
	require.Equal(t, childRoot, childConfig.GetProjectConfig().RootPath)
	require.NoFileExists(t, filepath.Join(childRoot, ".skills-seed", "prompts", "workspace", "workspace-profile.md"))
}

func TestInitializeWorkspaceChildrenReportsExistingChildWithSameAgent(t *testing.T) {
	workspaceRoot, childRoot := initWorkspaceWithInitializedChild(t, "codex", "codex")

	output := captureInitStdout(t, func() {
		require.NoError(t, initializeWorkspaceChildrenAt(workspaceRoot, "zh-CN"))
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "codex")
	require.Contains(t, output, "agent 相同")
	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "codex", childConfig.GetAgentConfig().Provider)
}

func TestInitializeWorkspaceChildrenReportsExistingChildWithDifferentAgent(t *testing.T) {
	workspaceRoot, childRoot := initWorkspaceWithInitializedChild(t, "codex", "claude")

	output := captureInitStdout(t, func() {
		require.NoError(t, initializeWorkspaceChildrenAt(workspaceRoot, "zh-CN"))
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "codex")
	require.Contains(t, output, "claude")
	require.Contains(t, output, "agent 不同")
	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "claude", childConfig.GetAgentConfig().Provider)
}

func TestEnsureWorkspacePromptFilesDoesNotCreateChildProjectPrompts(t *testing.T) {
	seedPath := t.TempDir()
	projectRoot := t.TempDir()
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := configRepo.Get()
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.Locale = "zh-CN"
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
	}
	require.NoError(t, configRepo.Update(cfg))

	require.NoError(t, ensureWorkspacePromptFiles(seedPath, projectRoot, "demo", configRepo))

	require.FileExists(t, filepath.Join(seedPath, "prompts", "workspace", "workspace-profile.md"))
	require.FileExists(t, filepath.Join(seedPath, "prompts", "workspace", "workspace-spec.md"))
	_, err = os.Stat(filepath.Join(seedPath, "prompts", "projects", "backend"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsureWorkspacePromptFilesDoesNotWriteRuntimePathPlaceholders(t *testing.T) {
	seedPath := t.TempDir()
	projectRoot := t.TempDir()
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := configRepo.Get()
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.Locale = "zh-CN"
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{
		{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
	}
	require.NoError(t, configRepo.Update(cfg))

	require.NoError(t, ensureWorkspacePromptFiles(seedPath, projectRoot, "hsm-workspace", configRepo))

	for _, name := range []string{"workspace-profile.md", "workspace-spec.md"} {
		content, err := os.ReadFile(filepath.Join(seedPath, "prompts", "workspace", name))
		require.NoError(t, err)
		text := string(content)
		require.Contains(t, text, "hsmwebapi")
		require.NotContains(t, text, "<workspace-input-file>")
		require.NotContains(t, text, "<workspace-profile-file>")
		require.NotContains(t, text, "<user-context-file>")
	}
}

func initGitDir(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0755))
}

func initWorkspaceWithInitializedChild(t *testing.T, rootAgent, childAgent string) (string, string) {
	t.Helper()

	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	initGitDir(t, childRoot)

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := rootConfig.Get()
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.Locale = "zh-CN"
	cfg.Project.RootPath = workspaceRoot
	cfg.Agent.Provider = rootAgent
	cfg.Agent.Commands = map[string]string{rootAgent: rootAgent}
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
	}
	require.NoError(t, rootConfig.Update(cfg))

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	childCfg := childConfig.Get()
	childCfg.Project.Mode = domain.ModeProject
	childCfg.Project.Locale = "zh-CN"
	childCfg.Project.RootPath = childRoot
	childCfg.Agent.Provider = childAgent
	childCfg.Agent.Commands = map[string]string{childAgent: childAgent}
	require.NoError(t, childConfig.Update(childCfg))

	return workspaceRoot, childRoot
}

func captureInitStdout(t *testing.T, fn func()) string {
	t.Helper()

	tempFile, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)

	originalStdout := os.Stdout
	os.Stdout = tempFile
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	require.NoError(t, tempFile.Close())
	data, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	return stripANSI(string(data))
}

func stripANSI(s string) string {
	replacer := strings.NewReplacer("\033[37m", "", "\033[33m", "", "\033[31m", "", "\033[0m", "")
	return replacer.Replace(s)
}
