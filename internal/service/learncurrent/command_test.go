package learncurrent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/git"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	workspacestore "github.com/silaswei-io/skills-seed/internal/infra/storage/workspace"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestLearnCurrentStepCount(t *testing.T) {
	require.Equal(t, 6, learnCurrentProjectStepTotal)
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

func learnCurrentPatternForTest(id, name string, category domain.Category, path string) *domain.Pattern {
	pattern := domain.NewPattern(id, name, category)
	pattern.Rule = "When changing this behavior, preserve the evidenced project constraint."
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: path, Kind: "file"}}
	return pattern
}

func admittedLearnCurrentPatternForTest(id, name string, category domain.Category, path string) *domain.Pattern {
	pattern := learnCurrentPatternForTest(id, name, category, path)
	pattern.Confidence = 0.9
	pattern.Frequency = 1
	return pattern
}

func TestBuildDeltaFocusResultsRequiresDecisionForEveryFocus(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	run := &learnCurrentProjectRun{}
	batch := learnCurrentBatch{focuses: []indexedEvidenceFocus{
		{index: 0, focus: domain.EvidenceFocus{ID: "auth", Name: "认证登录"}},
		{index: 1, focus: domain.EvidenceFocus{ID: "key", Name: "密钥创建"}},
	}}
	batchFocuses := []analyzer.AnalyzeCurrentEvidenceFocus{
		{EvidenceFocus: batch.focuses[0].focus},
		{EvidenceFocus: batch.focuses[1].focus},
	}

	_, err := run.buildDeltaFocusResults(batch, batchFocuses, nil, &analyzer.AnalyzeCurrentDeltaBatchResult{
		Changes: []domain.KnowledgeChange{{
			FocusAction:   domain.KnowledgeFocusExisting,
			FocusID:       "auth",
			FocusName:     "认证登录",
			PatternAction: domain.KnowledgePatternNoChange,
			Anchors:       []domain.PatternDiffAnchor{{Path: "internal/auth/login.go"}},
		}},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), i18n.GetWithParams("LearnCurrentDeltaBatchMissedFocus", map[string]interface{}{"Focus": "key"}))

	results, err := run.buildDeltaFocusResults(batch, batchFocuses, nil, &analyzer.AnalyzeCurrentDeltaBatchResult{
		Changes: []domain.KnowledgeChange{
			{FocusAction: domain.KnowledgeFocusExisting, FocusID: "auth", FocusName: "认证登录", PatternAction: domain.KnowledgePatternNoChange, Anchors: []domain.PatternDiffAnchor{{Path: "internal/auth/login.go"}}},
			{FocusAction: domain.KnowledgeFocusExisting, FocusID: "key", FocusName: "密钥创建", PatternAction: domain.KnowledgePatternNoChange, Anchors: []domain.PatternDiffAnchor{{Path: "internal/key/create.go"}}},
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 2)
	require.True(t, results[0].completed)
	require.True(t, results[1].completed)
	require.Empty(t, results[0].patterns)
	require.Empty(t, results[1].patterns)
}

func TestBuildDeltaFocusResultsMapsNewFocusByAnchor(t *testing.T) {
	run := &learnCurrentProjectRun{
		learnProjectCtx: learnProjectCtx{
			projectRoot: "/repo",
		},
	}
	batch := learnCurrentBatch{focuses: []indexedEvidenceFocus{
		{index: 0, focus: domain.EvidenceFocus{ID: "payment", Name: "支付流程"}},
	}}
	batchFocuses := []analyzer.AnalyzeCurrentEvidenceFocus{{
		EvidenceFocus: batch.focuses[0].focus,
		FocusAbsPaths: []string{"/repo/internal/payment/pay.go"},
	}}

	results, err := run.buildDeltaFocusResults(batch, batchFocuses, nil, &analyzer.AnalyzeCurrentDeltaBatchResult{
		Changes: []domain.KnowledgeChange{{
			FocusAction:   domain.KnowledgeFocusNew,
			FocusID:       "new-payment-policy",
			FocusName:     "新支付策略",
			PatternAction: domain.KnowledgePatternAdd,
			Proposal: &domain.Pattern{
				ID:       "payment-policy",
				Name:     "支付策略",
				Category: domain.CategoryBusiness,
			},
			Anchors: []domain.PatternDiffAnchor{{Path: "internal/payment/pay.go"}},
		}},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Len(t, results[0].patterns, 1)
	require.Equal(t, "payment", results[0].focus.ID)
	require.Equal(t, "支付流程", results[0].focus.Name)
}

func TestBuildDeltaFocusResultsAcceptsNoChangeDecisionWithoutAnchor(t *testing.T) {
	run := &learnCurrentProjectRun{
		learnProjectCtx: learnProjectCtx{
			projectRoot: "/repo",
		},
	}
	batch := learnCurrentBatch{focuses: []indexedEvidenceFocus{
		{index: 0, focus: domain.EvidenceFocus{ID: "app-config", Name: "应用配置"}},
	}}
	batchFocuses := []analyzer.AnalyzeCurrentEvidenceFocus{{
		EvidenceFocus: batch.focuses[0].focus,
		FocusAbsPaths: []string{"/repo/src/common/init.ts"},
	}}

	results, err := run.buildDeltaFocusResults(batch, batchFocuses, nil, &analyzer.AnalyzeCurrentDeltaBatchResult{
		Changes: []domain.KnowledgeChange{{
			FocusAction:   domain.KnowledgeFocusNoChange,
			FocusID:       "app-config",
			FocusName:     "应用配置",
			PatternAction: domain.KnowledgePatternNoChange,
			Reason:        "diff does not reveal reusable knowledge",
		}},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].completed)
	require.Empty(t, results[0].patterns)
}

func TestBuildDeltaFocusResultsRetiresOnlyEligibleRelatedPattern(t *testing.T) {
	run := &learnCurrentProjectRun{
		learnProjectCtx: learnProjectCtx{
			projectRoot: "/repo",
		},
	}
	batch := learnCurrentBatch{focuses: []indexedEvidenceFocus{{
		index: 0,
		focus: domain.EvidenceFocus{ID: "auth", Name: "认证登录"},
	}}}
	batchFocuses := []analyzer.AnalyzeCurrentEvidenceFocus{{
		EvidenceFocus: batch.focuses[0].focus,
		FocusAbsPaths: []string{"/repo/internal/auth/login.go"},
	}}
	learned := domain.NewPattern("legacy-auth-flow", "旧认证流程", domain.CategoryBusiness)
	learned.Source = domain.SourceLearnedCurrent
	userDefined := domain.NewPattern("explicit-auth-policy", "认证策略", domain.CategoryBusiness)
	userDefined.Source = domain.SourceUserDefined

	results, err := run.buildDeltaFocusResults(batch, batchFocuses, map[string][]domain.Pattern{
		"auth": {*learned, *userDefined},
	}, &analyzer.AnalyzeCurrentDeltaBatchResult{
		Changes: []domain.KnowledgeChange{
			{
				FocusID:       "auth",
				PatternAction: domain.KnowledgePatternRetire,
				PatternID:     learned.ID,
				Anchors:       []domain.PatternDiffAnchor{{Path: "internal/auth/login.go"}},
			},
			{
				FocusID:       "auth",
				PatternAction: domain.KnowledgePatternRetire,
				PatternID:     userDefined.ID,
				Anchors:       []domain.PatternDiffAnchor{{Path: "internal/auth/login.go"}},
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, []string{learned.ID}, results[0].retiredPatternIDs)
}

func TestRunLearnWorkspaceCurrentRequiresInitializedChildProject(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend")
	require.NoError(t, os.MkdirAll(childRoot, 0755))
	require.NoError(t, exec.Command("git", "-C", childRoot, "init").Run())

	_, err := runLearnCurrent(context.Background(), cont, learnCurrentOptions{scopeKind: runjournal.ScopeProject})
	require.Error(t, err)
	require.Contains(t, err.Error(), "backend")
	require.Contains(t, err.Error(), "skills-seed workspace add")
}

func TestRunLearnWorkspaceCurrentRequiresChildGitRepository(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	project := config.WorkspaceProjectConfig{ID: "backend", Path: "backend", Type: "backend", Language: "go"}
	cont := newLearnCurrentTestContainer(t, domain.ModeWorkspace, []config.WorkspaceProjectConfig{project})
	childRoot := filepath.Join(cont.ConfigRepo.GetProjectConfig().RootPath, "backend")
	require.NoError(t, os.MkdirAll(filepath.Join(childRoot, ".skills-seed"), 0755))
	childConfigRepo, err := config.NewRepository(filepath.Join(childRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := childConfigRepo.Get()
	cfg.Project.Mode = domain.ModeProject
	require.NoError(t, childConfigRepo.Update(cfg))

	_, err = runLearnCurrent(context.Background(), cont, learnCurrentOptions{scopeKind: runjournal.ScopeProject})
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
			pattern := learnCurrentPatternForTest("p1", "Error Handling", domain.CategoryError, "main.go")
			return &agent.AnalyzeCurrentCodebaseResult{
				Patterns: []domain.Pattern{*pattern},
			}, nil
		},
		RefreshProjectProfileFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return &agent.AnalyzeProjectResult{
				Language: "go",
				Summary:  "profile",
			}, nil
		},
	}
	gitRepo := git.NewRepository(projectRoot)
	patternNormSvc := patternnorm.NewService(patternRepo)

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
		FileTracker:          patternRepo,
		PatternNormSvc:       patternNormSvc,
	}
}
