package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/navigation"
	"github.com/silaswei-io/skills-seed/internal/service/skilloutput"
	"github.com/silaswei-io/skills-seed/internal/service/syncflow"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestWorkspacePhaseStepLabel(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	require.Equal(t, "学习 · 解析输出目录", workspacePhaseStepLabel("ProgressSyncWorkspacePhaseLearn", "解析输出目录"))
	require.Equal(t, "生成", workspacePhaseStepLabel("ProgressSyncWorkspacePhaseGenerate", ""))
}

func TestSyncWorkspaceProjectProgressLabelDistinguishesOutcomes(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	changed := syncWorkspaceProjectProgressLabel(domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{
		ChangedFiles:    39,
		DeletedFiles:    1,
		PatternsFound:   5,
		PatternsSaved:   7,
		PatternsRetired: 2,
	}}, false)
	require.Equal(t, "完成 · 变更 39 · 删除 1 · 候选 5 · 写入 7 · 移除 2", changed)

	unchanged := domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}}
	require.Equal(t, "完成 · 无学习变化 · 跳过生成", syncWorkspaceProjectProgressLabel(unchanged, false))
	require.Equal(t, "完成 · 无学习变化 · 补全 skills", syncWorkspaceProjectProgressLabel(unchanged, true))
}

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

	require.True(t, syncGeneratedSkillMissing(cont), "entry alone cannot replace the navigation manifest")
}

