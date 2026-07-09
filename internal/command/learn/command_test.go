package learn

import (
	"context"
	"encoding/json"
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
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	workspacestore "github.com/silaswei-io/skills-seed/internal/infra/storage/workspace"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/pkg/tokenusage"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	servicelearner "github.com/silaswei-io/skills-seed/internal/service/learner"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCmd_HistoryDefaultsUseLearningConfig(t *testing.T) {
	repo, err := config.NewRepository(t.TempDir(), "zh-CN")
	require.NoError(t, err)

	cfg := repo.Get()
	cfg.Learning.History.MaxCommits = 7
	cfg.Learning.History.BatchSize = 3
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

	contextPathFlag := currentCmd.Flags().Lookup("context-path")
	require.NotNil(t, contextPathFlag)

	forceFlag := currentCmd.Flags().Lookup("force")
	require.NotNil(t, forceFlag)
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

func requireRunLearnCurrentNoError(t *testing.T, cont *container.Container, opts learnCurrentOptions) domain.LearnCurrentResult {
	t.Helper()

	result, err := runLearnCurrent(cont, opts)
	require.NoError(t, err)
	return result
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
		{name: "full scan skips existing profile", mode: learnCurrentProfileAuto, profileExists: true, want: false},
		{name: "missing profile refreshes scoped scan", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileAuto, profileExists: false, want: true},
		{name: "narrow focus skips existing profile", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileAuto, profileExists: true, want: false},
		{name: "root focus skips existing profile", focusPaths: []string{projectRoot}, mode: learnCurrentProfileAuto, profileExists: true, want: false},
		{name: "critical focus skips existing profile", focusPaths: []string{filepath.Join(projectRoot, "internal", "domain")}, mode: learnCurrentProfileAuto, profileExists: true, want: false},
		{name: "multiple focus modules skips existing profile", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent"), filepath.Join(projectRoot, "internal", "prompts")}, mode: learnCurrentProfileAuto, profileExists: true, want: false},
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
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Contains(t, output, "当前代码增量学习完成")
	require.Contains(t, output, "Token 消耗:")
	require.Contains(t, lastNonEmptyLine(output), "Token 消耗:")
	require.NotContains(t, lastNonEmptyLine(output), "子项目")
	require.NotContains(t, output, "后续可执行:")
}

func TestRunLearnCurrentWritesSnapshotsAfterFirstLearning(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	requireRunLearnCurrentNoError(t, cont, opts)

	content, err := os.ReadFile(filepath.Join(cont.SeedPath, "cache", "snapshots", "main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(content))
}

func TestRunLearnCurrentReportsProjectSummaryWhenPatternsSaved(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	result, err := runLearnCurrent(cont, opts)

	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.PatternsSaved)
}

func TestRunLearnCurrentForceRelearnsUnchangedFiles(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	requireRunLearnCurrentNoError(t, cont, learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip))

	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	opts.force = true
	output := captureLearnStdout(t, func() {
		result := requireRunLearnCurrentNoError(t, cont, opts)
		require.False(t, result.Summary.NoFileChanges)
		require.Greater(t, result.Summary.ChangedFiles, 0)
		require.Greater(t, result.Summary.PatternsSaved, 0)
	})

	require.NotContains(t, output, "未检测到可学习文件变化")
}

func TestRunLearnCurrentWithFocusUpdatesFocusedSnapshotsAfterAnalysis(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/deleted.go", "package internal\n")
	gitAddAll(t, projectRoot)
	requireRunLearnCurrentNoError(t, cont, learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip))

	require.NoError(t, os.Remove(filepath.Join(projectRoot, "internal", "deleted.go")))
	gitAddAll(t, projectRoot)
	snapshotDir := filepath.Join(cont.SeedPath, "cache", "snapshots")
	require.NoError(t, os.WriteFile(filepath.Join(snapshotDir, "stale.go"), []byte("stale snapshot\n"), 0644))

	opts := learnCurrentOptionsForTest("", []string{"main.go", "internal"}, learnCurrentProfileSkip)
	requireRunLearnCurrentNoError(t, cont, opts)

	content, err := os.ReadFile(filepath.Join(snapshotDir, "main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(content))
	require.NoFileExists(t, filepath.Join(snapshotDir, "internal", "deleted.go"))
	staleContent, err := os.ReadFile(filepath.Join(snapshotDir, "stale.go"))
	require.NoError(t, err)
	require.Equal(t, "stale snapshot\n", string(staleContent))
}

func TestRunLearnWorkspaceCurrentPrintsProjectTokenUsageAfterProjectLogs(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
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
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	profileSavedIndex := strings.LastIndex(output, "项目画像同步已跳过")
	tokenIndex := strings.LastIndex(output, "Token 消耗: 子项目 backend")
	require.NotEqual(t, -1, profileSavedIndex)
	require.NotEqual(t, -1, tokenIndex)
	require.Greater(t, tokenIndex, profileSavedIndex)

	rootRecords, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	require.Empty(t, rootRecords)

	childRepo, err := boltdb.NewPatternRepository(filepath.Join(childRoot, ".skills-seed", "store", "project.db"))
	require.NoError(t, err)
	defer func() { require.NoError(t, childRepo.Close()) }()
	childRecords, err := childRepo.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	require.Len(t, childRecords, 1)
}

func TestRunLearnCurrentReportsWorkspaceChangedProjectsWhenPatternsSaved(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	result, err := runLearnCurrent(cont, opts)

	require.NoError(t, err)
	require.Equal(t, 1, result.Summary.ChangedProjects)
}

func TestRunLearnCurrentSkipsAIWhenFilesUnchanged(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	requireRunLearnCurrentNoError(t, cont, opts)

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
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Zero(t, analyzeCalls)
	require.Zero(t, profileCalls)
	require.Contains(t, output, "未检测到可学习文件变化")
}

func TestRunLearnCurrentRefreshesMissingProfileWhenFilesUnchanged(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	requireRunLearnCurrentNoError(t, cont, opts)
	require.NoError(t, os.Remove(filepath.Join(cont.SeedPath, "store", "documents", "project-profile.json")))

	analyzeCalls := 0
	profileCalls := 0
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		analyzeCalls++
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeProjectFn = func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
		profileCalls++
		return &agent.AnalyzeProjectResult{Language: "go", Summary: "rebuilt profile"}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)

	require.Zero(t, analyzeCalls)
	require.Equal(t, 1, profileCalls)
	profile, err := cont.ProfileRepo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "rebuilt profile", profile.Summary)
}

func TestRunLearnCurrentAutoMergesProfileDeltaWithoutFullRefresh(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.SelectRelevantFilesMinCandidates = 1
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	requireRunLearnCurrentNoError(t, cont, opts)

	writeLearnFile(t, cont.ConfigRepo.GetProjectConfig().RootPath, "main.go", "package main\nconst changed = true\n")
	gitAddAll(t, cont.ConfigRepo.GetProjectConfig().RootPath)

	var patternFocus []string
	profileCalls := 0
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		patternFocus = append([]string{}, req.FocusPaths...)
		pattern := domain.NewPattern("p2", "Changed File Pattern", domain.CategoryStructure)
		return &agent.AnalyzeCurrentCodebaseResult{
			Patterns: []domain.Pattern{*pattern},
			ProfileDelta: domain.ProjectProfileDelta{
				Summary: "merged profile delta",
				KeyModules: []domain.ModuleInfo{
					{Name: "main", Path: "main.go", Description: "changed entry"},
				},
			},
		}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeProjectFn = func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
		profileCalls++
		return &agent.AnalyzeProjectResult{Language: "go", Summary: "profile"}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)

	require.Equal(t, []string{"main.go"}, patternFocus)
	require.Zero(t, profileCalls)
	profile, err := cont.ProfileRepo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "profile", profile.Summary)
	require.Len(t, profile.KeyModules, 1)
	require.Equal(t, "main.go", profile.KeyModules[0].Path)
}

