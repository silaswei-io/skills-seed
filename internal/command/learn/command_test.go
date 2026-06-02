package learn

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/git"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	servicelearner "github.com/silaswei-io/skills-seed/internal/service/learner"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCmd_HistoryDefaultsUseLearningConfig(t *testing.T) {
	repo, err := config.NewRepository(t.TempDir(), "zh-CN")
	require.NoError(t, err)

	cfg := repo.Get()
	cfg.Learning.MaxCommits = 7
	cfg.Learning.BatchSize = 3
	require.NoError(t, repo.Update(cfg))

	cmd := Cmd(&container.Container{ConfigRepo: repo})
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	limitFlag := historyCmd.Flags().Lookup("limit")
	require.NotNil(t, limitFlag)
	require.Equal(t, "7", limitFlag.DefValue)

	batchFlag := historyCmd.Flags().Lookup("batch-size")
	require.NotNil(t, batchFlag)
	require.Equal(t, "3", batchFlag.DefValue)
}

func TestCmd_CurrentIncludesFocusAndProfileFlags(t *testing.T) {
	cmd := Cmd(&container.Container{})
	currentCmd, _, err := cmd.Find([]string{"current"})
	require.NoError(t, err)

	focusFlag := currentCmd.Flags().Lookup("focus")
	require.NotNil(t, focusFlag)
	require.Equal(t, "f", focusFlag.Shorthand)

	profileFlag := currentCmd.Flags().Lookup("profile")
	require.NotNil(t, profileFlag)
	require.Equal(t, learnCurrentProfileAuto, profileFlag.DefValue)

	contextFlag := currentCmd.Flags().Lookup("context")
	require.NotNil(t, contextFlag)

	contextFileFlag := currentCmd.Flags().Lookup("context-file")
	require.NotNil(t, contextFileFlag)
}

func TestResolveFocusPaths(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "internal", "agent"), 0755))

	paths, err := resolveFocusPaths(projectRoot, []string{"internal/agent"})
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(projectRoot, "internal", "agent")}, paths)

	_, err = resolveFocusPaths(projectRoot, []string{"../outside"})
	require.Error(t, err)
}

func TestShouldRefreshProfile(t *testing.T) {
	projectRoot := t.TempDir()

	tests := []struct {
		name          string
		focusPaths    []string
		mode          string
		profileExists bool
		want          bool
		wantErr       bool
	}{
		{name: "full scan refreshes existing profile", mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "missing profile refreshes scoped scan", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileAuto, profileExists: false, want: true},
		{name: "narrow focus skips existing profile", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileAuto, profileExists: true, want: false},
		{name: "root focus refreshes", focusPaths: []string{projectRoot}, mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "critical focus refreshes", focusPaths: []string{filepath.Join(projectRoot, "internal", "domain")}, mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "multiple focus modules refresh", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent"), filepath.Join(projectRoot, "internal", "prompts")}, mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "skip mode skips", mode: learnCurrentProfileSkip, profileExists: false, want: false},
		{name: "refresh mode refreshes", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileRefresh, profileExists: true, want: true},
		{name: "invalid mode fails", mode: "later", profileExists: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shouldRefreshProfile(projectRoot, tt.focusPaths, tt.mode, tt.profileExists)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRunLearnCurrentPrintsTokenUsageLast(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.Contains(t, output, "当前代码学习完成")
	require.Contains(t, output, "Token 消耗:")
	require.Contains(t, lastNonEmptyLine(output), "Token 消耗:")
	require.NotContains(t, lastNonEmptyLine(output), "子项目")
	require.NotContains(t, output, "后续可执行:")
}

func TestRunLearnWorkspaceCurrentPrintsProjectTokenUsageAfterProjectLogs(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{
		ID:       "backend",
		Path:     "backend",
		Type:     "backend",
		Language: "go",
	}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	profileSavedIndex := strings.LastIndex(output, "已跳过项目画像刷新")
	tokenIndex := strings.LastIndex(output, "Token 消耗: 子项目 backend")
	require.NotEqual(t, -1, profileSavedIndex)
	require.NotEqual(t, -1, tokenIndex)
	require.Greater(t, tokenIndex, profileSavedIndex)

	rootRecords, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	require.Empty(t, rootRecords)

	childRepo, err := boltdb.NewPatternRepository(filepath.Join(childRoot, ".skills-seed", "memory", "project.db"))
	require.NoError(t, err)
	defer func() { require.NoError(t, childRepo.Close()) }()
	childRecords, err := childRepo.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	require.Len(t, childRecords, 1)
}

func TestRunLearnCurrentSkipsAIWhenFilesUnchanged(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileAuto)
	defer restoreLearnFlags()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	require.NoError(t, runLearnCurrent(cont))

	analyzeCalls := 0
	profileCalls := 0
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		analyzeCalls++
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeProjectFn = func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
		profileCalls++
		return &agent.AnalyzeProjectResult{}, nil
	}

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.Zero(t, analyzeCalls)
	require.Zero(t, profileCalls)
	require.Contains(t, output, "未检测到可学习文件变化")
}

func TestRunLearnCurrentUsesChangedFilesAsFocusPaths(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileAuto)
	defer restoreLearnFlags()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	require.NoError(t, runLearnCurrent(cont))

	writeLearnFile(t, cont.ConfigRepo.GetProjectConfig().RootPath, "main.go", "package main\nconst changed = true\n")
	gitAddAll(t, cont.ConfigRepo.GetProjectConfig().RootPath)

	var patternFocus []string
	var profileFocus []string
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		patternFocus = append([]string{}, req.FocusPaths...)
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeProjectFn = func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
		profileFocus = append([]string{}, req.FocusPaths...)
		return &agent.AnalyzeProjectResult{Language: "go", Summary: "profile"}, nil
	}

	require.NoError(t, runLearnCurrent(cont))

	require.Equal(t, []string{"main.go"}, patternFocus)
	require.Equal(t, []string{"main.go"}, profileFocus)
}

