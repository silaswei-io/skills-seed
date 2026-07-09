package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestInitializeWorkspaceInitializesDetectedChildProjects(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDirWithOrigin(t, workspaceRoot, "git@example.com:workspace.git")
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))
	initGitDir(t, childRoot)
	shellRoot := filepath.Join(workspaceRoot, "base-xengine")
	require.NoError(t, os.MkdirAll(shellRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(shellRoot, "install.sh"), []byte("#!/bin/sh\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(shellRoot, "install.ini"), []byte("[install]\n"), 0644))
	initGitDir(t, shellRoot)

	require.NoError(t, initializeSkillWithOptions(workspaceRoot, "zh-CN", domain.ModeWorkspace, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
	}))

	require.FileExists(t, filepath.Join(workspaceRoot, ".skills-seed", "config.yaml"))
	require.FileExists(t, filepath.Join(childRoot, ".skills-seed", "config.yaml"))
	require.FileExists(t, filepath.Join(shellRoot, ".skills-seed", "config.yaml"))

	configRepo, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeWorkspace, configRepo.GetProjectConfig().Mode)
	require.Empty(t, configRepo.GetProjectConfig().Language)
	require.Equal(t, "git@example.com:workspace.git", configRepo.GetProjectConfig().GitRemote)
	require.Len(t, configRepo.GetWorkspaceConfig().Projects, 2)
	require.Equal(t, "backend", configRepo.GetWorkspaceConfig().Projects[0].Path)
	require.Equal(t, "base-xengine", configRepo.GetWorkspaceConfig().Projects[1].Path)
	require.Equal(t, "infra", configRepo.GetWorkspaceConfig().Projects[1].Type)
	require.Equal(t, "shell", configRepo.GetWorkspaceConfig().Projects[1].Language)

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, childConfig.GetProjectConfig().Mode)
	require.Equal(t, "backend", childConfig.GetProjectConfig().Name)
}

func TestInitializeProjectUsesTotalParallelismAsUnitParallelism(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:            true,
		showUserSummary:       true,
		agentTotalParallelism: 4,
		learningMode:          config.LearningModeDeep,
		learningScope:         config.LearningScopeDomain,
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, 0, configRepo.GetAgentConfig().Parallelism)
	require.Equal(t, 4, configRepo.GetCurrentLearningConfig().Parallelism)
	require.Equal(t, config.LearningModeDeep, configRepo.GetCurrentLearningConfig().Mode)
	require.Equal(t, config.LearningScopeDomain, configRepo.GetCurrentLearningConfig().Scope)
}

func TestInitializeWorkspaceAllocatesTotalParallelism(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	for _, name := range []string{"api", "web", "worker"} {
		childRoot := filepath.Join(workspaceRoot, name)
		require.NoError(t, os.MkdirAll(childRoot, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module "+name+"\n"), 0644))
		initGitDir(t, childRoot)
	}

	require.NoError(t, initializeSkillWithOptions(workspaceRoot, "zh-CN", domain.ModeWorkspace, initializeSkillOptions{
		initLogger:            true,
		showUserSummary:       true,
		agentTotalParallelism: 6,
		learningScope:         config.LearningScopeDomain,
	}))

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, 3, rootConfig.GetAgentConfig().Parallelism)
	require.Equal(t, 2, rootConfig.GetCurrentLearningConfig().Parallelism)
	require.Equal(t, config.LearningScopeDomain, rootConfig.GetCurrentLearningConfig().Scope)

	childConfig, err := config.NewRepository(filepath.Join(workspaceRoot, "api", ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, 0, childConfig.GetAgentConfig().Parallelism)
	require.Equal(t, 2, childConfig.GetCurrentLearningConfig().Parallelism)
	require.Equal(t, config.LearningScopeDomain, childConfig.GetCurrentLearningConfig().Scope)
}

func TestAllocateWorkspaceParallelism(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		projects      int
		wantWorkspace int
		wantUnit      int
	}{
		{name: "disabled", total: 0, projects: 3, wantWorkspace: 0, wantUnit: 1},
		{name: "no projects", total: 4, projects: 0, wantWorkspace: 0, wantUnit: 4},
		{name: "more total than projects", total: 6, projects: 3, wantWorkspace: 3, wantUnit: 2},
		{name: "fewer total than projects", total: 2, projects: 5, wantWorkspace: 2, wantUnit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWorkspace, gotUnit := allocateWorkspaceParallelism(tt.total, tt.projects)
			require.Equal(t, tt.wantWorkspace, gotWorkspace)
			require.Equal(t, tt.wantUnit, gotUnit)
		})
	}
}

