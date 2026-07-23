package sync

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	gencmd "github.com/silaswei-io/skills-seed/internal/command/generate"
	learncmd "github.com/silaswei-io/skills-seed/internal/command/learn"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
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
	"github.com/silaswei-io/skills-seed/internal/service/syncflow"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestSyncLearnAfterLearnGeneratesWhenLearnChanged(t *testing.T) {
	generateCalled := false

	err := syncflow.RunAfterLearn(domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{ChangedFiles: 1}}, false, func() error {
		generateCalled = true
		return nil
	}, nil)

	require.NoError(t, err)
	require.True(t, generateCalled)
}

func TestSyncLearnAfterLearnSkipsGenerateWhenNoFileChanges(t *testing.T) {
	generateCalled := false

	err := syncflow.RunAfterLearn(domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}}, false, func() error {
		generateCalled = true
		return nil
	}, nil)

	require.NoError(t, err)
	require.False(t, generateCalled)
}

func TestSyncLearnAfterLearnWrapsGenerateError(t *testing.T) {
	errGenerate := errors.New("boom")

	err := syncflow.RunAfterLearn(domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{ChangedFiles: 1}}, false, func() error {
		return errGenerate
	}, nil)

	require.ErrorIs(t, err, errGenerate)
}

func TestSyncLearnAfterLearnGeneratesWhenOutputMissing(t *testing.T) {
	generateCalled := false

	err := syncflow.RunAfterLearn(domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}}, true, func() error {
		generateCalled = true
		return nil
	}, nil)

	require.NoError(t, err)
	require.True(t, generateCalled)
}

func TestSyncGeneratedSkillMissing(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.RootPath = projectRoot
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", "demo")}
	require.NoError(t, configRepo.Update(cfg))
	cont := &container.Container{
		Config:     configRepo.Get(),
		ConfigRepo: configRepo,
	}

	require.True(t, syncGeneratedSkillMissing(cont))

	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".agents", "skills", "demo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, ".agents", "skills", "demo", "SKILL.md"), []byte("# skill\n"), 0644))

	require.False(t, syncGeneratedSkillMissing(cont))
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
	cfg.Skills.Locale = "zh-CN"
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
	staleLearnState := commandstate.NewState("learn-current", "demo", "go", userContext, []domain.FileAnalysisRecord{
		{Path: "internal/stale.go", Hash: "stale"},
	}, nil, []domain.AnalysisUnit{{ID: "stale", EntryPaths: []string{"internal/stale.go"}}})
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

	err = syncLearn(context.Background(), cont, stateScope, userContext, "", syncRunAuto, nil, commandDependenciesForTest())

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

func TestSyncRestartForcesCurrentLearning(t *testing.T) {
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
	cfg.Skills.Locale = "zh-CN"
	cfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", "demo-dev")}
	require.NoError(t, configRepo.Update(cfg))
	require.NoError(t, exec.Command("git", "-C", projectRoot, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\nfunc main() {}\n"), 0644))
	require.NoError(t, exec.Command("git", "-C", projectRoot, "add", "-A").Run())

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "store", "project.db"))
	require.NoError(t, err)
	defer patternRepo.Close()
	record := domain.FileAnalysisRecord{
		Path:           "main.go",
		Hash:           "a071a7bc9d4fd44b32558cc706a5a698",
		HashAlgorithm:  domain.FileAnalysisHashMD5,
		Source:         domain.FileAnalysisSourceCurrentCode,
		AnalysisStatus: domain.FileAnalysisStatusAnalyzed,
		LastAnalyzedAt: "2026-01-01T00:00:00Z",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	require.NoError(t, patternRepo.SaveAnalyzedFiles(context.Background(), []domain.FileAnalysisRecord{record}))

	planCalls := 0
	mockAgent := &mocks.MockAgent{NameVal: "mock", AvailableVal: true}
	mockAgent.PlanAnalysisUnitsFn = func(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
		planCalls++
		return &agent.PlanAnalysisUnitsResult{Units: []domain.AnalysisUnit{{ID: "main", Name: "Main", EntryPaths: []string{"main.go"}}}}, nil
	}
	mockAgent.AnalyzeCurrentCodebaseFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
		pattern := domain.NewPattern("sync-restart-pattern", "Restart Pattern", domain.CategoryBusiness)
		pattern.Confidence = 0.9
		pattern.SetDescription("description")
		pattern.SetRule("rule")
		pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "main.go", Line: 2, Symbol: "main", Kind: "func"}}
		return &agent.AnalyzeCurrentCodebaseResult{Patterns: []domain.Pattern{*pattern}}, nil
	}

	profileRepo := profilestore.NewRepository(seedPath)
	require.NoError(t, profileRepo.Save(context.Background(), &domain.ProjectProfile{
		ProjectName: "demo",
		Language:    "go",
		Summary:     "profile",
		GeneratedAt: "2026-01-01 00:00:00",
	}))
	curatorSvc := curator.NewService(mockAgent, patternRepo)
	cont := &container.Container{
		SeedPath:     seedPath,
		Config:       configRepo.Get(),
		ConfigRepo:   configRepo,
		GitRepo:      git.NewRepository(projectRoot),
		PatternRepo:  patternRepo,
		FileTracker:  patternRepo,
		ProfileRepo:  profileRepo,
		StateRepo:    statestore.NewRepository(seedPath),
		Agent:        mockAgent,
		AnalyzerSvc:  analyzer.NewAnalyzerService(mockAgent, configRepo),
		LearnerSvc:   servicelearner.NewLearnerService(mockAgent, git.NewRepository(projectRoot), patternRepo, patternRepo, curatorSvc),
		GeneratorSvc: generator.NewGeneratorService(patternRepo, profileRepo, skills.NewLoaderForAgent("codex", "zh-CN"), configRepo, nil),
	}

	err = syncLearn(context.Background(), cont, commandutil.CommandStateScope("sync"), "", "", syncRunRestart, nil, commandDependenciesForTest())

	require.NoError(t, err)
	require.Equal(t, 1, planCalls)
	require.FileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-dev", "SKILL.md"))
}

