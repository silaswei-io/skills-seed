package generate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCmdDoesNotExposeWorkspaceChildSkillPolicyFlags(t *testing.T) {
	cmd := Cmd(&container.Container{})

	require.Nil(t, cmd.Flags().Lookup("overwrite"))
	require.Nil(t, cmd.Flags().Lookup("root-only"))
	require.NotNil(t, cmd.Flags().Lookup("context"))
	require.NotNil(t, cmd.Flags().Lookup("context-file"))
}

func TestRunGenerateWorkspaceGeneratesChildrenBeforeRootSkill(t *testing.T) {
	resetGenerateFlagsForTest(t)
	provider := registerGenerateWorkspaceMockAgentFactory(t)
	workspaceRoot := t.TempDir()
	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}

	childRoot := initGenerateWorkspaceChildProject(t, workspaceRoot, project, provider)
	seedGenerateChildMemory(t, childRoot, "Backend Rule")
	cont := initGenerateWorkspaceRootContainer(t, workspaceRoot, provider, []config.WorkspaceProjectConfig{project})
	defer cont.Close()

	cmd := Cmd(cont)
	require.NoError(t, runGenerate(cont, cmd))

	require.FileExists(t, filepath.Join(childRoot, ".agents", "skills", "backend-dev", "SKILL.md"))
	rootOverview := readGenerateFile(t, workspaceRoot, ".agents", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.Contains(t, rootOverview, "backend/.agents/skills/backend-dev/SKILL.md")
	require.Contains(t, rootOverview, "backend 开发技能")
}

func TestRunGenerateWorkspaceSkipsManualChildSkillAndKeepsRootGenerated(t *testing.T) {
	resetGenerateFlagsForTest(t)
	provider := registerGenerateWorkspaceMockAgentFactory(t)
	workspaceRoot := t.TempDir()
	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}

	childRoot := initGenerateWorkspaceChildProject(t, workspaceRoot, project, provider)
	seedGenerateChildMemory(t, childRoot, "Backend Rule")
	manualSkillDir := filepath.Join(childRoot, ".agents", "skills", "backend-dev")
	require.NoError(t, os.MkdirAll(manualSkillDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(manualSkillDir, "SKILL.md"), []byte("# Manual Backend Skill\n"), 0644))
	cont := initGenerateWorkspaceRootContainer(t, workspaceRoot, provider, []config.WorkspaceProjectConfig{project})
	defer cont.Close()

	cmd := Cmd(cont)
	require.NoError(t, runGenerate(cont, cmd))

	require.Equal(t, "# Manual Backend Skill\n", readGenerateFile(t, childRoot, ".agents", "skills", "backend-dev", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(childRoot, ".agents", "skills", "backend-dev", "references", "project-spec.md"))
	rootOverview := readGenerateFile(t, workspaceRoot, ".agents", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.Contains(t, rootOverview, "Manual Backend Skill")
}

func TestGenerateWorkspaceChildSkillsUsesConfiguredParallelism(t *testing.T) {
	resetGenerateFlagsForTest(t)
	var active int32
	var maxActive int32
	provider := registerGenerateWorkspaceMockAgentFactoryWithSummary(t, func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
		current := atomic.AddInt32(&active, 1)
		for {
			previous := atomic.LoadInt32(&maxActive)
			if current <= previous || atomic.CompareAndSwapInt32(&maxActive, previous, current) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return &agent.GenerateSkillsResult{}, nil
	})

	workspaceRoot := t.TempDir()
	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
	}
	for _, project := range projects {
		childRoot := initGenerateWorkspaceChildProject(t, workspaceRoot, project, provider)
		setGenerateChildOutputPath(t, childRoot, provider, filepath.Join(".agents", "skills", project.ID+"-dev"))
		seedGenerateChildMemory(t, childRoot, project.ID+" Rule")
	}
	cont := initGenerateWorkspaceRootContainer(t, workspaceRoot, provider, projects)
	defer cont.Close()
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))

	require.NoError(t, generateWorkspaceChildSkills(context.Background(), cont))

	require.GreaterOrEqual(t, atomic.LoadInt32(&maxActive), int32(2))
}