func TestRunLearnCurrentAutoSkipsExistingProfileWhenNoPatternsSaved(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.SelectRelevantFilesMinCandidates = 1
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	requireRunLearnCurrentNoError(t, cont, opts)

	writeLearnFile(t, cont.ConfigRepo.GetProjectConfig().RootPath, "main.go", "package main\nconst changed = true\n")
	gitAddAll(t, cont.ConfigRepo.GetProjectConfig().RootPath)

	profileCalls := 0
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		return &agent.AnalyzeCurrentCodebaseResult{
			Patterns: nil,
		}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeProjectFn = func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
		profileCalls++
		return &agent.AnalyzeProjectResult{Language: "go", Summary: "profile"}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)

	require.Zero(t, profileCalls)
}

func TestRunLearnCurrentRefreshProfileUsesChangedFilesAsFocusPaths(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileRefresh)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.SelectRelevantFilesMinCandidates = 1
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	requireRunLearnCurrentNoError(t, cont, opts)

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

	requireRunLearnCurrentNoError(t, cont, opts)

	require.Equal(t, []string{"main.go"}, patternFocus)
	require.Equal(t, []string{"main.go"}, profileFocus)
}

func TestRunLearnCurrentAIFileSelectorNarrowsAnalysisFiles(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	requireRunLearnCurrentNoError(t, cont, opts)

	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.SelectRelevantFilesMinCandidates = 1
	require.NoError(t, cont.ConfigRepo.Update(cfg))

	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/logic/create.go", "package logic\nconst selected = true\n")
	writeLearnFile(t, projectRoot, "internal/types/types.go", "package types\nconst skipped = true\n")
	gitAddAll(t, projectRoot)

	var selectorReq *agent.SelectFilesRequest
	var received agent.AnalyzeCurrentCodebaseRequest
	var savedRecords []domain.FileAnalysisRecord
	originalTracker := cont.FileTracker
	cont.FileTracker = &mocks.MockFileAnalysisTracker{
		ListAnalyzedFilesFn: originalTracker.ListAnalyzedFiles,
		SaveAnalyzedFilesFn: func(ctx context.Context, records []domain.FileAnalysisRecord) error {
			savedRecords = append([]domain.FileAnalysisRecord(nil), records...)
			return originalTracker.SaveAnalyzedFiles(ctx, records)
		},
		DeleteAnalyzedFilesFn: originalTracker.DeleteAnalyzedFiles,
	}
	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.SelectFilesFn = func(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
		selectorReq = req
		return &agent.SelectFilesResult{
			SelectedPaths: []string{"internal/logic/create.go"},
			Exclude:       []string{"internal/types/**"},
			Reason:        "prefer high-signal implementation files",
		}, nil
	}
	mockAgent.AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		received = *req
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.NotNil(t, selectorReq)
	require.Equal(t, 2, selectorReq.CandidateNum)
	require.Equal(t, []string{"internal/logic/create.go"}, received.FocusPaths)
	require.Len(t, received.SampleFiles, 1)
	require.Equal(t, "internal/logic/create.go", received.SampleFiles[0].Path)
	require.Len(t, savedRecords, 2)
	require.ElementsMatch(t, []string{"internal/logic/create.go", "internal/types/types.go"}, fileAnalysisRecordPaths(savedRecords))
	savedByPath := fileAnalysisRecordByPath(savedRecords)
	require.Equal(t, domain.FileAnalysisStatusAnalyzed, savedByPath["internal/logic/create.go"].AnalysisStatus)
	require.Equal(t, domain.FileAnalysisStatusAISkipped, savedByPath["internal/types/types.go"].AnalysisStatus)
	require.Equal(t, "prefer high-signal implementation files", savedByPath["internal/types/types.go"].SelectionReason)
	require.Contains(t, output, "文件筛选结果:")
	require.Contains(t, output, "AI 筛选: 输入 2，保留 1")
	require.Contains(t, output, "状态: AI 文件筛选已生效")
	require.Contains(t, output, "最终待分析: 1")
	require.Contains(t, output, "AI 文件筛选")
	require.NotContains(t, output, "文件指纹提交计划")
}

