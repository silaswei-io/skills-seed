package container

import (
	"context"
	"fmt"
	"path/filepath"
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
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/checker"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/service/learner"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
)

// Container 应用容器
type Container struct {
	SeedPath             string // .skills-seed 目录的路径
	Config               *config.Config
	ConfigRepo           *config.Repository
	GitRepo              *git.Repository
	PatternRepo          *boltdb.PatternRepository
	ProfileRepo          *profilestore.Repository
	StateRepo            *statestore.Repository
	WorkspaceProfileRepo *workspacestore.ProfileRepository
	WorkspaceSpecRepo    *workspacestore.SpecRepository
	Agent                agent.Agent
	AnalyzerSvc          *analyzer.AnalyzerService
	LearnerSvc           *learner.LearnerService
	CheckerSvc           *checker.CheckerService
	GeneratorSvc         *generator.GeneratorService
	MergerSvc            *merger.MergerService
	PromptLoader         *prompts.Loader
	SkillsLoader         *skills.Loader
}

// AgentFactory 创建指定 provider 的 Agent
type AgentFactory func(commandPath string, timeout time.Duration, loader *prompts.Loader, allowUserPlugins bool) agent.Agent

var agentFactories = map[string]AgentFactory{
	"claude": func(commandPath string, timeout time.Duration, loader *prompts.Loader, allowUserPlugins bool) agent.Agent {
		return claude.New(commandPath, timeout, loader, allowUserPlugins)
	},
	"codex": func(commandPath string, timeout time.Duration, loader *prompts.Loader, allowUserPlugins bool) agent.Agent {
		return codex.New(commandPath, timeout, loader, allowUserPlugins)
	},
}

// RegisterAgentFactory 注册自定义 Agent 工厂
func RegisterAgentFactory(provider string, factory AgentFactory) {
	if provider == "" || factory == nil {
		return
	}
	agentFactories[provider] = factory
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
	locale := cfg.Project.Locale
	if locale == "" {
		locale = domain.DefaultLocale // 默认中文
	}
	if err := i18n.Init(locale); err != nil {
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
		return nil, fmt.Errorf("%s: %w", i18n.Get("ContainerCreatePatternRepoFailed"), err)
	}

	profileRepo := profilestore.NewRepository(seedPath)
	stateRepo := statestore.NewRepository(seedPath)
	workspaceProfileRepo := workspacestore.NewProfileRepository(seedPath)
	workspaceSpecRepo := workspacestore.NewSpecRepository(seedPath)

	// 5. 创建加载器
	promptLoader := prompts.NewLoader(cfg.Agent.Provider, locale, seedPath)
	skillsLoader := skills.NewLoaderForAgent(cfg.Agent.Provider, locale)

	// 6. 创建 Agent
	agentImpl, err := createAgent(cfg, promptLoader)
	if err != nil {
		_ = patternRepo.Close()
		return nil, err
	}

	// 7. 创建服务
	analyzerSvc := analyzer.NewAnalyzerService(agentImpl, configRepo)
	mergerSvc := merger.NewMergerService(agentImpl, patternRepo)
	learnerSvc := learner.NewLearnerService(agentImpl, gitRepo, patternRepo, patternRepo, mergerSvc)

	checkerSvc := checker.NewCheckerService(agentImpl, gitRepo, patternRepo, configRepo)
	generatorSvc := generator.NewGeneratorService(patternRepo, profileRepo, skillsLoader, agentImpl, configRepo)

	return &Container{
		SeedPath:             seedPath,
		Config:               cfg,
		ConfigRepo:           configRepo,
		GitRepo:              gitRepo,
		PatternRepo:          patternRepo,
		ProfileRepo:          profileRepo,
		StateRepo:            stateRepo,
		WorkspaceProfileRepo: workspaceProfileRepo,
		WorkspaceSpecRepo:    workspaceSpecRepo,
		Agent:                agentImpl,
		AnalyzerSvc:          analyzerSvc,
		LearnerSvc:           learnerSvc,
		CheckerSvc:           checkerSvc,
		GeneratorSvc:         generatorSvc,
		MergerSvc:            mergerSvc,
		PromptLoader:         promptLoader,
		SkillsLoader:         skillsLoader,
	}, nil
}

func createAgent(cfg *config.Config, promptLoader *prompts.Loader) (agent.Agent, error) {
	timeout := time.Duration(cfg.Agent.Timeout) * time.Second
	provider := cfg.Agent.Provider

	factory, ok := agentFactories[provider]
	if !ok {
		return nil, fmt.Errorf("%s", i18n.GetWithParams("AgentProviderUnsupported", map[string]interface{}{"Provider": cfg.Agent.Provider}))
	}

	command := cfg.Agent.Commands[provider]
	if command == "" {
		command = provider
	}

	return factory(command, timeout, promptLoader, cfg.Agent.AllowUserPlugins), nil
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
