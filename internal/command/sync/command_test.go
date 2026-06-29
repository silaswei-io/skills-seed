package sync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/git"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	servicelearner "github.com/silaswei-io/skills-seed/internal/service/learner"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestSyncLearnAfterLearnAlwaysGenerates(t *testing.T) {
	generateCalled := false

	err := syncLearnAfterLearn(domain.LearnCurrentResult{}, func() error {
		generateCalled = true
		return nil
	}, nil)

	require.NoError(t, err)
	require.True(t, generateCalled)
}

func TestSyncLearnAfterLearnWrapsGenerateError(t *testing.T) {
	errGenerate := errors.New("boom")

	err := syncLearnAfterLearn(domain.LearnCurrentResult{}, func() error {
		return errGenerate
	}, nil)

	require.ErrorIs(t, err, errGenerate)
}

func TestSyncLearnUsesSyncScopedCommandState(t *testing.T) {
	userContext := "sync context"
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = "demo"
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.RootPath = projectRoot
	cfg.Project.Language = "go"
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", "demo-dev")}
	require.NoError(t, configRepo.Update(cfg))
	require.NoError(t, exec.Command("git", "-C", projectRoot, "init", "-q").Run())
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "internal"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "internal", "key.go"), []byte("package internal\n"), 0644))
	require.NoError(t, exec.Command("git", "-C", projectRoot, "add", "-A").Run())

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "store", "project.db"))
	require.NoError(t, err)
	defer patternRepo.Close()
	mockAgent := &mocks.MockAgent{NameVal: "mock", AvailableVal: true}
	gitRepo := git.NewRepository(projectRoot)
	curatorSvc := curator.NewService(mockAgent, patternRepo)
	cont := &container.Container{
		SeedPath:    seedPath,
		Config:      configRepo.Get(),
		ConfigRepo:  configRepo,
		GitRepo:     gitRepo,
		PatternRepo: patternRepo,
		FileTracker: patternRepo,
		ProfileRepo: profilestore.NewRepository(seedPath),
		Agent:       mockAgent,
		AnalyzerSvc: analyzer.NewAnalyzerService(mockAgent, configRepo),
		LearnerSvc:  servicelearner.NewLearnerService(mockAgent, gitRepo, patternRepo, patternRepo, curatorSvc),
	}
	staleLearnState := commandstate.NewState("learn-current", "demo", "go", userContext, []commandstate.FileInput{
		{Path: "internal/stale.go", Hash: "stale", Status: "present"},
	}, []domain.AnalysisUnit{{ID: "stale", EntryPaths: []string{"internal/stale.go"}}})
	require.NoError(t, commandstate.NewRepository(seedPath, "learn-current").Save(context.Background(), staleLearnState))

	planCalls := 0
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		planCalls++
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{
			{ID: "key", Name: "Key", EntryPaths: []string{"internal/key.go"}},
		}}, nil
	}
	mockAgent.AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		return nil, errors.New("stop before state is cleared")
	}
	stateScope := commandutil.CommandStateScope("sync")

	err = syncLearn(context.Background(), cont, stateScope, userContext, nil)

	require.Error(t, err)
	require.Equal(t, 1, planCalls)
	syncStateBytes, readErr := os.ReadFile(commandstate.NewRepository(seedPath, stateScope).Path())
	require.NoError(t, readErr)
	var syncState commandstate.State
	require.NoError(t, json.Unmarshal(syncStateBytes, &syncState))
	require.Equal(t, "sync", syncState.Command)
	require.Equal(t, []domain.AnalysisUnit{{ID: "key", Name: "Key", EntryPaths: []string{"internal/key.go"}}}, syncState.Units)
	require.FileExists(t, commandstate.NewRepository(seedPath, "learn-current").Path())
}

