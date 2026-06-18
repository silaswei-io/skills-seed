package container

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/claude"
	"github.com/silaswei-io/skills-seed/internal/agent/codex"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/git"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	workspacestore "github.com/silaswei-io/skills-seed/internal/infra/storage/workspace"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/checker"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/service/learner"
	ws "github.com/silaswei-io/skills-seed/internal/service/workspace"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	bolt "go.etcd.io/bbolt"
)

// Container 应用容器
type Container struct {
	SeedPath              string // .skills-seed 目录的路径
	Config                *config.Config
	ConfigRepo            *config.Repository
	GitRepo               *git.Repository
	PatternRepo           *boltdb.PatternRepository
	PatternReader         domain.PatternRepository
	FileTracker           domain.FileAnalysisTracker
	PatternStats          domain.PatternStatsRepository
	ReviewRepo            domain.ReviewRepository
	ProfileRepo           *profilestore.Repository
	StateRepo             *statestore.Repository
	WorkspaceProfileRepo  *workspacestore.ProfileRepository
	WorkspaceSpecRepo     *workspacestore.SpecRepository
	Agent                 agent.Agent
	AnalyzerSvc           *analyzer.AnalyzerService
	LearnerSvc            *learner.LearnerService
	CheckerSvc            *checker.CheckerService
	GeneratorSvc          *generator.GeneratorService
	WorkspaceGeneratorSvc *ws.WorkspaceGenerator
	CuratorSvc            *curator.Service
	PromptLoader          *promptloader.Loader
	SkillsLoader          *skills.Loader
}

// AgentFactory 创建指定 engine 的 Agent
type AgentFactory func(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) agent.Agent

var (
	agentFactoriesMu sync.RWMutex
	agentFactories   = map[string]AgentFactory{
		"claude": func(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) agent.Agent {
			return claude.New(commandPath, timeout, loader, allowUserPlugins, retryCfg)
		},
		"codex": func(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) agent.Agent {
			return codex.New(commandPath, timeout, loader, allowUserPlugins, retryCfg)
		},
	}
)

// RegisterAgentFactory 注册自定义 Agent 工厂
func RegisterAgentFactory(engine string, factory AgentFactory) {
	if engine == "" || factory == nil {
		return
	}
	agentFactoriesMu.Lock()
	defer agentFactoriesMu.Unlock()
	agentFactories[engine] = factory
}

// RegisterAgentFactoryForTest 注册测试 Agent 工厂，并返回清理函数。
func RegisterAgentFactoryForTest(engine string, factory AgentFactory) func() {
	if engine == "" || factory == nil {
		return func() {}
	}
	agentFactoriesMu.Lock()
	previous, existed := agentFactories[engine]
	agentFactories[engine] = factory
	agentFactoriesMu.Unlock()

	return func() {
		agentFactoriesMu.Lock()
		defer agentFactoriesMu.Unlock()
		if existed {
			agentFactories[engine] = previous
			return
		}
		delete(agentFactories, engine)
	}
}