func TestRunLearnCurrentAIFileSelectorCommitsSkippedFileFingerprintsAfterSuccess(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.SelectRelevantFilesMinCandidates = 1
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	requireRunLearnCurrentNoError(t, cont, opts)

	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/logic/create.go", "package logic\nconst selected = true\n")
	writeLearnFile(t, projectRoot, "internal/types/types.go", "package types\nconst skipped = true\n")
	gitAddAll(t, projectRoot)

	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.SelectFilesFn = func(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
		return &agent.SelectFilesResult{
			SelectedPaths: []string{"internal/logic/create.go"},
			Exclude:       []string{"internal/types/**"},
			Reason:        "prefer high-signal implementation files",
		}, nil
	}
	requireRunLearnCurrentNoError(t, cont, opts)

	analyzeCalls := 0
	selectCalls := 0
	mockAgent.SelectFilesFn = func(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
		selectCalls++
		return &agent.SelectFilesResult{}, nil
	}
	mockAgent.AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		analyzeCalls++
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Zero(t, selectCalls)
	require.Zero(t, analyzeCalls)
	require.Contains(t, output, "未检测到可学习文件变化")
}

func TestRunLearnCurrentAIFileSelectorCanBeDisabled(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.SelectRelevantFiles = false
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	requireRunLearnCurrentNoError(t, cont, opts)

	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/logic/create.go", "package logic\nconst selected = true\n")
	writeLearnFile(t, projectRoot, "internal/types/types.go", "package types\nconst skipped = true\n")
	gitAddAll(t, projectRoot)

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.SelectFilesFn = func(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
		t.Fatal("SelectFiles should not be called when learning.current.select_relevant_files is false")
		return nil, nil
	}
	mockAgent.AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		received = *req
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)

	require.ElementsMatch(t, []string{"internal/logic/create.go", "internal/types/types.go"}, received.FocusPaths)
	require.Len(t, received.SampleFiles, 2)
}

func TestRunLearnCurrentAIFileSelectorSkipsBelowCandidateThreshold(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.SelectRelevantFiles = true
	cfg.Learning.Current.SelectRelevantFilesMinCandidates = 10
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	requireRunLearnCurrentNoError(t, cont, opts)

	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/logic/create.go", "package logic\nconst selected = true\n")
	writeLearnFile(t, projectRoot, "internal/types/types.go", "package types\nconst skipped = true\n")
	gitAddAll(t, projectRoot)

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.SelectFilesFn = func(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
		t.Fatal("SelectFiles should not be called below learning.current.select_relevant_files_min_candidates")
		return nil, nil
	}
	mockAgent.AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		received = *req
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.ElementsMatch(t, []string{"internal/logic/create.go", "internal/types/types.go"}, received.FocusPaths)
	require.Len(t, received.SampleFiles, 2)
	require.Contains(t, output, "文件筛选结果:")
	require.Contains(t, output, "AI 筛选: 输入 -，保留 -")
	require.Contains(t, output, "状态: 已跳过（候选 2 个，未达到阈值 10）")
	require.Contains(t, output, "最终待分析: 2")
	require.Contains(t, output, "跳过 AI 文件筛选（候选 2 个，未达到阈值 10）")
}

func TestRunLearnCurrentDoesNotCommitFileFingerprintWhenPatternStoreFails(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cont.PatternRepo.Close()
	cont.FileTracker = &mocks.MockFileAnalysisTracker{
		ListAnalyzedFilesFn: func(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
			return []domain.FileAnalysisRecord{}, nil
		},
		SaveAnalyzedFilesFn: func(ctx context.Context, records []domain.FileAnalysisRecord) error {
			t.Fatalf("file fingerprint should not be committed when pattern save fails: %#v", records)
			return nil
		},
		DeleteAnalyzedFilesFn: func(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
			return nil
		},
	}

	_, err := runLearnCurrent(cont, opts)

	require.Error(t, err)
	require.Contains(t, err.Error(), "patterns")
}