func TestRunGenerateWorkspacePrintsConcurrentChildProjectProgress(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	resetGenerateFlagsForTest(t)
	stubGenerateChildStepSleep(t, func(time.Duration) {})
	provider := registerGenerateWorkspaceMockAgentFactory(t)
	workspaceRoot := t.TempDir()
	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
	}
	for _, project := range projects {
		childRoot := initGenerateWorkspaceChildProject(t, workspaceRoot, project, provider)
		seedGenerateChildMemory(t, childRoot, project.ID+" Rule")
	}
	cont := initGenerateWorkspaceRootContainer(t, workspaceRoot, provider, projects)
	defer cont.Close()

	output := captureGenerateStdout(t, func() {
		cmd := Cmd(cont)
		require.NoError(t, runGenerate(cont, cmd))
	})

	require.Contains(t, output, "生成工作区子项目 skills")
	require.Contains(t, output, "backend")
	require.Contains(t, output, "frontend")
	require.Contains(t, output, "写入技能文件")
	require.Contains(t, output, "生成 skills 摘要")
	require.Contains(t, output, "读取项目画像")
	require.Contains(t, output, "backend      5/5")
	require.Contains(t, output, "frontend     5/5")
	require.Contains(t, output, "2/2 生成工作区子项目 skills")
	require.NotContains(t, output, "2/2 - 写入技能文件")
	require.NotContains(t, output, "统计已学习模式")
	require.NotContains(t, output, "backend      1/1")
}

func TestRunGenerateWorkspaceShowsRootSkillWriteAfterChildProjectProgress(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	resetGenerateFlagsForTest(t)
	stubGenerateChildStepSleep(t, func(time.Duration) {})
	provider := registerGenerateWorkspaceMockAgentFactory(t)
	workspaceRoot := t.TempDir()
	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
	}
	for _, project := range projects {
		childRoot := initGenerateWorkspaceChildProject(t, workspaceRoot, project, provider)
		seedGenerateChildMemory(t, childRoot, project.ID+" Rule")
	}
	cont := initGenerateWorkspaceRootContainer(t, workspaceRoot, provider, projects)
	defer cont.Close()

	output := captureGenerateStdout(t, func() {
		cmd := Cmd(cont)
		require.NoError(t, runGenerate(cont, cmd))
	})

	childIndex := strings.Index(output, "生成工作区子项目 skills")
	rootIndex := strings.Index(output, "写入工作区根 skills")
	require.NotEqual(t, -1, childIndex, output)
	require.NotEqual(t, -1, rootIndex, output)
	require.Less(t, childIndex, rootIndex, output)
}

func TestGenerateWorkspaceChildSkillsDoesNotPassWorkspaceContextToChildren(t *testing.T) {
	resetGenerateFlagsForTest(t)
	var seenContextsMu sync.Mutex
	var seenContexts []string
	provider := registerGenerateWorkspaceMockAgentFactoryWithSummary(t, func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
		seenContextsMu.Lock()
		defer seenContextsMu.Unlock()
		seenContexts = append(seenContexts, req.UserContext)
		return &agent.GenerateSkillsResult{}, nil
	})

	workspaceRoot := t.TempDir()
	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	childRoot := initGenerateWorkspaceChildProject(t, workspaceRoot, project, provider)
	seedGenerateChildMemory(t, childRoot, "Backend Rule")
	cont := initGenerateWorkspaceRootContainer(t, workspaceRoot, provider, []config.WorkspaceProjectConfig{project})
	defer cont.Close()

	ctx := runtimecontext.WithUserContext(context.Background(), "workspace 根说明不能透传给子项目")
	require.NoError(t, generateWorkspaceChildSkills(ctx, cont))

	require.Equal(t, []string{""}, seenContexts)
}