func TestRunLearnWorkspaceCurrentDelegatesIncrementalSkipToChildProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	require.NoError(t, runLearnCurrent(cont))

	atomic.StoreInt32(&learnWorkspaceMockAnalyzeCalls, 0)

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.Zero(t, atomic.LoadInt32(&learnWorkspaceMockAnalyzeCalls))
	require.Contains(t, output, "未检测到可学习文件变化")
}

func TestRunLearnWorkspaceCurrentWritesChildDetailsToChildLog(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	workspaceLogDir := filepath.Join(cont.SeedPath, "logs")
	require.NoError(t, logger.InitWithRetention(workspaceLogDir, "learn", logger.INFO, 0))
	workspaceLogPath := logger.CurrentLogPath()
	defer logger.Close()

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	childLogs := readLearnLogFiles(t, filepath.Join(childRoot, ".skills-seed", "logs"))
	require.NotEmpty(t, childLogs)
	require.Contains(t, strings.Join(childLogs, "\n"), `"operation":"command.learn_current"`)

	require.NoError(t, logger.Close())
	workspaceLog, err := os.ReadFile(workspaceLogPath)
	require.NoError(t, err)
	require.NotContains(t, string(workspaceLog), `"operation":"command.learn_current"`)
	require.Contains(t, string(workspaceLog), "正在学习子项目 backend")
	require.Contains(t, string(workspaceLog), "子项目 backend 独立学习完成")
	require.Contains(t, output, "正在学习子项目 backend")
	require.Contains(t, output, filepath.Join(childRoot, ".skills-seed", "logs"))
}

func TestRunLearnWorkspaceCurrentUsesConfiguredParallelism(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	var active int32
	var maxActive int32
	provider := registerLearnWorkspaceMockAgentFactoryWithAnalyze(t, func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		current := atomic.AddInt32(&active, 1)
		for {
			previous := atomic.LoadInt32(&maxActive)
			if current <= previous || atomic.CompareAndSwapInt32(&maxActive, previous, current) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		pattern := domain.NewPattern(req.ProjectName+"-pattern", "Error Handling", domain.CategoryError)
		pattern.Confidence = 0.9
		return &agent.AnalyzeCurrentCodebaseResult{Patterns: []domain.Pattern{*pattern}, Summary: "summary"}, nil
	})

	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
	}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, projects)
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	for _, project := range projects {
		initLearnWorkspaceChildProjectWithProvider(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n", provider)
	}

	require.NoError(t, runLearnCurrent(cont))

	require.GreaterOrEqual(t, atomic.LoadInt32(&maxActive), int32(2))
}