func TestRunLearnCurrentResumesPendingUnitFromCachedPlan(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\n")
	writeLearnFile(t, projectRoot, "internal/key/create.go", "package key\n")
	gitAddAll(t, projectRoot)

	mockAgent := cont.Agent.(*mocks.MockAgent)
	planCalls := 0
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		planCalls++
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "auth", Name: "认证登录", EntryPaths: []string{"internal/auth/login.go"}},
			{ID: "key", Name: "密钥创建", EntryPaths: []string{"internal/key/create.go"}},
		}}, nil
	}
	var analyzed [][]string
	var labels []string
	var units []string
	mockAgent.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		labels = append(labels, req.RuntimeLabel)
		results := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, 1)
		for _, unit := range req.Units {
			analyzed = append(analyzed, append([]string{}, unit.FocusPaths...))
			units = append(units, unit.AnalysisUnit.ID)
			if unit.AnalysisUnit.ID == "auth" {
				pattern := domain.NewPattern("p-"+strings.ReplaceAll(unit.FocusPaths[0], "/", "-"), "Unit Pattern", domain.CategoryBusiness)
				results = append(results, agent.AnalyzeCurrentCodebaseUnitResult{
					UnitID:   unit.AnalysisUnit.ID,
					UnitName: unit.AnalysisUnit.Name,
					Patterns: []domain.Pattern{*pattern},
				})
			}
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Units: results}, nil
	}

	_, err := runLearnCurrent(cont, opts)
	require.Error(t, err)
	require.Equal(t, 1, planCalls)
	require.Equal(t, [][]string{{"internal/auth/login.go"}, {"internal/key/create.go"}}, analyzed)
	require.Equal(t, []string{"batch-001", "batch-002"}, labels)
	require.Equal(t, []string{"auth", "key"}, units)

	stateRepo := learnCurrentStateRepo(cont.SeedPath, commandStateLearnCurrent)
	stateBytes, err := os.ReadFile(stateRepo.Path())
	require.NoError(t, err)
	var cachedState commandstate.State
	require.NoError(t, json.Unmarshal(stateBytes, &cachedState))
	require.Equal(t, commandStateLearnCurrent, cachedState.Command)
	require.Equal(t, learnCurrentStateMode(string(config.LearningModeNormal), string(config.LearningScopeFlow)), cachedState.Mode)
	require.Len(t, cachedState.Units, 2)

	analyzed = nil
	labels = nil
	units = nil
	mockAgent.SelectFilesFn = func(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
		t.Fatal("SelectFiles should not be called when resuming cached command state")
		return nil, nil
	}
	mockAgent.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		labels = append(labels, req.RuntimeLabel)
		results := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, len(req.Units))
		for _, unit := range req.Units {
			analyzed = append(analyzed, append([]string{}, unit.FocusPaths...))
			units = append(units, unit.AnalysisUnit.ID)
			pattern := domain.NewPattern("p-resumed", "Resumed Pattern", domain.CategoryBusiness)
			results = append(results, agent.AnalyzeCurrentCodebaseUnitResult{
				UnitID:   unit.AnalysisUnit.ID,
				UnitName: unit.AnalysisUnit.Name,
				Patterns: []domain.Pattern{*pattern},
			})
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Units: results}, nil
	}

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})
	require.Equal(t, 1, planCalls, "cached state should be reused without replanning")
	require.Equal(t, [][]string{{"internal/key/create.go"}}, analyzed)
	require.Equal(t, []string{"batch-001"}, labels)
	require.Equal(t, []string{"key"}, units)
	require.Contains(t, output, "恢复未完成的 learn-current 执行计划")
	require.Contains(t, output, "本地候选: 可学习 3，待处理 3")
	require.Contains(t, output, "AI 筛选: 输入 -，保留 -")
	require.Contains(t, output, "恢复后: 待分析 2，分析单元 2")
	require.Contains(t, output, "分析当前代码库 · 单元 2/2 · 密钥创建")
	require.NotContains(t, output, "增量文件变化:")
	require.NotContains(t, output, "计划输入文件:")
	require.NotContains(t, output, "文件筛选结果:")
	require.NoFileExists(t, stateRepo.Path())
}

func TestRunLearnCurrentCommitsSnapshotsOnlyForSuccessfulUnits(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\nconst login = false\n")
	writeLearnFile(t, projectRoot, "internal/key/create.go", "package key\nconst create = false\n")
	gitAddAll(t, projectRoot)
	requireRunLearnCurrentNoError(t, cont, opts)

	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\nconst login = true\n")
	writeLearnFile(t, projectRoot, "internal/key/create.go", "package key\nconst create = true\n")
	gitAddAll(t, projectRoot)

	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "auth", Name: "认证登录", EntryPaths: []string{"internal/auth/login.go"}},
			{ID: "key", Name: "密钥创建", EntryPaths: []string{"internal/key/create.go"}},
		}}, nil
	}
	receivedDiffs := map[string][]string{}
	mockAgent.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		var results []agent.AnalyzeCurrentCodebaseUnitResult
		for _, unit := range req.Units {
			for _, diff := range unit.DiffFiles {
				receivedDiffs[unit.AnalysisUnit.ID] = append(receivedDiffs[unit.AnalysisUnit.ID], diff.Path)
			}
			if unit.AnalysisUnit.ID == "auth" {
				pattern := domain.NewPattern("p-"+unit.AnalysisUnit.ID, "Unit Pattern", domain.CategoryBusiness)
				results = append(results, agent.AnalyzeCurrentCodebaseUnitResult{
					UnitID:   unit.AnalysisUnit.ID,
					UnitName: unit.AnalysisUnit.Name,
					Patterns: []domain.Pattern{*pattern},
				})
			}
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Units: results}, nil
	}

	_, err := runLearnCurrent(cont, opts)

	require.Error(t, err)
	require.Equal(t, []string{"internal/auth/login.go"}, receivedDiffs["auth"])
	require.Equal(t, []string{"internal/key/create.go"}, receivedDiffs["key"])
	snapshotDir := filepath.Join(cont.SeedPath, "cache", "snapshots")
	authSnapshot, err := os.ReadFile(filepath.Join(snapshotDir, "internal", "auth", "login.go"))
	require.NoError(t, err)
	require.Equal(t, "package auth\nconst login = true\n", string(authSnapshot))
	keySnapshot, err := os.ReadFile(filepath.Join(snapshotDir, "internal", "key", "create.go"))
	require.NoError(t, err)
	require.Equal(t, "package key\nconst create = false\n", string(keySnapshot))
}

func TestRunLearnCurrentReplansWhenLearningModeChanges(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\n")
	writeLearnFile(t, projectRoot, "internal/key/create.go", "package key\n")
	gitAddAll(t, projectRoot)

	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.Mode = config.LearningModeDeep
	require.NoError(t, cont.ConfigRepo.Update(cfg))

	mockAgent := cont.Agent.(*mocks.MockAgent)
	planModes := []config.LearningMode{}
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		planModes = append(planModes, req.LearningMode)
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "auth", Name: "认证登录", EntryPaths: []string{"internal/auth/login.go"}},
			{ID: "key", Name: "密钥创建", EntryPaths: []string{"internal/key/create.go"}},
		}}, nil
	}
	analyzeModes := []config.LearningMode{}
	mockAgent.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		analyzeModes = append(analyzeModes, req.LearningMode)
		results := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, 1)
		for _, unit := range req.Units {
			if unit.AnalysisUnit.ID != "auth" {
				continue
			}
			pattern := domain.NewPattern("p-"+unit.AnalysisUnit.ID, "Unit Pattern", domain.CategoryBusiness)
			results = append(results, agent.AnalyzeCurrentCodebaseUnitResult{
				UnitID:   unit.AnalysisUnit.ID,
				UnitName: unit.AnalysisUnit.Name,
				Patterns: []domain.Pattern{*pattern},
			})
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Units: results}, nil
	}

	_, err := runLearnCurrent(cont, opts)
	require.Error(t, err)
	require.Equal(t, []config.LearningMode{config.LearningModeDeep}, planModes)
	require.Equal(t, []config.LearningMode{config.LearningModeDeep, config.LearningModeDeep}, analyzeModes)

	cfg = cont.ConfigRepo.Get()
	cfg.Learning.Current.Mode = config.LearningModeFast
	require.NoError(t, cont.ConfigRepo.Update(cfg))

	analyzeModes = nil
	mockAgent.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		analyzeModes = append(analyzeModes, req.LearningMode)
		results := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, len(req.Units))
		for _, unit := range req.Units {
			pattern := domain.NewPattern("p-"+unit.AnalysisUnit.ID, "Unit Pattern", domain.CategoryBusiness)
			results = append(results, agent.AnalyzeCurrentCodebaseUnitResult{
				UnitID:   unit.AnalysisUnit.ID,
				UnitName: unit.AnalysisUnit.Name,
				Patterns: []domain.Pattern{*pattern},
			})
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Units: results}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)
	require.Equal(t, []config.LearningMode{config.LearningModeDeep, config.LearningModeFast}, planModes)
	require.Equal(t, []config.LearningMode{config.LearningModeFast}, analyzeModes)
}