func TestInitializeWorkspaceWithoutDetectedChildrenKeepsRootSeed(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)

	require.NoError(t, initializeSkillWithOptions(workspaceRoot, "zh-CN", domain.ModeWorkspace, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
	}))

	configRepo, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeWorkspace, configRepo.GetProjectConfig().Mode)
	require.Empty(t, configRepo.GetWorkspaceConfig().Projects)
}

func TestInteractiveInitDefaultsMatchNoArgDefaults(t *testing.T) {
	opts := withInitDefaults(commandOptions{mode: domain.ModeProject})

	require.Equal(t, "zh-CN", opts.locale)
	require.Equal(t, "en-US", opts.skillsLocale)
	require.Equal(t, domain.ModeProject, opts.mode)
	require.False(t, opts.workspace)
	require.Equal(t, "claude", opts.agent)
	require.Equal(t, "claude", opts.skills)
	require.Equal(t, config.LearningModeNormal, opts.learningMode)
	require.Equal(t, config.LearningScopeFlow, opts.learningScope)
}

func TestInteractiveInitParallelismPlanSummary(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	require.Equal(t,
		"单项目：learning.current.parallelism=4",
		initParallelismPlanSummary(domain.ModeProject, 4, 0),
	)
	require.Equal(t,
		"工作区：检测到 3 个子项目，agent.parallelism=3，learning.current.parallelism=2",
		initParallelismPlanSummary(domain.ModeWorkspace, 6, 3),
	)
}

func TestInteractiveInitWorkspaceDefaultNormalizesMode(t *testing.T) {
	opts := withInitDefaults(commandOptions{workspace: true})

	require.Equal(t, domain.ModeWorkspace, opts.mode)
	require.True(t, opts.workspace)
}

func TestIsProjectInitialized(t *testing.T) {
	projectRoot := t.TempDir()

	initialized, err := isProjectInitialized(projectRoot)
	require.NoError(t, err)
	require.False(t, initialized)

	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".skills-seed"), 0755))
	initialized, err = isProjectInitialized(projectRoot)
	require.NoError(t, err)
	require.True(t, initialized)
}

func TestPrintExistingInitSummaryDoesNotModifyConfig(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)
	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      false,
		showUserSummary: false,
		agentEngine:     "codex",
		skillsTarget:    "codex",
	}))

	cmd := Cmd()
	output := captureInitStdout(t, func() {
		require.NoError(t, printExistingInitSummary(projectRoot, cmd))
	})

	require.Contains(t, output, "当前初始化配置")
	require.Contains(t, output, "codex")
	require.Contains(t, output, filepath.Join(".skills-seed", "config.yaml"))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "codex", configRepo.GetAgentConfig().Engine)
	require.Equal(t, "codex", configRepo.GetSkillsConfig().Target)
}

func TestBannerTagsUseVersionMetadata(t *testing.T) {
	tags := bannerTags()

	require.Len(t, tags, 2)
	require.Equal(t, "cli "+metadata.ProgramVersion, tags[0].Label)
	require.NotContains(t, tags[0].Label, "open-source")
	require.NotContains(t, tags[1].Label, "open-source")
	require.True(t, strings.HasPrefix(tags[1].Label, "prompts "))
	require.Len(t, strings.TrimPrefix(tags[1].Label, "prompts "), 8)
}

func TestInitializeProjectWithAgentSetsEngine(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
		agentEngine:     "codex",
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, configRepo.GetProjectConfig().Mode)
	require.Equal(t, "codex", configRepo.GetAgentConfig().Engine)
}

func TestInitializeProjectWithSkillsSetsTarget(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
		skillsTarget:    "codex",
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, configRepo.GetProjectConfig().Mode)
	require.Equal(t, "claude", configRepo.GetAgentConfig().Engine)
	require.Equal(t, "codex", configRepo.GetSkillsConfig().Target)
	require.Equal(t, ".agents/skills/skills-seed-skills", configRepo.GetSkillsConfig().Paths["codex"])
}