func TestRunGenerateWorkspacePrintsWorkspaceProgressBeforeChildDetailsAndTokenUsage(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	resetGenerateFlagsForTest(t)
	var provider string
	provider = registerGenerateWorkspaceMockAgentFactoryWithSummary(t, func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
		agent.LogTokenUsageForContext(ctx, provider, "GenerateSkillsSummary", tokenusage.Usage{
			InputTokens:  1_000,
			OutputTokens: 200,
		})
		return &agent.GenerateSkillsResult{}, nil
	})
	workspaceRoot := t.TempDir()
	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}

	childRoot := initGenerateWorkspaceChildProject(t, workspaceRoot, project, provider)
	seedGenerateChildMemory(t, childRoot, "Backend Rule")
	cont := initGenerateWorkspaceRootContainer(t, workspaceRoot, provider, []config.WorkspaceProjectConfig{project})
	defer cont.Close()

	output := captureGenerateStdout(t, func() {
		cmd := Cmd(cont)
		require.NoError(t, runGenerate(cont, cmd))
	})

	stepIndex := strings.Index(output, "生成工作区子项目 skills")
	startIndex := strings.Index(output, "子项目 backend 开始生成 skills")
	doneIndex := strings.Index(output, "子项目 backend skills 生成完成")
	tokenIndex := strings.Index(output, "Token 消耗: 子项目 backend")
	require.NotEqual(t, -1, stepIndex, output)
	require.NotEqual(t, -1, startIndex, output)
	require.NotEqual(t, -1, doneIndex, output)
	require.NotEqual(t, -1, tokenIndex, output)
	require.Greater(t, startIndex, stepIndex)
	require.Greater(t, doneIndex, stepIndex)
	require.Greater(t, tokenIndex, doneIndex)
}

func TestRunGenerateProjectRequiresAvailableAgent(t *testing.T) {
	resetGenerateFlagsForTest(t)
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Mode = domain.ModeProject
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	require.NoError(t, configRepo.Update(cfg))

	cont := &container.Container{
		ConfigRepo: configRepo,
		Agent:      &mocks.MockAgent{NameVal: "mock", AvailableVal: false},
	}

	err = commandutil.RequireAgentAvailable(cont)
	require.Error(t, err)
}

func TestRunGenerateWorkspaceRequiresAvailableAgent(t *testing.T) {
	resetGenerateFlagsForTest(t)
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	require.NoError(t, configRepo.Update(cfg))

	cont := &container.Container{
		ConfigRepo: configRepo,
		Agent:      &mocks.MockAgent{NameVal: "mock", AvailableVal: false},
	}

	err = commandutil.RequireAgentAvailable(cont)
	require.Error(t, err)
}

func TestOutputPathForCurrentTargetUsesConfiguredSkillsTarget(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Agent.Engine = "claude"
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{
		"claude": ".claude/skills/skills-seed-skills",
		"codex":  ".agents/skills/skills-seed-skills",
	}
	require.NoError(t, configRepo.Update(cfg))

	require.Equal(t, ".agents/skills/skills-seed-skills", outputPathForCurrentTarget(&container.Container{ConfigRepo: configRepo}))
}

func resetGenerateFlagsForTest(t *testing.T) {
	t.Helper()
	previousOutputPath := outputPath
	previousMerge := merge
	previousContextText := contextText
	previousContextFile := contextFile
	outputPath = ""
	merge = false
	contextText = ""
	contextFile = ""
	t.Cleanup(func() {
		outputPath = previousOutputPath
		merge = previousMerge
		contextText = previousContextText
		contextFile = previousContextFile
	})
}

func registerGenerateWorkspaceMockAgentFactory(t *testing.T) string {
	t.Helper()
	return registerGenerateWorkspaceMockAgentFactoryWithSummary(t, nil)
}

var generateWorkspaceFactoryMu sync.Mutex