func TestRunLearnCurrentAnalyzesPlannedUnitsOnePerCallByDefault(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\n")
	writeLearnFile(t, projectRoot, "internal/key/create.go", "package key\n")
	gitAddAll(t, projectRoot)

	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "auth", Name: "认证登录", EntryPaths: []string{"internal/auth/login.go"}},
			{ID: "key", Name: "密钥创建", EntryPaths: []string{"internal/key/create.go"}},
		}}, nil
	}
	var batches []agent.AnalyzeCurrentCodebaseBatchRequest
	mockAgent.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		batches = append(batches, *req)
		results := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, len(req.Units))
		for _, unit := range req.Units {
			pattern := domain.NewPattern("p-"+unit.AnalysisUnit.ID, "Unit Pattern", domain.CategoryBusiness)
			results = append(results, agent.AnalyzeCurrentCodebaseUnitResult{
				UnitID:   unit.AnalysisUnit.ID,
				UnitName: unit.AnalysisUnit.Name,
				Patterns: []domain.Pattern{*pattern},
			})
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Units: results}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)

	require.Len(t, batches, 2)
	require.Equal(t, "batch-001", batches[0].RuntimeLabel)
	require.Len(t, batches[0].Units, 1)
	require.Equal(t, "auth", batches[0].Units[0].AnalysisUnit.ID)
	require.Equal(t, []string{"internal/auth/login.go"}, batches[0].Units[0].FocusPaths)
	require.Equal(t, "batch-002", batches[1].RuntimeLabel)
	require.Len(t, batches[1].Units, 1)
	require.Equal(t, "key", batches[1].Units[0].AnalysisUnit.ID)
	require.Equal(t, []string{"internal/key/create.go"}, batches[1].Units[0].FocusPaths)
}

func TestRunLearnCurrentAnalyzesPlannedUnitsInBatchWhenConfigured(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Learning.Current.MaxUnitsPerCall = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\n")
	writeLearnFile(t, projectRoot, "internal/key/create.go", "package key\n")
	gitAddAll(t, projectRoot)

	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "auth", Name: "认证登录", EntryPaths: []string{"internal/auth/login.go"}},
			{ID: "key", Name: "密钥创建", EntryPaths: []string{"internal/key/create.go"}},
		}}, nil
	}
	var batches []agent.AnalyzeCurrentCodebaseBatchRequest
	mockAgent.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		batches = append(batches, *req)
		results := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, len(req.Units))
		for _, unit := range req.Units {
			pattern := domain.NewPattern("p-"+unit.AnalysisUnit.ID, "Unit Pattern", domain.CategoryBusiness)
			results = append(results, agent.AnalyzeCurrentCodebaseUnitResult{
				UnitID:   unit.AnalysisUnit.ID,
				UnitName: unit.AnalysisUnit.Name,
				Patterns: []domain.Pattern{*pattern},
			})
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Units: results}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)

	require.Len(t, batches, 1)
	require.Equal(t, "batch-001", batches[0].RuntimeLabel)
	require.Len(t, batches[0].Units, 2)
	require.Equal(t, "auth", batches[0].Units[0].AnalysisUnit.ID)
	require.Equal(t, "key", batches[0].Units[1].AnalysisUnit.ID)
}

func TestRunLearnCurrentShowsAnalysisUnitProgressDetails(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\n")
	writeLearnFile(t, projectRoot, "internal/key/create.go", "package key\n")
	gitAddAll(t, projectRoot)

	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "auth", Name: "认证登录", EntryPaths: []string{"internal/auth/login.go"}},
			{ID: "key", Name: "密钥创建", EntryPaths: []string{"internal/key/create.go"}},
		}}, nil
	}

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Contains(t, output, "分析单元规划")
	require.Contains(t, output, "分析当前代码库 · 单元 1/2 · 认证登录")
	require.Contains(t, output, "分析当前代码库 · 提交指纹 2/2 · 密钥创建")
}

func TestRunLearnCurrentIncludesAnalysisUnitInFailure(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/auth/login.go", "package auth\n")
	gitAddAll(t, projectRoot)

	mockAgent := cont.Agent.(*mocks.MockAgent)
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "auth", Name: "认证登录", EntryPaths: []string{"internal/auth/login.go"}},
		}}, nil
	}
	mockAgent.AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		return nil, assertAnError("解析结果失败")
	}

	var runErr error
	output := captureLearnStdout(t, func() {
		_, runErr = runLearnCurrent(cont, opts)
	})

	require.Error(t, runErr)
	require.Contains(t, runErr.Error(), "分析当前代码库 · 单元 1/1 · 认证登录")
	require.Contains(t, runErr.Error(), "解析结果失败")
	require.Contains(t, output, "分析当前代码库 · 单元 1/1 · 认证登录")
}