func TestHasSyncCommandState(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := commandstate.NewRepository(seedPath, "sync")

	hasState, err := hasSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.False(t, hasState)

	require.NoError(t, repo.Save(context.Background(), commandstate.NewState("sync", "demo", "go", "", []commandstate.FileInput{
		{Path: "main.go", Hash: "hash", Status: "present"},
	}, []domain.AnalysisUnit{{ID: "main", EntryPaths: []string{"main.go"}}})))

	hasState, err = hasSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.True(t, hasState)
}

func TestSyncModeFromFlagsRejectsConflict(t *testing.T) {
	_, err := syncModeFromFlags(true, true)
	require.Error(t, err)
}

func TestSyncModeFromFlagsResolvesExplicitModes(t *testing.T) {
	mode, err := syncModeFromFlags(true, false)
	require.NoError(t, err)
	require.Equal(t, syncRunResume, mode)

	mode, err = syncModeFromFlags(false, true)
	require.NoError(t, err)
	require.Equal(t, syncRunRestart, mode)

	mode, err = syncModeFromFlags(false, false)
	require.NoError(t, err)
	require.Equal(t, syncRunAuto, mode)
}

func TestSyncRestartClearsOnlySyncCommandState(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	syncRepo := commandstate.NewRepository(seedPath, "sync")
	learnRepo := commandstate.NewRepository(seedPath, "learn-current")
	state := commandstate.NewState("sync", "demo", "go", "", []commandstate.FileInput{
		{Path: "main.go", Hash: "hash", Status: "present"},
	}, []domain.AnalysisUnit{{ID: "main", EntryPaths: []string{"main.go"}}})
	require.NoError(t, syncRepo.Save(context.Background(), state))
	require.NoError(t, learnRepo.Save(context.Background(), state))

	require.NoError(t, commandstate.NewRepository(seedPath, "sync").Clear())

	_, err := syncRepo.Load(context.Background())
	require.ErrorIs(t, err, commandstate.ErrStateNotFound)
	_, err = learnRepo.Load(context.Background())
	require.NoError(t, err)
}

func TestSyncWithUserPatternPassesContextOnlyToPatternDefinition(t *testing.T) {
	userContext := "私有化部署，不是 SaaS"
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = "demo"
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.RootPath = projectRoot
	cfg.Project.Language = "go"
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", "demo-dev")}
	require.NoError(t, configRepo.Update(cfg))

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "store", "project.db"))
	require.NoError(t, err)
	defer patternRepo.Close()

	var patternContext string
	pattern := domain.NewPattern("p1", "Context Boundary", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("description")
	pattern.SetRule("rule")
	mockAgent := &mocks.MockAgent{
		NameVal:      "mock",
		AvailableVal: true,
		UserDefinePatternFn: func(ctx context.Context, req *agent.UserDefinePatternRequest) (*agent.UserDefinePatternResult, error) {
			patternContext = req.UserContext
			return &agent.UserDefinePatternResult{Pattern: pattern}, nil
		},
	}
	profileRepo := profilestore.NewRepository(seedPath)
	require.NoError(t, profileRepo.Save(context.Background(), &domain.ProjectProfile{
		ProjectName: "demo",
		Language:    "go",
		Summary:     "profile",
		GeneratedAt: "2026-06-04 00:00:00",
	}))

	cont := &container.Container{
		SeedPath:     seedPath,
		Config:       configRepo.Get(),
		ConfigRepo:   configRepo,
		PatternRepo:  patternRepo,
		ProfileRepo:  profileRepo,
		StateRepo:    statestore.NewRepository(seedPath),
		Agent:        mockAgent,
		CuratorSvc:   curator.NewService(mockAgent, patternRepo),
		GeneratorSvc: generator.NewGeneratorService(patternRepo, profileRepo, skills.NewLoaderForAgent("codex", "zh-CN"), configRepo),
	}

	require.NoError(t, syncWithUserPattern(context.Background(), cont, "所有 API 必须有错误处理", "business", nil, userContext, nil))

	require.Equal(t, userContext, patternContext)
	require.FileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-dev", "SKILL.md"))
}