// NewContainer 创建应用容器
func NewContainer(ctx context.Context, seedPath string) (*Container, error) {
	// 1. 加载配置
	configRepo, err := config.NewRepository(seedPath, "")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ContainerLoadConfigFailed"), err)
	}
	cfg := configRepo.Get()

	// 2. 初始化 i18n
	if err := i18n.Init(configRepo.GetToolLocale()); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ContainerI18nInitFailed"), err)
	}

	// 3. 创建 Git 仓储
	// seedPath 是 .skills-seed 目录的路径，项目根目录是其父目录
	projectRoot := filepath.Dir(seedPath)
	gitRepo := git.NewRepository(projectRoot)

	// 4. 创建 BoltDB 仓储
	dbPath := filepath.Join(seedPath, "memory", "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	if err != nil {
		return nil, patternRepositoryError(err)
	}

	profileRepo := profilestore.NewRepository(seedPath)
	stateRepo := statestore.NewRepository(seedPath)
	workspaceProfileRepo := workspacestore.NewProfileRepository(seedPath)
	workspaceSpecRepo := workspacestore.NewSpecRepository(seedPath)

	// 5. 创建加载器
	promptLoader := promptloader.NewWithLocales(cfg.Agent.Engine, configRepo.GetToolLocale(), configRepo.GetSkillsLocale(), seedPath)
	skillsLoader := skills.NewLoaderForAgent(configRepo.GetEffectiveSkillsTarget(), configRepo.GetSkillsLocale())

	// 6. 创建 Agent
	agentImpl, err := createAgent(cfg, promptLoader)
	if err != nil {
		_ = patternRepo.Close()
		return nil, err
	}

	// 7. 创建服务
	analyzerSvc := analyzer.NewAnalyzerService(agentImpl, configRepo)
	curatorSvc := curator.NewService(agentImpl, patternRepo)
	learnerSvc := learner.NewLearnerService(agentImpl, gitRepo, patternRepo, patternRepo, curatorSvc)

	checkerSvc := checker.NewCheckerService(agentImpl, gitRepo, patternRepo, configRepo)
	generatorSvc := generator.NewGeneratorService(patternRepo, profileRepo, skillsLoader, agentImpl, configRepo)
	workspaceGeneratorSvc := ws.NewWorkspaceGenerator(patternRepo, profileRepo, skillsLoader, agentImpl, configRepo, workspaceProfileRepo, workspaceSpecRepo)

	return &Container{
		SeedPath:              seedPath,
		Config:                cfg,
		ConfigRepo:            configRepo,
		GitRepo:               gitRepo,
		PatternRepo:           patternRepo,
		PatternReader:         patternRepo,
		FileTracker:           patternRepo,
		PatternStats:          patternRepo,
		ReviewRepo:            patternRepo,
		ProfileRepo:           profileRepo,
		StateRepo:             stateRepo,
		WorkspaceProfileRepo:  workspaceProfileRepo,
		WorkspaceSpecRepo:     workspaceSpecRepo,
		Agent:                 agentImpl,
		AnalyzerSvc:           analyzerSvc,
		LearnerSvc:            learnerSvc,
		CheckerSvc:            checkerSvc,
		GeneratorSvc:          generatorSvc,
		WorkspaceGeneratorSvc: workspaceGeneratorSvc,
		CuratorSvc:            curatorSvc,
		PromptLoader:          promptLoader,
		SkillsLoader:          skillsLoader,
	}, nil
}

func createAgent(cfg *config.Config, promptLoader *promptloader.Loader) (agent.Agent, error) {
	timeout := time.Duration(cfg.Agent.Timeout) * time.Second
	engine := cfg.Agent.Engine

	agentFactoriesMu.RLock()
	factory, ok := agentFactories[engine]
	agentFactoriesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%s", i18n.GetWithParams("AgentProviderUnsupported", map[string]interface{}{"Provider": cfg.Agent.Engine}))
	}

	command := cfg.Agent.Commands[engine]
	if command == "" {
		command = engine
	}

	return factory(command, timeout, promptLoader, cfg.Agent.AllowUserPlugins, cfg.Agent.Retry), nil
}

func patternRepositoryError(err error) error {
	if errors.Is(err, bolt.ErrTimeout) {
		return fmt.Errorf("%s: %w. %s", i18n.Get("ContainerCreatePatternRepoFailed"), err, i18n.Get("ContainerPatternDBLockedHint"))
	}
	return fmt.Errorf("%s: %w", i18n.Get("ContainerCreatePatternRepoFailed"), err)
}

// Close 关闭容器
func (c *Container) Close() error {
	if c.PatternRepo != nil {
		return c.PatternRepo.Close()
	}
	return nil
}

// GetPatternRepository 获取 Pattern 仓储
func (c *Container) GetPatternRepository() *boltdb.PatternRepository {
	return c.PatternRepo
}

// GetLoggingConfig 获取日志配置
func (c *Container) GetLoggingConfig() config.LoggingConfig {
	return c.ConfigRepo.GetLoggingConfig()
}