func TestRunLearnCurrentSendsDeletedFilesAsDiffs(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, projectRoot, "internal/deleted.go", "package internal\n")
	gitAddAll(t, projectRoot)
	requireRunLearnCurrentNoError(t, cont, opts)

	require.NoError(t, os.Remove(filepath.Join(projectRoot, "internal", "deleted.go")))
	gitAddAll(t, projectRoot)

	var received agent.AnalyzeCurrentCodebaseRequest
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		received = *req
		return &agent.AnalyzeCurrentCodebaseResult{}, nil
	}

	requireRunLearnCurrentNoError(t, cont, opts)

	require.Equal(t, []string{"internal/deleted.go"}, received.FocusPaths)
	require.Len(t, received.DiffFiles, 1)
	require.Equal(t, "internal/deleted.go", received.DiffFiles[0].Path)
	diffContent := readLearnFilePath(t, received.DiffFiles[0].DiffPath)
	require.Contains(t, diffContent, "-package internal")
}

func TestRunLearnCurrentWithContextPassesUserContextToAnalysis(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()

	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})

	var receivedContext string
	cont.Agent.(*mocks.MockAgent).AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		receivedContext = req.UserContext
		pattern := domain.NewPattern("p-context", "Context Boundary", domain.CategoryBusiness)
		pattern.Confidence = 0.9
		return &agent.AnalyzeCurrentCodebaseResult{Patterns: []domain.Pattern{*pattern}}, nil
	}

	_, err := RunLearnCurrentWithContext(cont, "私有化部署，不是 SaaS")
	require.NoError(t, err)

	require.Equal(t, "私有化部署，不是 SaaS", receivedContext)
}

func TestRunLearnWorkspaceCurrentDelegatesIncrementalSkipToChildProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	requireRunLearnCurrentNoError(t, cont, opts)

	atomic.StoreInt32(&learnWorkspaceMockAnalyzeCalls, 0)

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Zero(t, atomic.LoadInt32(&learnWorkspaceMockAnalyzeCalls))
	require.Contains(t, output, "未检测到可学习文件变化")
}

func TestRunLearnWorkspaceCurrentAnalyzesAndSavesWorkspaceArtifacts(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	registerLearnWorkspaceMockAgentFactoryWithHandlers(t, learnWorkspaceMockHandlers{
		analyzeProject: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return &agent.AnalyzeProjectResult{
				Language:   "go",
				Frameworks: []string{"Gin"},
				Summary:    "backend 是私有化部署主后端。",
				KeyModules: []domain.ModuleInfo{
					{Path: "internal/api", Description: "管理 API 入口"},
				},
			}, nil
		},
	})
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")
	require.DirExists(t, childRoot)

	var profileReq *agent.AnalyzeWorkspaceProfileRequest
	var specReq *agent.AnalyzeWorkspaceSpecRequest
	cont.Agent.(*mocks.MockAgent).AnalyzeWorkspaceProfileFn = func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
		profileReq = req
		workspaceInput := readLearnFilePath(t, req.WorkspaceInputPath)
		userContext := readLearnFilePath(t, req.UserContextPath)
		require.Contains(t, workspaceInput, `"project_profile_path"`)
		require.Contains(t, workspaceInput, "backend 是私有化部署主后端。")
		require.Contains(t, workspaceInput, "internal/api: 管理 API 入口")
		require.Contains(t, userContext, "工作区用于离线交付")
		return &domain.WorkspaceProfile{
			Name:     req.WorkspaceName,
			RootPath: req.WorkspaceRoot,
			Summary:  "学习阶段分析：工作区用于离线交付。",
			Projects: []domain.WorkspaceProject{
				{ID: "backend", Path: "backend", Type: "backend", Language: "go", Responsibility: "负责管理 API", Frameworks: []string{"Gin"}},
			},
			Shared: []domain.WorkspacePath{
				{Path: "shared", Description: "离线交付共享配置", Consumers: []string{"backend"}},
			},
		}, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeWorkspaceSpecFn = func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
		specReq = req
		profileInput := readLearnFilePath(t, req.WorkspaceProfilePath)
		userContext := readLearnFilePath(t, req.UserContextPath)
		require.Contains(t, profileInput, "学习阶段分析：工作区用于离线交付。")
		require.Contains(t, userContext, "工作区用于离线交付")
		return &domain.WorkspaceSpec{
			Name:     req.WorkspaceName,
			RootPath: req.WorkspaceRoot,
			Rules: []domain.WorkspaceRule{
				{Title: "离线交付边界", Description: "变更 backend 时必须保留离线安装包验证。", AppliesTo: []string{"backend"}},
			},
		}, nil
	}

	requireRunLearnCurrentNoError(t, cont, learnCurrentOptionsForTestWithContext("工作区用于离线交付"))

	require.NotNil(t, profileReq)
	require.NotNil(t, specReq)
	require.NotEmpty(t, profileReq.UserContextPath)
	require.Equal(t, profileReq.UserContextPath, specReq.UserContextPath)
	require.NoFileExists(t, profileReq.WorkspaceInputPath)
	require.NoFileExists(t, profileReq.UserContextPath)
	require.NoFileExists(t, specReq.WorkspaceProfilePath)

	profile, err := cont.WorkspaceProfileRepo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "学习阶段分析：工作区用于离线交付。", profile.Summary)
	require.Equal(t, "负责管理 API", profile.Projects[0].Responsibility)
	require.Equal(t, []string{"Gin"}, profile.Projects[0].Frameworks)
	require.Equal(t, "shared", profile.Shared[0].Path)

	spec, err := cont.WorkspaceSpecRepo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "离线交付边界", spec.Rules[0].Title)
	require.Contains(t, spec.Rules[0].Description, "离线安装包验证")
}