func TestRunLearnWorkspaceCurrentSuppressesChildNextSteps(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.NotContains(t, output, "后续可执行:")
	require.NotContains(t, output, "查看模式: skills-seed view patterns")
}

func TestRunLearnWorkspaceCurrentParallelModeShowsPerChildProgressWithoutDetailedLogs(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
	}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, projects)
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	for _, project := range projects {
		initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")
	}

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "frontend")
	require.Contains(t, output, "准备项目上下文")
	require.Contains(t, output, "检测增量文件变化")
	require.NotContains(t, output, "项目根路径:")
	require.NotContains(t, output, "增量文件变化:")
	require.NotContains(t, output, "后续可执行:")
	require.Contains(t, output, "子项目 backend 独立学习完成")
	require.Contains(t, output, "子项目 frontend 独立学习完成")
}

func TestRunLearnWorkspaceCurrentShowsPerChildProgressLines(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileSkip)
	defer restoreLearnFlags()
	var pauseDurations []time.Duration
	var pauseMu sync.Mutex
	restorePause := setWorkspaceChildStepPauseForTest(func(duration time.Duration) {
		pauseMu.Lock()
		pauseDurations = append(pauseDurations, duration)
		pauseMu.Unlock()
	})
	defer restorePause()

	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "front", Path: "front", Type: "frontend", Language: "typescript"},
	}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, projects)
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	for _, project := range projects {
		initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")
	}

	output := captureLearnStdout(t, func() {
		require.NoError(t, runLearnCurrent(cont))
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "front")
	require.Contains(t, output, "分析当前代码库")
	require.Contains(t, output, "backend      3/5")
	require.Contains(t, output, "完成")
	require.Contains(t, output, "学习工作区子项目")
	require.NotContains(t, output, "0/2 | backend")
	require.NotContains(t, output, "0/2 | front")
	pauseMu.Lock()
	require.NotEmpty(t, pauseDurations)
	require.Equal(t, time.Second, pauseDurations[0])
	pauseMu.Unlock()
}

func TestRunLearnWorkspaceCurrentMarksFailedChildProgress(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreLearnFlags := setLearnCurrentFlagsForTest("", nil, learnCurrentProfileAuto)
	defer restoreLearnFlags()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	provider := registerLearnWorkspaceMockAgentFactoryWithHandlers(t, learnWorkspaceMockHandlers{
		analyzeProject: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			if req.ProjectName == "front" {
				return nil, assertAnError("profile overloaded")
			}
			return &agent.AnalyzeProjectResult{Language: "go", Summary: "profile"}, nil
		},
	})
	projects := []config.WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
		{ID: "front", Path: "front", Type: "frontend", Language: "typescript"},
	}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, projects)
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	for _, project := range projects {
		initLearnWorkspaceChildProjectWithProvider(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n", provider)
	}

	output := captureLearnStdout(t, func() {
		err := runLearnCurrent(cont)
		require.Error(t, err)
		require.Contains(t, err.Error(), "front")
		require.Contains(t, err.Error(), "profile overloaded")
	})

	require.Contains(t, output, "front        5/5 失败")
	require.Contains(t, output, "学习工作区子项目")
}

func TestRunLearnWorkspaceCurrentRequiresInitializedChildProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, exec.Command("git", "-C", childRoot, "init").Run())

	err := runLearnCurrent(cont)

	require.Error(t, err)
	require.Contains(t, err.Error(), "backend")
	require.Contains(t, err.Error(), "skills-seed add")
}

func TestRunLearnWorkspaceCurrentRequiresChildGitRepository(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend")
	require.NoError(t, os.MkdirAll(filepath.Join(childRoot, ".skills-seed"), 0755))
	childConfigRepo, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := childConfigRepo.Get()
	cfg.Project.Mode = domain.ModeProject
	require.NoError(t, childConfigRepo.Update(cfg))

	err = runLearnCurrent(cont)

	require.Error(t, err)
	require.Contains(t, err.Error(), "backend")
	require.Contains(t, err.Error(), "不是独立 Git 仓库")
	require.Contains(t, err.Error(), childRoot)
}