func TestInitializeProjectWithSkillsLocaleSetsAISkillsContentLanguage(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
		skillsLocale:    "zh-CN",
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "zh-CN", configRepo.GetToolLocale())
	require.Equal(t, "zh-CN", configRepo.GetSkillsLocale())
}

func TestCmdAcceptsSeparateSkillsLocale(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)
	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectRoot))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousDir))
	})

	cmd := Cmd()
	cmd.SetArgs([]string{"--locale", "zh-CN", "--skills-locale", "zh-CN"})
	require.NoError(t, cmd.Execute())

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "zh-CN", configRepo.GetToolLocale())
	require.Equal(t, "zh-CN", configRepo.GetSkillsLocale())
}

func TestCmdDefaultsSkillsLocaleToEnglishButKeepsContextTemplatesInToolLocale(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)
	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectRoot))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousDir))
	})

	cmd := Cmd()
	cmd.SetArgs([]string{"--locale", "zh-CN"})
	require.NoError(t, cmd.Execute())

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "zh-CN", configRepo.GetToolLocale())
	require.Equal(t, "en-US", configRepo.GetSkillsLocale())

	profile, err := os.ReadFile(filepath.Join(projectRoot, ".skills-seed", "context", "background.md"))
	require.NoError(t, err)
	require.Contains(t, string(profile), "# 背景与外部事实")
	require.NotContains(t, string(profile), "# Background and External Facts")

	instructions, err := os.ReadFile(filepath.Join(projectRoot, ".skills-seed", "context", "constraints.md"))
	require.NoError(t, err)
	require.Contains(t, string(instructions), "# 约束与边界")
	require.NotContains(t, string(instructions), "# Constraints and Boundaries")
}

func TestInitializeProjectDetectsFrontendLanguage(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{
  "scripts": {"dev": "vite --host 0.0.0.0"},
  "dependencies": {"@vitejs/plugin-react": "latest", "react": "latest"},
  "devDependencies": {"typescript": "latest", "vite": "latest"}
}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "tsconfig.json"), []byte(`{"compilerOptions": {}}`), 0644))

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "typescript", configRepo.GetProjectConfig().Language)
}

func TestInitializeProjectDetectsJavaScriptFrontendLanguage(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte(`{
  "scripts": {"dev": "vite --host 0.0.0.0"},
  "dependencies": {"@vitejs/plugin-react": "latest", "react": "latest"},
  "devDependencies": {"vite": "latest"}
}`), 0644))

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "javascript", configRepo.GetProjectConfig().Language)
}

func TestInitializeProjectWithAgentAndSkillsCanDiffer(t *testing.T) {
	projectRoot := t.TempDir()
	initGitDir(t, projectRoot)

	require.NoError(t, initializeSkillWithOptions(projectRoot, "zh-CN", domain.ModeProject, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
		agentEngine:     "claude",
		skillsTarget:    "codex",
	}))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "claude", configRepo.GetAgentConfig().Engine)
	require.Equal(t, "codex", configRepo.GetSkillsConfig().Target)
	require.Equal(t, ".agents/skills/skills-seed-skills", configRepo.GetSkillsConfig().Paths["codex"])
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

func TestInitializeWorkspaceInitializesChildProjectsWithRootAgent(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))
	initGitDir(t, childRoot)

	require.NoError(t, initializeSkillWithOptions(workspaceRoot, "zh-CN", domain.ModeWorkspace, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
		agentEngine:     "codex",
	}))

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeWorkspace, rootConfig.GetProjectConfig().Mode)
	require.Equal(t, "codex", rootConfig.GetAgentConfig().Engine)

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, childConfig.GetProjectConfig().Mode)
	require.Equal(t, "backend", childConfig.GetProjectConfig().Name)
	require.Equal(t, "go", childConfig.GetProjectConfig().Language)
	require.Equal(t, childRoot, childConfig.GetProjectConfig().RootPath)
	require.Equal(t, "codex", childConfig.GetAgentConfig().Engine)
}