func TestRunLearnWorkspaceCurrentShowsRootAnalysisProgress(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip))
	})

	profileIndex := strings.Index(output, "分析工作区画像")
	specIndex := strings.Index(output, "分析工作区规范")
	saveIndex := strings.Index(output, "保存工作区关系")
	completeIndex := strings.LastIndex(output, "工作区增量学习完成")
	require.NotEqual(t, -1, profileIndex, "expected workspace profile analysis progress in output: %q", output)
	require.NotEqual(t, -1, specIndex, "expected workspace spec analysis progress in output: %q", output)
	require.NotEqual(t, -1, saveIndex, "expected workspace relationship save progress in output: %q", output)
	require.NotEqual(t, -1, completeIndex, "expected workspace completion in output: %q", output)
	require.Less(t, profileIndex, specIndex)
	require.Less(t, specIndex, saveIndex)
	require.Less(t, saveIndex, completeIndex)
}

func TestRunLearnWorkspaceCurrentSkipsWorkspaceArtifactsWhenInputUnchanged(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	requireRunLearnCurrentNoError(t, cont, opts)
	cont.Agent.(*mocks.MockAgent).AnalyzeWorkspaceProfileFn = func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
		t.Fatal("workspace profile analysis should be skipped when input fingerprint is unchanged")
		return nil, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeWorkspaceSpecFn = func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
		t.Fatal("workspace spec analysis should be skipped when input fingerprint is unchanged")
		return nil, nil
	}

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Contains(t, output, "工作区关系输入未变化，已跳过工作区画像和规范分析")
}

func TestRunLearnWorkspaceCurrentSkipsWorkspaceArtifactsWhenChildrenUnchangedWithLegacyFingerprint(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	requireRunLearnCurrentNoError(t, cont, opts)
	ctx := context.Background()
	require.NoError(t, cont.FileTracker.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{
		{
			ProjectID:      workspaceRelationshipFingerprintScope().ProjectID,
			ScopePath:      workspaceRelationshipFingerprintScope().ScopePath,
			Path:           "workspace-relationships.json",
			Hash:           "legacy-fingerprint",
			HashAlgorithm:  domain.FileAnalysisHashMD5,
			Source:         domain.FileAnalysisSourceInputDigest,
			AnalysisStatus: domain.FileAnalysisStatusInputDigest,
			LastAnalyzedAt: time.Now().Format(time.RFC3339),
		},
	}))
	cont.Agent.(*mocks.MockAgent).AnalyzeWorkspaceProfileFn = func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
		t.Fatal("workspace profile analysis should be skipped when children are unchanged")
		return nil, nil
	}
	cont.Agent.(*mocks.MockAgent).AnalyzeWorkspaceSpecFn = func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
		t.Fatal("workspace spec analysis should be skipped when children are unchanged")
		return nil, nil
	}

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Contains(t, output, "工作区关系输入未变化，已跳过工作区画像和规范分析")
	record, err := cont.FileTracker.GetAnalyzedFile(ctx, workspaceRelationshipFingerprintScope(), "workspace-relationships.json")
	require.NoError(t, err)
	require.NotNil(t, record)
	require.NotEqual(t, "legacy-fingerprint", record.Hash)
}

func TestRunLearnWorkspaceCurrentWritesChildDetailsToChildLog(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	workspaceLogDir := filepath.Join(cont.SeedPath, "runtime/logs")
	require.NoError(t, logger.InitWithRetention(workspaceLogDir, "learn", logger.INFO, 0))
	workspaceLogPath := logger.CurrentLogPath()
	defer logger.Close()

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	childLogs := readLearnLogFiles(t, filepath.Join(childRoot, ".skills-seed", "runtime/logs"))
	require.NotEmpty(t, childLogs)
	require.Contains(t, strings.Join(childLogs, "\n"), `"operation":"command.learn_current"`)

	require.NoError(t, logger.Close())
	workspaceLog, err := os.ReadFile(workspaceLogPath)
	require.NoError(t, err)
	require.NotContains(t, string(workspaceLog), `"operation":"command.learn_current"`)
	require.Contains(t, string(workspaceLog), "子项目 backend 开始增量学习")
	require.Contains(t, string(workspaceLog), "子项目 backend 独立执行完成")
	require.Contains(t, output, "子项目 backend 开始增量学习")
	require.Contains(t, output, filepath.Join(childRoot, ".skills-seed", "runtime/logs"))
}

func TestRunLearnWorkspaceCurrentUsesConfiguredParallelism(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
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
		return &agent.AnalyzeCurrentCodebaseResult{Patterns: []domain.Pattern{*pattern}}, nil
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

	requireRunLearnCurrentNoError(t, cont, opts)

	require.GreaterOrEqual(t, atomic.LoadInt32(&maxActive), int32(2))
}