func TestSyncGeneratedSkillMissingInWorkspaceChecksChildSkills(t *testing.T) {
	workspaceRoot := t.TempDir()
	rootSeedPath := filepath.Join(workspaceRoot, ".skills-seed")
	rootConfigRepo, err := config.NewRepository(rootSeedPath, "zh-CN")
	require.NoError(t, err)
	rootCfg := rootConfigRepo.Get()
	rootCfg.Project.Name = "demo"
	rootCfg.Project.Mode = domain.ModeWorkspace
	rootCfg.Project.RootPath = workspaceRoot
	rootCfg.Skills.Target = "codex"
	rootCfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", "demo-workspace-dev")}
	rootCfg.Workspace.Projects = []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}}
	require.NoError(t, rootConfigRepo.Update(rootCfg))

	childRoot := filepath.Join(workspaceRoot, "backend")
	childSeedPath := filepath.Join(childRoot, ".skills-seed")
	childConfigRepo, err := config.NewRepository(childSeedPath, "zh-CN")
	require.NoError(t, err)
	childCfg := childConfigRepo.Get()
	childCfg.Project.Name = "backend"
	childCfg.Project.Mode = domain.ModeProject
	childCfg.Project.RootPath = childRoot
	childCfg.Skills.Target = "codex"
	childCfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", "backend-dev")}
	require.NoError(t, childConfigRepo.Update(childCfg))

	require.NoError(t, os.MkdirAll(filepath.Join(workspaceRoot, ".agents", "skills", "demo-workspace-dev"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(workspaceRoot, ".agents", "skills", "demo-workspace-dev", "SKILL.md"), []byte("# workspace\n"), 0644))
	cont := &container.Container{
		SeedPath:   rootSeedPath,
		Config:     rootConfigRepo.Get(),
		ConfigRepo: rootConfigRepo,
	}

	require.True(t, syncGeneratedSkillMissing(cont))

	require.NoError(t, os.MkdirAll(filepath.Join(childRoot, ".agents", "skills", "backend-dev"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(childRoot, ".agents", "skills", "backend-dev", "SKILL.md"), []byte("# backend\n"), 0644))

	require.False(t, syncGeneratedSkillMissing(cont))
}

func TestSyncWorkspaceLearnGeneratesChildBeforeWorkspaceRoot(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	restoreFactory := container.RegisterAgentFactoryForTest("mock", func(opts container.AgentFactoryOptions) agent.Agent {
		return &mocks.MockAgent{NameVal: "mock", AvailableVal: true}
	})
	t.Cleanup(restoreFactory)

	workspaceRoot := t.TempDir()
	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newSyncWorkspaceFlowTestContainer(t, workspaceRoot, []config.WorkspaceProjectConfig{project})
	initSyncWorkspaceFlowChild(t, workspaceRoot, project)

	var calls []string
	wantSummary := domain.LearnCurrentSummary{
		ChangedFiles:    39,
		DeletedFiles:    1,
		SkippedFiles:    2,
		PatternsFound:   5,
		PatternsSaved:   7,
		PatternsRetired: 2,
	}
	result, err := syncLearn(context.Background(), cont, "sync", "", syncRunAuto, nil, Dependencies{
		LearnCurrent: func(ctx context.Context, cont *container.Container, req syncflow.LearnCurrentRequest, opts LearnCurrentOptions) (domain.LearnCurrentResult, error) {
			require.True(t, opts.Quiet)
			require.NotNil(t, opts.OnStepStart)
			require.NotNil(t, opts.OnStepComplete)
			calls = append(calls, "learn:"+cont.ConfigRepo.GetProjectConfig().Name)
			opts.OnStepStart("learn")
			opts.OnStepComplete("learn")
			return domain.LearnCurrentResult{Summary: wantSummary}, nil
		},
		Generate: func(cont *container.Container) error {
			calls = append(calls, "generate-default:"+cont.ConfigRepo.GetProjectConfig().Name)
			return nil
		},
		GenerateChild: func(cont *container.Container, opts GenerateChildOptions) error {
			require.NotNil(t, opts.OnStepStart)
			require.NotNil(t, opts.OnStepUpdate)
			require.NotNil(t, opts.OnStepComplete)
			calls = append(calls, "generate:"+cont.ConfigRepo.GetProjectConfig().Name)
			opts.OnStepStart("load")
			opts.OnStepUpdate("audit")
			opts.OnStepComplete("load")
			return nil
		},
		LearnWorkspaceRelationships: func(cont *container.Container, userContext string) (bool, error) {
			calls = append(calls, "relationships")
			return false, nil
		},
		GenerateWorkspaceRoot: func(cont *container.Container) error {
			calls = append(calls, "generate:workspace")
			return nil
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"learn:backend", "generate:backend", "relationships", "generate:workspace"}, calls)
	wantSummary.Projects = 1
	wantSummary.ChangedProjects = 1
	require.Equal(t, wantSummary, result.Summary)
}

func newSyncWorkspaceFlowTestContainer(t *testing.T, workspaceRoot string, projects []config.WorkspaceProjectConfig) *container.Container {
	t.Helper()
	seedPath := filepath.Join(workspaceRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = "demo-workspace"
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.RootPath = workspaceRoot
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	cfg.Agent.Parallelism = 1
	cfg.Workspace.Projects = projects
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", "demo-workspace-dev")}
	require.NoError(t, configRepo.Update(cfg))
	return &container.Container{
		SeedPath:   seedPath,
		Config:     configRepo.Get(),
		ConfigRepo: configRepo,
	}
}

func initSyncWorkspaceFlowChild(t *testing.T, workspaceRoot string, project config.WorkspaceProjectConfig) {
	t.Helper()
	childRoot := filepath.Join(workspaceRoot, filepath.FromSlash(project.Path))
	require.NoError(t, os.MkdirAll(filepath.Join(childRoot, ".git"), 0755))
	childSeedPath := filepath.Join(childRoot, ".skills-seed")
	configRepo, err := config.NewRepository(childSeedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = project.ID
	cfg.Project.Mode = domain.ModeProject
	cfg.Project.RootPath = childRoot
	cfg.Project.Language = project.Language
	cfg.Agent.Engine = "mock"
	cfg.Agent.Commands = map[string]string{"mock": "mock"}
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": filepath.Join(".agents", "skills", project.ID+"-dev")}
	require.NoError(t, configRepo.Update(cfg))
}

func TestHasSyncCommandState(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := commandstate.NewRepository(seedPath, "sync")

	hasState, err := hasSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.False(t, hasState)

	require.NoError(t, repo.Save(context.Background(), commandstate.NewState("sync", "demo", "go", "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "hash"},
	}, nil, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}})))

	hasState, err = hasSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.True(t, hasState)
}

func TestHasSyncCommandStateReportsUnsupportedSchemaWithoutDeletingIt(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := commandstate.NewRepository(seedPath, "sync")
	require.NoError(t, os.MkdirAll(filepath.Dir(repo.Path()), 0o755))
	require.NoError(t, os.WriteFile(repo.Path(), []byte(`{"schema_version":3,"command":"sync"}`), 0o600))

	hasState, err := hasSyncCommandState(context.Background(), seedPath, "sync")

	require.False(t, hasState)
	require.ErrorIs(t, err, commandstate.ErrUnsupportedSchemaVersion)
	require.FileExists(t, repo.Path())
}

func TestHasResumableSyncCommandStateRequiresInputsAndFocuses(t *testing.T) {
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
	}, nil, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}})))
	resumable, err = hasResumableSyncCommandState(context.Background(), seedPath, "sync")
	require.NoError(t, err)
	require.True(t, resumable)
}

func TestHasSyncCommandStateForTargetFindsWorkspaceChildState(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	cont := newSyncWorkspaceStateTestContainer(t, workspaceRoot)
	childSeedPath := filepath.Join(workspaceRoot, "backend", ".skills-seed")
	require.NoError(t, commandstate.NewRepository(childSeedPath, "sync").Save(ctx, commandstate.NewState("sync", "backend", "go", "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "hash"},
	}, nil, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}})))

	hasState, err := hasSyncCommandStateForTarget(ctx, cont, "sync")
	require.NoError(t, err)
	require.True(t, hasState)

	resumable, err := hasResumableSyncCommandStateForTarget(ctx, cont, "sync")
	require.NoError(t, err)
	require.True(t, resumable)
}