func TestInitializeWorkspaceInitializesChildProjectsWithRootSkills(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))
	initGitDir(t, childRoot)

	require.NoError(t, initializeSkillWithOptions(workspaceRoot, "zh-CN", domain.ModeWorkspace, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
		agentEngine:     "claude",
		skillsTarget:    "codex",
	}))

	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeWorkspace, rootConfig.GetProjectConfig().Mode)
	require.Equal(t, "claude", rootConfig.GetAgentConfig().Engine)
	require.Equal(t, "codex", rootConfig.GetSkillsConfig().Target)
	require.Equal(t, ".agents/skills/skills-seed-skills", rootConfig.GetSkillsConfig().Paths["codex"])

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, childConfig.GetProjectConfig().Mode)
	require.Equal(t, "claude", childConfig.GetAgentConfig().Engine)
	require.Equal(t, "codex", childConfig.GetSkillsConfig().Target)
	require.Equal(t, ".agents/skills/skills-seed-skills", childConfig.GetSkillsConfig().Paths["codex"])
}

func TestInitializeWorkspaceChildrenFailsForConfiguredChildWithoutGitRepository(t *testing.T) {
	workspaceRoot := t.TempDir()
	initGitDir(t, workspaceRoot)
	childRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module backend\n"), 0644))

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

	err = initializeWorkspaceChildrenWithRepo(workspaceRoot, "zh-CN", rootConfig)
	require.Error(t, err)
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

	require.NoError(t, initializeWorkspaceChildrenWithRepo(workspaceRoot, "zh-CN", rootConfig))

	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, domain.ModeProject, childConfig.GetProjectConfig().Mode)
	require.Equal(t, "backend", childConfig.GetProjectConfig().Name)
	require.Equal(t, "go", childConfig.GetProjectConfig().Language)
	require.Equal(t, childRoot, childConfig.GetProjectConfig().RootPath)
	require.NoFileExists(t, filepath.Join(childRoot, ".skills-seed", "context", "workspace.md"))
}

func TestInitializeWorkspaceChildrenReportsExistingChildWithSameAgent(t *testing.T) {
	workspaceRoot, childRoot := initWorkspaceWithInitializedChild(t, "codex", "codex")
	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)

	output := captureInitStdout(t, func() {
		require.NoError(t, initializeWorkspaceChildrenWithRepo(workspaceRoot, "zh-CN", rootConfig))
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "codex")
	require.Contains(t, output, "agent 相同")
	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "codex", childConfig.GetAgentConfig().Engine)
}

func TestInitializeWorkspaceChildrenReportsExistingChildWithDifferentAgent(t *testing.T) {
	workspaceRoot, childRoot := initWorkspaceWithInitializedChild(t, "codex", "claude")
	rootConfig, err := config.NewRepository(filepath.Join(workspaceRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)

	output := captureInitStdout(t, func() {
		require.NoError(t, initializeWorkspaceChildrenWithRepo(workspaceRoot, "zh-CN", rootConfig))
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "codex")
	require.Contains(t, output, "claude")
	require.Contains(t, output, "agent 不同")
	childConfig, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "claude", childConfig.GetAgentConfig().Engine)
}

func TestEnsureWorkspaceContextFilesDoesNotCreateChildProjectContext(t *testing.T) {
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

	require.NoError(t, ensureWorkspaceContextFiles(seedPath, projectRoot, "demo", configRepo))

	require.FileExists(t, filepath.Join(seedPath, "context", "workspace.md"))
	_, err = os.Stat(filepath.Join(seedPath, "context", "projects", "backend"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsureWorkspaceContextFilesDoesNotWriteRuntimePathPlaceholders(t *testing.T) {
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

	require.NoError(t, ensureWorkspaceContextFiles(seedPath, projectRoot, "hsm-workspace", configRepo))

	content, err := os.ReadFile(filepath.Join(seedPath, "context", "workspace.md"))
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "# 工作区背景")
	require.NotContains(t, text, "# Workspace Background")
	require.Contains(t, text, "hsmwebapi")
	require.NotContains(t, text, "Workspace Profile Prompt")
	require.NotContains(t, text, "Analysis Goal")
	require.NotContains(t, text, "<workspace-input-file>")
	require.NotContains(t, text, "<workspace-profile-file>")
	require.NotContains(t, text, "<user-context-path>")
}

func initGitDir(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0755))
}

func initGitDirWithOrigin(t *testing.T, root, origin string) {
	t.Helper()
	initGitDir(t, root)
	configDir := filepath.Join(root, ".git")
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config"), []byte("[remote \"origin\"]\n\turl = "+origin+"\n"), 0644))
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
	cfg.Agent.Engine = rootAgent
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
	childCfg.Agent.Engine = childAgent
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