func TestRunLearnWorkspaceCurrentSuppressesChildNextSteps(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	initLearnWorkspaceChildProject(t, cont.ConfigRepo.GetProjectConfig().RootPath, project, "package main\n")

	output := captureLearnStdout(t, func() {
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.NotContains(t, output, "后续可执行:")
	require.NotContains(t, output, "查看模式: skills-seed view patterns")
	require.NotContains(t, output, "查看模式: skills-seed patterns show")
}

func TestRunLearnWorkspaceCurrentParallelModeShowsPerChildProgressWithoutDetailedLogs(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
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
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "frontend")
	require.Contains(t, output, "准备项目上下文")
	require.Contains(t, output, "本地文件过滤")
	require.Contains(t, output, "分析单元规划")
	require.Contains(t, output, "分析当前代码库 · 单元 1/1 · 当前代码变更")
	require.NotContains(t, output, "项目根路径:")
	require.NotContains(t, output, "本地过滤统计:")
	require.NotContains(t, output, "后续可执行:")
	require.NotContains(t, output, "开始增量学习")
	require.NotContains(t, output, "未检测到可学习文件变化")
	require.NotContains(t, output, "独立执行完成")
}

func TestRunLearnWorkspaceCurrentShowsRetryReasonInChildProgressLine(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	restorePause := setWorkspaceChildStepPauseForTest(func(time.Duration) {})
	defer restorePause()

	provider := registerLearnWorkspaceMockAgentFactoryWithAnalyze(t, func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		agent.ReportRetryForContext(ctx, agent.RetryInfo{
			AgentName:    "claude",
			Operation:    "AnalyzeCurrentCodebase",
			Attempt:      1,
			MaxRetries:   3,
			WaitDuration: 15 * time.Second,
			CallDuration: 217 * time.Second,
			Reason:       "API Error: 529 overloaded_error",
		})
		agent.ReportRetryAttemptForContext(ctx, agent.RetryInfo{
			AgentName:  "claude",
			Operation:  "AnalyzeCurrentCodebase",
			Attempt:    2,
			MaxRetries: 3,
		})
		agent.ReportRetryRecoveredForContext(ctx, agent.RetryInfo{
			AgentName:  "claude",
			Operation:  "AnalyzeCurrentCodebase",
			Attempt:    2,
			MaxRetries: 3,
		})
		pattern := domain.NewPattern("p1", "Error Handling", domain.CategoryError)
		pattern.Confidence = 0.9
		return &agent.AnalyzeCurrentCodebaseResult{
			Patterns: []domain.Pattern{*pattern},
		}, nil
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
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	retryLabel := "分析当前代码库 · 单元 1/1 · 当前代码变更（API Error: 529 overloaded_error，本次调用 3m37s，15s 后重试）"
	attemptLabel := "分析当前代码库 · 单元 1/1 · 当前代码变更（第2次尝试）"
	require.Contains(t, output, retryLabel)
	require.Contains(t, output, attemptLabel)
	afterRetry := output[strings.Index(output, retryLabel)+len(retryLabel):]
	attemptIndex := strings.Index(afterRetry, attemptLabel)
	require.NotEqual(t, -1, attemptIndex, "expected retry attempt progress label after retry wait, got %q", output)
	afterAttempt := afterRetry[attemptIndex+len(attemptLabel):]
	restoreIndex := strings.Index(afterAttempt, "5/7 分析当前代码库 · 单元 1/1 · 当前代码变更\n")
	require.NotEqual(t, -1, restoreIndex, "expected retry progress label to be restored after a successful retry, got %q", output)
}

func TestRunLearnWorkspaceCurrentShowsPerChildProgressLines(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	restoreFactory := registerLearnWorkspaceMockAgentFactory(t)
	defer restoreFactory()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
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
		requireRunLearnCurrentNoError(t, cont, opts)
	})

	require.Contains(t, output, "backend")
	require.Contains(t, output, "front")
	require.Contains(t, output, "分析当前代码库")
	require.Contains(t, output, "backend")
	require.Contains(t, output, "7/7")
	require.Contains(t, output, "完成")
	require.Contains(t, output, "学习工作区子项目")
	require.NotContains(t, output, "0/2 | backend")
	require.NotContains(t, output, "0/2 | front")
	require.NotContains(t, output, "开始增量学习")
	require.NotContains(t, output, "未检测到可学习文件变化")
	require.NotContains(t, output, "独立执行完成")
	pauseMu.Lock()
	require.NotEmpty(t, pauseDurations)
	require.Equal(t, progress.FastStepPause, pauseDurations[0])
	pauseMu.Unlock()
}

func TestRunLearnWorkspaceCurrentMarksFailedChildProgress(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	tokenusage.Reset()
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto)
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
		_, err := runLearnCurrent(cont, opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "front")
		require.Contains(t, err.Error(), "profile overloaded")
	})

	require.Contains(t, output, "front        7/7 失败")
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

	_, err := runLearnCurrent(cont, learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto))

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

	_, err = runLearnCurrent(cont, learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto))

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

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "store", "project.db"))
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
	curatorSvc := curator.NewService(mockAgent, patternRepo)

	return &container.Container{
		SeedPath:             seedPath,
		Config:               configRepo.Get(),
		ConfigRepo:           configRepo,
		GitRepo:              gitRepo,
		PatternRepo:          patternRepo,
		ProfileRepo:          profilestore.NewRepository(seedPath),
		StateRepo:            statestore.NewRepository(seedPath),
		WorkspaceProfileRepo: workspacestore.NewProfileRepository(seedPath),
		WorkspaceSpecRepo:    workspacestore.NewSpecRepository(seedPath),
		Agent:                mockAgent,
		AnalyzerSvc:          analyzer.NewAnalyzerService(mockAgent, configRepo),
		LearnerSvc:           servicelearner.NewLearnerService(mockAgent, gitRepo, patternRepo, patternRepo, curatorSvc),
		FileTracker:          patternRepo,
		CuratorSvc:           curatorSvc,
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
	restoreFactory := container.RegisterAgentFactoryForTest(provider, func(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) agent.Agent {
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
	t.Cleanup(restoreFactory)
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
	cfg.Learning.Current.Structural.Enabled = false
	require.NoError(t, childConfigRepo.Update(cfg))
	return childRoot
}

func learnCurrentOptionsForTest(testLanguage string, testFocusPaths []string, testProfileMode string) learnCurrentOptions {
	if testProfileMode == "" {
		testProfileMode = learnCurrentProfileAuto
	}
	return learnCurrentOptions{
		language:    testLanguage,
		focusPaths:  testFocusPaths,
		profileMode: testProfileMode,
	}
}

func learnCurrentOptionsForTestWithContext(userContext string) learnCurrentOptions {
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileAuto)
	opts.userContext = userContext
	return opts
}

func readLearnFilePath(t *testing.T, path string) string {
	t.Helper()
	require.NotEmpty(t, path)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func fileAnalysisRecordPaths(records []domain.FileAnalysisRecord) []string {
	paths := make([]string, 0, len(records))
	for _, record := range records {
		paths = append(paths, record.Path)
	}
	return paths
}

func fileAnalysisRecordByPath(records []domain.FileAnalysisRecord) map[string]domain.FileAnalysisRecord {
	byPath := make(map[string]domain.FileAnalysisRecord, len(records))
	for _, record := range records {
		byPath[record.Path] = record
	}
	return byPath
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