func newLearnCurrentTestContainer(t *testing.T, mode string, projects []config.WorkspaceProjectConfig) *container.Container {
	t.Helper()

	projectRoot := initLearnGitRepo(t)
	writeLearnFile(t, projectRoot, "main.go", "package main\n")
	gitAddAll(t, projectRoot)
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := configRepo.Get()
	cfg.Project.Name = "demo"
	cfg.Project.Mode = mode
	cfg.Project.Language = "go"
	cfg.Project.RootPath = projectRoot
	cfg.Project.Locale = "zh-CN"
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	cfg.Workspace.Projects = projects
	require.NoError(t, configRepo.Update(cfg))

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "memory", "project.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, patternRepo.Close()) })

	mockAgent := &mocks.MockAgent{
		NameVal:      "mock",
		AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			agent.LogTokenUsageForContext(ctx, "mock", "AnalyzeCurrentCodebase", tokenusage.Usage{
				InputTokens:  100,
				OutputTokens: 20,
			})
			pattern := domain.NewPattern("p1", "Error Handling", domain.CategoryError)
			return &agent.AnalyzeCurrentCodebaseResult{
				Patterns: []domain.Pattern{*pattern},
				Summary:  "summary",
			}, nil
		},
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			agent.LogTokenUsageForContext(ctx, "mock", "AnalyzeProject", tokenusage.Usage{
				InputTokens:  50,
				OutputTokens: 10,
			})
			return &agent.AnalyzeProjectResult{
				Language: "go",
				Summary:  "profile",
			}, nil
		},
	}
	gitRepo := git.NewRepository(projectRoot)
	mergerSvc := merger.NewMergerService(mockAgent, patternRepo)

	return &container.Container{
		SeedPath:    seedPath,
		Config:      configRepo.Get(),
		ConfigRepo:  configRepo,
		GitRepo:     gitRepo,
		PatternRepo: patternRepo,
		ProfileRepo: profilestore.NewRepository(seedPath),
		StateRepo:   statestore.NewRepository(seedPath),
		Agent:       mockAgent,
		AnalyzerSvc: analyzer.NewAnalyzerService(mockAgent, configRepo),
		LearnerSvc:  servicelearner.NewLearnerService(mockAgent, gitRepo, patternRepo, patternRepo, mergerSvc),
		FileTracker: patternRepo,
		MergerSvc:   mergerSvc,
	}
}

var learnWorkspaceMockAnalyzeCalls int32
var learnWorkspaceFactoryMu sync.Mutex

type learnWorkspaceMockHandlers struct {
	analyzeCurrent func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error)
	analyzeProject func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error)
}

func registerLearnWorkspaceMockAgentFactory(t *testing.T) func() {
	t.Helper()

	registerLearnWorkspaceMockAgentFactoryWithAnalyze(t, nil)
	return func() {
		atomic.StoreInt32(&learnWorkspaceMockAnalyzeCalls, 0)
	}
}

func registerLearnWorkspaceMockAgentFactoryWithAnalyze(t *testing.T, analyzeFn func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error)) string {
	t.Helper()
	return registerLearnWorkspaceMockAgentFactoryWithHandlers(t, learnWorkspaceMockHandlers{analyzeCurrent: analyzeFn})
}