func commandDependenciesForTest() Dependencies {
	return Dependencies{
		LearnCurrent: func(cont *container.Container, req syncflow.LearnRequest) (domain.LearnCurrentResult, error) {
			return learncmd.RunLearnCurrentWithStateScopeOptions(cont, req.StateScope, req.UserContext, learncmd.CurrentRunOptions{
				Force:          req.Force,
				CurationOutput: req.CurationOutput,
			})
		},
		Generate: gencmd.RunGenerate,
	}
}

func TestHasSyncCommandState(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := commandstate.NewRepository(seedPath, "sync")

	hasState, err := hasSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.False(t, hasState)

	require.NoError(t, repo.Save(context.Background(), commandstate.NewState("sync", "demo", "go", "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "hash"},
	}, nil, []domain.AnalysisUnit{{ID: "main", EntryPaths: []string{"main.go"}}})))

	hasState, err = hasSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.True(t, hasState)
}

func TestHasResumableSyncCommandStateRequiresInputsAndUnits(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := commandstate.NewRepository(seedPath, "sync")

	resumable, err := hasResumableSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.False(t, resumable)

	require.NoError(t, repo.Save(context.Background(), commandstate.NewState("sync", "demo", "go", "", nil, nil, nil)))
	resumable, err = hasResumableSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.False(t, resumable)

	require.NoError(t, repo.Save(context.Background(), commandstate.NewState("sync", "demo", "go", "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "hash"},
	}, nil, []domain.AnalysisUnit{{ID: "main", EntryPaths: []string{"main.go"}}})))
	resumable, err = hasResumableSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.True(t, resumable)
}

func TestNormalizeSyncInputsTrimsAndRejectsUnsafeFiles(t *testing.T) {
	inputs, err := normalizeSyncInputs(syncInputs{
		UserContext: " context ",
	})
	require.NoError(t, err)
	require.Equal(t, "context", inputs.UserContext)
}

