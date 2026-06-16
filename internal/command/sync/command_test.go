package sync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestSyncLearnAfterLearnSkipsGenerateWhenLearnDidNotDirtySkills(t *testing.T) {
	generateCalled := false

	err := syncLearnAfterLearn(domain.LearnCurrentResult{}, func() error {
		generateCalled = true
		return nil
	})

	require.NoError(t, err)
	require.False(t, generateCalled)
}

func TestSyncLearnAfterLearnGeneratesWhenLearnDirtiedSkills(t *testing.T) {
	generateCalled := false

	err := syncLearnAfterLearn(domain.LearnCurrentResult{
		SkillsDirty: domain.SkillsDirtyTarget{Workspace: true},
	}, func() error {
		generateCalled = true
		return nil
	})

	require.NoError(t, err)
	require.True(t, generateCalled)
}

func TestSyncLearnAfterLearnWrapsGenerateError(t *testing.T) {
	errGenerate := errors.New("boom")

	err := syncLearnAfterLearn(domain.LearnCurrentResult{
		SkillsDirty: domain.SkillsDirtyTarget{Project: true},
	}, func() error {
		return errGenerate
	})

	require.ErrorIs(t, err, errGenerate)
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

	patternRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "memory", "project.db"))
	require.NoError(t, err)
	defer patternRepo.Close()

	var patternContext string
	generateCalled := false
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
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			generateCalled = true
			return &agent.GenerateSkillsResult{}, nil
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
		GeneratorSvc: generator.NewGeneratorService(patternRepo, profileRepo, skills.NewLoaderForAgent("codex", "zh-CN"), mockAgent, configRepo),
	}

	require.NoError(t, syncWithUserPattern(context.Background(), cont, "所有 API 必须有错误处理", "business", nil, userContext))

	require.Equal(t, userContext, patternContext)
	require.True(t, generateCalled)
}