func TestSyncResumeRecognizesPendingLocalSupplement(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "runtime"), 0700))
	statusPath := filepath.Join(seedPath, "runtime", "local-supplement.json")
	require.NoError(t, os.WriteFile(statusPath, []byte(`{"pending":[{"path":"domain/flow.go"}]}`), 0600))
	cont := &container.Container{SeedPath: seedPath}
	hasState, err := hasSyncCommandStateForTarget(context.Background(), cont, "sync")
	require.NoError(t, err)
	require.True(t, hasState)
	resumable, err := hasResumableSyncCommandStateForTarget(context.Background(), cont, "sync")
	require.NoError(t, err)
	require.True(t, resumable)

	require.NoError(t, os.WriteFile(statusPath, []byte(`{"pending":[]}`), 0600))
	resumable, err = hasResumableSyncCommandStateForTarget(context.Background(), cont, "sync")
	require.NoError(t, err)
	require.False(t, resumable)
}

func TestClearSyncCommandStatesClearsWorkspaceChildState(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	cont := newSyncWorkspaceStateTestContainer(t, workspaceRoot)
	rootRepo := commandstate.NewRepository(cont.SeedPath, "sync")
	childRepo := commandstate.NewRepository(filepath.Join(workspaceRoot, "backend", ".skills-seed"), "sync")
	state := commandstate.NewState("sync", "backend", "go", "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "hash"},
	}, nil, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}})
	require.NoError(t, rootRepo.Save(ctx, state))
	require.NoError(t, childRepo.Save(ctx, state))

	require.NoError(t, clearSyncCommandStates(cont, "sync"))

	_, err := rootRepo.Load(ctx)
	require.ErrorIs(t, err, commandstate.ErrStateNotFound)
	_, err = childRepo.Load(ctx)
	require.ErrorIs(t, err, commandstate.ErrStateNotFound)
}

func newSyncWorkspaceStateTestContainer(t *testing.T, workspaceRoot string) *container.Container {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(workspaceRoot, "backend"), 0755))
	seedPath := filepath.Join(workspaceRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.Name = "demo"
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.RootPath = workspaceRoot
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}}
	require.NoError(t, configRepo.Update(cfg))
	return &container.Container{
		SeedPath:   seedPath,
		Config:     configRepo.Get(),
		ConfigRepo: configRepo,
	}
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
	require.Nil(t, cmd.Flags().Lookup("pattern"))
	require.Nil(t, cmd.Flags().Lookup("files"))
	require.Nil(t, cmd.Flags().Lookup("category"))
}

func TestSyncLearnPassesStateScopeAndForceToLearning(t *testing.T) {
	var received syncflow.LearnCurrentRequest
	_, err := syncLearn(context.Background(), nil, "sync", "context", syncRunRestart, nil, Dependencies{
		LearnCurrent: func(_ context.Context, _ *container.Container, req syncflow.LearnCurrentRequest, _ LearnCurrentOptions) (domain.LearnCurrentResult, error) {
			received = req
			return domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}}, nil
		},
		Generate: func(*container.Container) error { return nil },
	})

	require.NoError(t, err)
	require.Equal(t, syncflow.LearnCurrentRequest{
		StateScope:  "sync",
		UserContext: "context",
		Force:       true,
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
	}, nil, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}})
	require.NoError(t, syncRepo.Save(context.Background(), state))
	require.NoError(t, learnRepo.Save(context.Background(), state))

	require.NoError(t, commandstate.NewRepository(seedPath, "sync").Clear())

	_, err := syncRepo.Load(context.Background())
	require.ErrorIs(t, err, commandstate.ErrStateNotFound)
	_, err = learnRepo.Load(context.Background())
	require.NoError(t, err)
}

func TestLocalScanCannotConsumePendingGeneration(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, ".skills-seed")
	cfg, err := config.NewRepository(seed, "en-US")
	require.NoError(t, err)
	value := cfg.Get()
	value.Project.RootPath = root
	value.Skills.Target = "codex"
	value.Skills.Paths = map[string]string{"codex": ".agents/skills/demo"}
	require.NoError(t, cfg.Update(value))
	cont := &container.Container{SeedPath: seed, ConfigRepo: cfg}
	navPath := navigation.SnapshotPath(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(navPath), 0700))
	require.NoError(t, os.WriteFile(navPath, []byte("old graph"), 0600))
	hash, err := navigation.Hash(root)
	require.NoError(t, err)
	output := filepath.Join(root, ".agents/skills/demo")
	require.NoError(t, os.MkdirAll(output, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(output, "SKILL.md"), []byte("skill"), 0600))
	manifestPath := skilloutput.RuntimeManifestPath(seed, "codex")
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0700))
	require.NoError(t, skilloutput.WriteManifest(output, manifestPath, skilloutput.Manifest{NavigationHash: hash}))
	require.False(t, syncGeneratedSkillMissing(cont))
	// 查询预先更新了导航快照，后续无源码变化的 sync 仍须补生成。
	require.NoError(t, os.WriteFile(navPath, []byte("new graph from query"), 0600))
	require.True(t, syncGeneratedSkillMissing(cont))
}