func TestSyncCmdOnlyExposesSyncFlags(t *testing.T) {
	cmd := Cmd(&container.Container{})

	require.NotNil(t, cmd.Flags().Lookup("context"))
	require.NotNil(t, cmd.Flags().Lookup("context-path"))
	require.NotNil(t, cmd.Flags().Lookup("resume"))
	require.NotNil(t, cmd.Flags().Lookup("restart"))
	require.NotNil(t, cmd.Flags().Lookup("no-interactive"))
	require.NotNil(t, cmd.Flags().Lookup("curation-output"))
	require.Nil(t, cmd.Flags().Lookup("pattern"))
	require.Nil(t, cmd.Flags().Lookup("files"))
	require.Nil(t, cmd.Flags().Lookup("category"))
}

func TestSyncCurationOutputRequiresExplicitResume(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	cmd := Cmd(&container.Container{})
	cmd.SetArgs([]string{"--curation-output", "curation.raw.txt", "--no-interactive"})

	err := cmd.Execute()

	require.ErrorContains(t, err, "--curation-output 必须与 --resume 一起使用")
}

func TestSyncLearnPassesCurationOutputToLearning(t *testing.T) {
	var received syncflow.LearnRequest
	err := syncLearn(context.Background(), nil, "sync", "context", "curation.raw.txt", syncRunResume, nil, Dependencies{
		LearnCurrent: func(_ *container.Container, req syncflow.LearnRequest) (domain.LearnCurrentResult, error) {
			received = req
			return domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}}, nil
		},
		Generate: func(*container.Container) error { return nil },
	})

	require.NoError(t, err)
	require.Equal(t, syncflow.LearnRequest{
		StateScope:     "sync",
		UserContext:    "context",
		CurationOutput: "curation.raw.txt",
	}, received)
}

func TestInteractiveSyncRunModeOptionsDifferForFirstRun(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	title, options := interactiveSyncRunModeOptions(true)
	require.Contains(t, title, "第一版")
	require.Len(t, options, 2)
	require.Equal(t, syncRunAuto, options[0].Value)
	require.Equal(t, syncRunMode(""), options[1].Value)

	title, options = interactiveSyncRunModeOptions(false)
	require.Contains(t, title, "sync")
	require.Len(t, options, 3)
	require.Equal(t, syncRunAuto, options[0].Value)
	require.Equal(t, syncRunRestart, options[1].Value)
	require.Equal(t, syncRunMode(""), options[2].Value)
}

func TestSyncContextPathFlagIsRepeatable(t *testing.T) {
	cmd := Cmd(&container.Container{})

	err := cmd.ParseFlags([]string{"--context-path", "docs/plan.md", "--context-path", "docs/specs"})

	require.NoError(t, err)
	values, err := cmd.Flags().GetStringArray("context-path")
	require.NoError(t, err)
	require.Equal(t, []string{"docs/plan.md", "docs/specs"}, values)
}

func TestSyncContextPathDoesNotConsumePositionalArgs(t *testing.T) {
	cmd := Cmd(&container.Container{})
	cmd.SetArgs([]string{"--context-path", "docs/plan.md", "docs/specs"})

	err := cmd.Execute()

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown command")
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
	state := commandstate.NewState("sync", "demo", "go", "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "hash"},
	}, nil, []domain.AnalysisUnit{{ID: "main", EntryPaths: []string{"main.go"}}})
	require.NoError(t, syncRepo.Save(context.Background(), state))
	require.NoError(t, learnRepo.Save(context.Background(), state))

	require.NoError(t, commandstate.NewRepository(seedPath, "sync").Clear())

	_, err := syncRepo.Load(context.Background())
	require.ErrorIs(t, err, commandstate.ErrStateNotFound)
	_, err = learnRepo.Load(context.Background())
	require.NoError(t, err)
}