func registerLearnWorkspaceMockAgentFactoryWithHandlers(t *testing.T, handlers learnWorkspaceMockHandlers) string {
	t.Helper()
	provider := "mock-workspace-learn-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	learnWorkspaceFactoryMu.Lock()
	container.RegisterAgentFactory(provider, func(commandPath string, timeout time.Duration, loader *prompts.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) agent.Agent {
		return &mocks.MockAgent{
			NameVal:      provider,
			AvailableVal: true,
			AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
				if handlers.analyzeCurrent != nil {
					return handlers.analyzeCurrent(ctx, req)
				}
				atomic.AddInt32(&learnWorkspaceMockAnalyzeCalls, 1)
				agent.LogTokenUsageForContext(ctx, provider, "AnalyzeCurrentCodebase", tokenusage.Usage{
					InputTokens:  100,
					OutputTokens: 20,
				})
				pattern := domain.NewPattern("p1", "Error Handling", domain.CategoryError)
				pattern.Confidence = 0.9
				return &agent.AnalyzeCurrentCodebaseResult{
					Patterns: []domain.Pattern{*pattern},
					Summary:  "summary",
				}, nil
			},
			AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
				if handlers.analyzeProject != nil {
					return handlers.analyzeProject(ctx, req)
				}
				agent.LogTokenUsageForContext(ctx, provider, "AnalyzeProject", tokenusage.Usage{
					InputTokens:  50,
					OutputTokens: 10,
				})
				return &agent.AnalyzeProjectResult{
					Language: "go",
					Summary:  "profile",
				}, nil
			},
		}
	})
	learnWorkspaceFactoryMu.Unlock()
	atomic.StoreInt32(&learnWorkspaceMockAnalyzeCalls, 0)
	t.Cleanup(func() {
		atomic.StoreInt32(&learnWorkspaceMockAnalyzeCalls, 0)
	})
	return provider
}

func assertAnError(message string) error {
	return errors.New(message)
}

func initLearnWorkspaceChildProject(t *testing.T, workspaceRoot string, project config.WorkspaceProjectConfig, mainContent string) string {
	t.Helper()
	return initLearnWorkspaceChildProjectWithProvider(t, workspaceRoot, project, mainContent, "mock-workspace-learn-"+strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()))
}

func initLearnWorkspaceChildProjectWithProvider(t *testing.T, workspaceRoot string, project config.WorkspaceProjectConfig, mainContent string, provider string) string {
	t.Helper()

	childRoot := filepath.Join(workspaceRoot, filepath.FromSlash(project.Path))
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, exec.Command("git", "-C", childRoot, "init").Run())
	require.NoError(t, exec.Command("git", "-C", childRoot, "config", "user.email", "test@example.com").Run())
	require.NoError(t, exec.Command("git", "-C", childRoot, "config", "user.name", "Test User").Run())
	writeLearnFile(t, childRoot, "main.go", mainContent)
	gitAddAll(t, childRoot)

	childSeedPath := filepath.Join(childRoot, ".skills-seed")
	childConfigRepo, err := config.NewRepository(childSeedPath, "zh-CN")
	require.NoError(t, err)
	cfg := childConfigRepo.Get()
	cfg.Project.Name = project.ID
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.Language = project.Language
	cfg.Project.RootPath = childRoot
	cfg.Project.Locale = "zh-CN"
	cfg.Agent.Engine = provider
	cfg.Agent.Commands = map[string]string{provider: provider}
	cfg.Analysis.CodeGraph.Enabled = false
	require.NoError(t, childConfigRepo.Update(cfg))
	return childRoot
}

func setLearnCurrentFlagsForTest(testLanguage string, testFocusPaths []string, testProfileOpt string) func() {
	previousLanguage := language
	previousFocusPaths := focusPaths
	previousProfileOpt := learnCurrentProfileOpt
	previousContextText := contextText
	previousContextFile := contextFile
	language = testLanguage
	focusPaths = testFocusPaths
	learnCurrentProfileOpt = testProfileOpt
	contextText = ""
	contextFile = ""
	return func() {
		language = previousLanguage
		focusPaths = previousFocusPaths
		learnCurrentProfileOpt = previousProfileOpt
		contextText = previousContextText
		contextFile = previousContextFile
	}
}

func setWorkspaceChildStepPauseForTest(fn func(time.Duration)) func() {
	previous := sleepAfterWorkspaceChildStep
	sleepAfterWorkspaceChildStep = fn
	return func() {
		sleepAfterWorkspaceChildStep = previous
	}
}

func captureLearnStdout(t *testing.T, fn func()) string {
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

func lastNonEmptyLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

func readLearnLogFiles(t *testing.T, logDir string) []string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(logDir, "learn-*.log"))
	require.NoError(t, err)
	contents := make([]string, 0, len(matches))
	for _, match := range matches {
		data, err := os.ReadFile(match)
		require.NoError(t, err)
		contents = append(contents, string(data))
	}
	return contents
}