func registerGenerateWorkspaceMockAgentFactoryWithSummary(t *testing.T, summaryFn func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error)) string {
	t.Helper()
	provider := "mock-generate-workspace-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	generateWorkspaceFactoryMu.Lock()
	container.RegisterAgentFactory(provider, func(commandPath string, timeout time.Duration, loader *prompts.Loader, allowUserPlugins bool) agent.Agent {
		return &mocks.MockAgent{
			NameVal:      provider,
			AvailableVal: true,
			GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
				if summaryFn != nil {
					return summaryFn(ctx, req)
				}
				return &agent.GenerateSkillsResult{}, nil
			},
		}
	})
	generateWorkspaceFactoryMu.Unlock()
	return provider
}

func initGenerateWorkspaceRootContainer(t *testing.T, workspaceRoot, provider string, projects []config.WorkspaceProjectConfig) *container.Container {
	t.Helper()

	seedPath := filepath.Join(workspaceRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = "demo"
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.Language = "go"
	cfg.Project.Locale = "zh-CN"
	cfg.Project.RootPath = workspaceRoot
	cfg.Agent.Engine = provider
	cfg.Agent.Commands = map[string]string{provider: provider}
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": ".agents/skills/skills-seed-skills"}
	cfg.Workspace.Projects = projects
	require.NoError(t, configRepo.Update(cfg))

	cont, err := container.NewContainer(context.Background(), seedPath)
	require.NoError(t, err)
	return cont
}

func initGenerateWorkspaceChildProject(t *testing.T, workspaceRoot string, project config.WorkspaceProjectConfig, provider string) string {
	t.Helper()

	childRoot := filepath.Join(workspaceRoot, filepath.FromSlash(project.Path))
	require.NoError(t, os.MkdirAll(filepath.Join(childRoot, ".git"), 0755))

	childSeedPath := filepath.Join(childRoot, ".skills-seed")
	childConfigRepo, err := config.NewRepository(childSeedPath, "zh-CN")
	require.NoError(t, err)
	cfg := childConfigRepo.Get()
	cfg.Project.Name = project.ID
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.Language = project.Language
	cfg.Project.Locale = "zh-CN"
	cfg.Project.RootPath = childRoot
	cfg.Agent.Engine = provider
	cfg.Agent.Commands = map[string]string{provider: provider}
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": ".agents/skills/backend-dev"}
	require.NoError(t, childConfigRepo.Update(cfg))
	return childRoot
}

func setGenerateChildOutputPath(t *testing.T, childRoot, provider, outputPath string) {
	t.Helper()
	childConfigRepo, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := childConfigRepo.Get()
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": outputPath}
	require.NoError(t, childConfigRepo.Update(cfg))
}

func seedGenerateChildMemory(t *testing.T, childRoot, patternName string) {
	t.Helper()
	ctx := context.Background()
	seedPath := filepath.Join(childRoot, ".skills-seed")

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "memory", "project.db"))
	require.NoError(t, err)
	defer patternRepo.Close()

	pattern := domain.NewPattern("p1", patternName, domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("workspace child pattern")
	pattern.SetRule("render child skills before root skills")
	require.NoError(t, patternRepo.Save(ctx, pattern))

	profileRepo := profilestore.NewRepository(seedPath)
	require.NoError(t, profileRepo.Save(ctx, &domain.ProjectProfile{
		ProjectName: "backend",
		Language:    "go",
		Summary:     "backend service profile",
		GeneratedAt: "2026-05-27 00:00:00",
	}))
}

func readGenerateFile(t *testing.T, root string, parts ...string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	require.NoError(t, err)
	return string(content)
}

func captureGenerateStdout(t *testing.T, fn func()) string {
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
	return string(data)
}

func stubGenerateChildStepSleep(t *testing.T, fn func(time.Duration)) {
	t.Helper()
	previous := sleepAfterGenerateChildStep
	sleepAfterGenerateChildStep = fn
	t.Cleanup(func() {
		sleepAfterGenerateChildStep = previous
	})
}
