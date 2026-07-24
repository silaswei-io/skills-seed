package initcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cliskillscmd "github.com/silaswei-io/skills-seed/internal/command/cliskills"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

type commandOptions struct {
	locale                 string
	skillsLocale           string
	mode                   string
	agent                  string
	skills                 string
	workspace              bool
	noInteractive          bool
	installGlobalCLISkills bool
	globalCLISkillsTarget  string
	agentTotalParallelism  int
	learningMode           config.LearningMode
	learningScope          config.LearningScope
}

// Cmd 返回 init 命令
func Cmd() *cobra.Command {
	opts := commandOptions{mode: domain.ModeProject}
	initCmd := &cobra.Command{
		Use:     "init",
		Short:   i18n.Get("InitShort"),
		Long:    i18n.Get("InitLongDesc"),
		Example: i18n.Get("InitExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitCommand(cmd, &opts)
		},
	}

	// 添加 --locale 参数
	initCmd.Flags().StringVarP(&opts.locale, "locale", "l", "", i18n.Get("InitFlagLocale"))
	initCmd.Flags().StringVar(&opts.skillsLocale, "skills-locale", "", i18n.Get("InitFlagSkillsLocale"))
	initCmd.Flags().StringVar(&opts.mode, "mode", domain.ModeProject, i18n.Get("InitFlagMode"))
	initCmd.Flags().StringVar(&opts.agent, "agent", "", i18n.Get("InitFlagAgent"))
	initCmd.Flags().StringVar(&opts.skills, "skills", "", i18n.Get("InitFlagSkills"))
	initCmd.Flags().BoolVar(&opts.workspace, "workspace", false, i18n.Get("InitFlagWorkspace"))
	initCmd.Flags().BoolVar(&opts.noInteractive, "no-interactive", false, i18n.Get("InteractiveFlagNoInteractive"))

	return initCmd
}

func runInitCommand(cmd *cobra.Command, opts *commandOptions) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}
	initialized, err := isProjectInitialized(projectRoot)
	if err != nil {
		return err
	}
	if initialized && shouldRunInteractiveInit(cmd, *opts) {
		return runExistingInitCommand(cmd, projectRoot, opts)
	}
	if err := resolveInitOptions(cmd, opts); err != nil {
		if errors.Is(err, interactive.ErrCanceled) {
			return nil
		}
		return err
	}
	if err := validateInitOptions(*opts); err != nil {
		return err
	}
	if err := initializeSkillWithOptionsFromCWD(opts.locale, opts.skillsLocale, effectiveInitMode(*opts), opts.agent, opts.skills, opts.agentTotalParallelism, opts.learningMode, opts.learningScope); err != nil {
		return fmt.Errorf("%s", i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
	}
	return maybeInstallGlobalCLISkills(cmd, *opts)
}

func runExistingInitCommand(cmd *cobra.Command, projectRoot string, opts *commandOptions) error {
	action, err := resolveInteractiveExistingInit(cmd, projectRoot)
	if err != nil {
		if errors.Is(err, interactive.ErrCanceled) {
			return nil
		}
		return err
	}
	switch action {
	case initExistingInspect:
		return printExistingInitSummary(projectRoot, cmd)
	case initExistingInstallGlobalCLISkills:
		return installGlobalCLISkillsFromInit(cmd, cliskillscmd.TargetAuto)
	case initExistingReset:
		return runExistingInitReset(cmd, opts)
	default:
		return nil
	}
}

func runExistingInitReset(cmd *cobra.Command, opts *commandOptions) error {
	if err := resolveInitOptions(cmd, opts); err != nil {
		if errors.Is(err, interactive.ErrCanceled) {
			return nil
		}
		return err
	}
	if err := validateInitOptions(*opts); err != nil {
		return err
	}
	if err := resetSkillWithOptions(opts.locale, opts.skillsLocale, effectiveInitMode(*opts), opts.agent, opts.skills, opts.agentTotalParallelism, opts.learningMode, opts.learningScope); err != nil {
		return fmt.Errorf("%s", i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
	}
	return maybeInstallGlobalCLISkills(cmd, *opts)
}

func resolveInitOptions(cmd *cobra.Command, opts *commandOptions) error {
	if !shouldRunInteractiveInit(cmd, *opts) {
		return nil
	}
	resolved, err := resolveInteractiveInit(cmd, *opts)
	if err != nil {
		return err
	}
	*opts = resolved
	return nil
}

func validateInitOptions(opts commandOptions) error {
	if !isValidLocale(opts.locale) {
		return fmt.Errorf("%s", i18n.Get("InitLocaleInvalid"))
	}
	if !isValidLocale(opts.skillsLocale) {
		return fmt.Errorf("%s", i18n.Get("InitLocaleInvalid"))
	}
	return nil
}

func effectiveInitMode(opts commandOptions) string {
	if opts.workspace {
		return domain.ModeWorkspace
	}
	return opts.mode
}

func maybeInstallGlobalCLISkills(cmd *cobra.Command, opts commandOptions) error {
	if !opts.installGlobalCLISkills {
		return nil
	}
	return installGlobalCLISkillsFromInit(cmd, opts.globalCLISkillsTarget)
}

func installGlobalCLISkillsFromInit(cmd *cobra.Command, target string) error {
	results, err := cliskillscmd.InstallGlobal(target)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGlobalCLISkillsFailed"), err)
	}
	return cliskillscmd.PrintInstallResults(cmd.OutOrStdout(), results)
}

func printExistingInitSummary(projectRoot string, cmd *cobra.Command) error {
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configPath := filepath.Join(seedPath, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		return err
	}
	configRepo, err := config.NewRepository(seedPath, "")
	if err != nil {
		return err
	}
	cfg := configRepo.Get()
	interactive.PrintSummary(cmd.OutOrStdout(), i18n.Get("InteractiveInitExistingSummaryTitle"), []interactive.SummaryItem{
		{Label: i18n.Get("InteractiveInitSummaryMode"), Value: localizedInitMode(cfg.Project.Mode)},
		{Label: i18n.Get("InteractiveInitSummaryToolLocale"), Value: configRepo.GetToolLocale()},
		{Label: i18n.Get("InteractiveInitSummarySkillsLocale"), Value: configRepo.GetSkillsLocale()},
		{Label: i18n.Get("InteractiveInitSummaryAgent"), Value: cfg.Agent.Engine},
		{Label: i18n.Get("InteractiveInitSummarySkills"), Value: cfg.Skills.Target},
		{Label: "config", Value: filepath.Join(".skills-seed", "config.yaml")},
	})
	return nil
}

func isValidLocale(locale string) bool {
	return config.IsSupportedLocale(locale)
}

// ResetCmd 返回 reset 命令
func ResetCmd() *cobra.Command {
	opts := commandOptions{mode: domain.ModeProject}
	resetCmd := &cobra.Command{
		Use:     "reset",
		Short:   i18n.Get("ResetShort"),
		Long:    i18n.Get("ResetLongDesc"),
		Example: i18n.Get("ResetExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isValidLocale(opts.locale) {
				return fmt.Errorf("%s", i18n.Get("InitLocaleInvalid"))
			}
			if !isValidLocale(opts.skillsLocale) {
				return fmt.Errorf("%s", i18n.Get("InitLocaleInvalid"))
			}
			effectiveMode := opts.mode
			if opts.workspace {
				effectiveMode = domain.ModeWorkspace
			}
			if err := resetSkill(opts.locale, opts.skillsLocale, effectiveMode); err != nil {
				return fmt.Errorf("%s", i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
			}
			return nil
		},
	}
	resetCmd.Flags().StringVarP(&opts.locale, "locale", "l", "", i18n.Get("InitFlagLocale"))
	resetCmd.Flags().StringVar(&opts.skillsLocale, "skills-locale", "", i18n.Get("InitFlagSkillsLocale"))
	resetCmd.Flags().StringVar(&opts.mode, "mode", domain.ModeProject, i18n.Get("InitFlagMode"))
	resetCmd.Flags().BoolVar(&opts.workspace, "workspace", false, i18n.Get("InitFlagWorkspace"))
	return resetCmd
}

func initializeSkillWithOptionsFromCWD(locale, skillsLocale, mode, agentEngine, skillsTarget string, agentTotalParallelism int, learningMode config.LearningMode, learningScope config.LearningScope) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}
	return initializeSkillWithOptions(projectRoot, locale, mode, initializeSkillOptions{
		initLogger:            true,
		showUserSummary:       true,
		agentEngine:           agentEngine,
		skillsTarget:          skillsTarget,
		skillsLocale:          skillsLocale,
		agentTotalParallelism: agentTotalParallelism,
		learningMode:          learningMode,
		learningScope:         learningScope,
	})
}

type initializeSkillOptions struct {
	initLogger            bool
	showUserSummary       bool
	language              string
	agentEngine           string
	skillsTarget          string
	skillsLocale          string
	agentTotalParallelism int
	learningMode          config.LearningMode
	learningScope         config.LearningScope
}

func initializeSkillWithOptions(projectRoot, locale, mode string, opts initializeSkillOptions) error {
	mode = normalizeInitMode(mode)
	seedPath := filepath.Join(projectRoot, ".skills-seed")

	if err := initializeI18nForInit(locale); err != nil {
		return err
	}
	if err := ensureInitProjectRoot(projectRoot, seedPath); err != nil {
		return err
	}

	seedCreated := false
	initSucceeded := false
	defer func() {
		if !initSucceeded && seedCreated {
			_ = os.RemoveAll(seedPath)
		}
	}()

	created, err := createInitSeedDirectories(seedPath)
	seedCreated = created
	if err != nil {
		return err
	}
	configRepo, err := config.NewRepository(seedPath, locale)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitCreateConfigFailed"), err)
	}
	if err := syncInitLocale(configRepo, locale); err != nil {
		return err
	}
	closeLogger, err := startInitLogger(seedPath, configRepo, opts.initLogger)
	if err != nil {
		return err
	}
	defer closeLogger()

	if opts.showUserSummary {
		logger.Info(i18n.Get("InitStart"))
	}

	projectName, err := configureInitializedProject(projectRoot, mode, configRepo, opts)
	if err != nil {
		return err
	}
	if err := ensureInitializedProjectContext(projectRoot, seedPath, projectName, configRepo); err != nil {
		logger.Error(i18n.Get("InitCreateProjectContextFailed"), "error", err)
		return err
	}
	if mode == domain.ModeWorkspace {
		if err := configureInitializedWorkspace(projectRoot, seedPath, projectName, configRepo, opts); err != nil {
			logger.Error(i18n.Get("InitCreateProjectContextFailed"), "error", err)
			return err
		}
	}
	if err := saveInitialRuntimeState(seedPath, mode); err != nil {
		return err
	}
	if mode == domain.ModeWorkspace {
		if err := initializeWorkspaceChildrenWithRepo(projectRoot, locale, configRepo); err != nil {
			return err
		}
	}

	if opts.showUserSummary {
		logger.Info(i18n.GetWithParams("InitSuccess", map[string]interface{}{"Path": relativeSeedPath(projectRoot, seedPath)}))
		logger.Info(i18n.GetWithParams("InitDocumentation", map[string]interface{}{"URL": versionedReadmeURL()}))
	}

	initSucceeded = true
	return nil
}

func initializeI18nForInit(locale string) error {
	if locale == "" {
		return nil
	}
	if err := i18n.Init(locale); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
	}
	return nil
}

func ensureInitProjectRoot(projectRoot, seedPath string) error {
	gitDir := filepath.Join(projectRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.GetWithParams("ErrInitNotGitRepo", map[string]interface{}{"Path": projectRoot}))
	}
	if _, err := os.Stat(seedPath); err == nil {
		return fmt.Errorf("%s", i18n.Get("ErrInitAlreadyInitialized"))
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func createInitSeedDirectories(seedPath string) (bool, error) {
	seedLayout := layout.New(seedPath)
	dirs := []string{
		seedPath,
		seedLayout.StoreDocuments(),
		seedLayout.Cache(),
		seedLayout.Runtime(),
	}

	seedCreated := false
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return seedCreated, fmt.Errorf("%s: %w", i18n.GetWithParams("InitCreateDirFailed", map[string]interface{}{"Path": dir}), err)
		}
		if dir == seedPath {
			seedCreated = true
		}
	}
	return seedCreated, nil
}

func syncInitLocale(configRepo *config.Repository, locale string) error {
	if locale != "" {
		if configRepo.GetToolLocale() != locale {
			if err := configRepo.SetLocale(locale); err != nil {
				return fmt.Errorf("%s: %w", i18n.Get("InitSetLocaleFailed"), err)
			}
		}
		if err := i18n.Init(locale); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
		}
		return nil
	}
	if err := i18n.Init(configRepo.GetToolLocale()); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
	}
	return nil
}

func startInitLogger(seedPath string, configRepo *config.Repository, enabled bool) (func(), error) {
	if !enabled {
		return func() {}, nil
	}
	loggingConfig := configRepo.GetLoggingConfig()
	logDir := filepath.Join(seedPath, loggingConfig.LogsPath)
	logLevel := logger.ParseLevel(loggingConfig.Level)
	if err := logger.InitWithRetention(logDir, "init", logLevel, loggingConfig.MaxLogFiles); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("InitLoggerInitFailed"), err)
	}
	return func() { _ = logger.Close() }, nil
}

func configureInitializedProject(projectRoot, mode string, configRepo *config.Repository, opts initializeSkillOptions) (string, error) {
	projectName := filepath.Base(projectRoot)
	if err := configRepo.SetProjectMode(mode); err != nil {
		return "", err
	}
	if err := configRepo.SetProjectName(projectName); err != nil {
		logger.Error(i18n.Get("InitSetProjectNameFailed"), "error", err)
		return "", err
	}
	if err := setInitializedProjectSkillsPaths(configRepo, projectName, mode); err != nil {
		return "", err
	}
	if err := configRepo.SetRootPath(projectRoot); err != nil {
		logger.Error(i18n.Get("InitSetRootPathFailed"), "error", err)
		return "", err
	}
	if gitRemote := gitOriginRemote(projectRoot); gitRemote != "" {
		if err := configRepo.SetGitRemote(gitRemote); err != nil {
			return "", err
		}
	}
	if err := configureInitializedProjectLanguage(projectRoot, mode, configRepo, opts.language); err != nil {
		return "", err
	}
	if err := applyInitAgentAndSkillsOptions(configRepo, opts, projectName, mode); err != nil {
		return "", err
	}
	if err := applyInitLearningOptions(configRepo, opts, mode); err != nil {
		return "", err
	}
	return projectName, nil
}

func configureInitializedProjectLanguage(projectRoot, mode string, configRepo *config.Repository, language string) error {
	if language != "" {
		return configRepo.SetProjectLanguage(language)
	}
	if mode == domain.ModeWorkspace {
		return configRepo.SetProjectLanguage("")
	}
	if _, detectedLanguage, ok := workspacediscovery.DetectProjectKindAndLanguage(projectRoot); ok && detectedLanguage != "" && detectedLanguage != "unknown" {
		return configRepo.SetProjectLanguage(detectedLanguage)
	}
	return nil
}

func applyInitAgentAndSkillsOptions(configRepo *config.Repository, opts initializeSkillOptions, projectName, mode string) error {
	if opts.agentEngine != "" {
		cfg := configRepo.Get()
		cfg.Agent.Engine = opts.agentEngine
		if cfg.Agent.Commands == nil {
			cfg.Agent.Commands = map[string]string{}
		}
		if cfg.Agent.Commands[opts.agentEngine] == "" {
			cfg.Agent.Commands[opts.agentEngine] = opts.agentEngine
		}
		if cfg.Skills.Target == "" {
			cfg.Skills.Target = opts.agentEngine
		}
		if cfg.Skills.Paths == nil {
			cfg.Skills.Paths = map[string]string{}
		}
		if cfg.Skills.Paths[cfg.Skills.Target] == "" {
			cfg.Skills.Paths[cfg.Skills.Target] = config.DefaultSkillsPathForTarget(cfg.Skills.Target)
		}
		if err := configRepo.Update(cfg); err != nil {
			return err
		}
	}
	if opts.skillsTarget != "" {
		cfg := configRepo.Get()
		ensureSkillsTarget(cfg, opts.skillsTarget, projectName, mode)
		if err := configRepo.Update(cfg); err != nil {
			return err
		}
	}
	if opts.skillsLocale != "" {
		return configRepo.SetSkillsLocale(opts.skillsLocale)
	}
	return nil
}

func applyInitLearningOptions(configRepo *config.Repository, opts initializeSkillOptions, mode string) error {
	if opts.learningMode != "" || opts.learningScope != "" {
		cfg := configRepo.Get()
		if opts.learningMode != "" {
			cfg.Learning.Current.Mode = config.NormalizeLearningMode(string(opts.learningMode))
		}
		if opts.learningScope != "" {
			cfg.Learning.Current.Scope = config.NormalizeLearningScope(string(opts.learningScope))
		}
		if err := configRepo.Update(cfg); err != nil {
			return err
		}
	}
	if opts.agentTotalParallelism > 0 && mode == domain.ModeProject {
		cfg := configRepo.Get()
		cfg.Agent.Parallelism = 0
		cfg.Learning.Current.Parallelism = opts.agentTotalParallelism
		return configRepo.Update(cfg)
	}
	return nil
}

func ensureInitializedProjectContext(projectRoot, seedPath, projectName string, configRepo *config.Repository) error {
	projectLanguage := configRepo.Get().Project.Language
	if projectLanguage == "" {
		projectLanguage = "unknown"
	}

	analyzerSvc := analyzer.NewAnalyzerService(nil, configRepo)
	structure, _ := analyzerSvc.GetProjectStructure(projectRoot)
	mainFiles := analyzerSvc.FindMainFiles(projectRoot)

	return prompts.EnsureProjectContext(seedPath, prompts.ProjectContextData{
		ProjectName: projectName,
		Language:    projectLanguage,
		ProjectRoot: projectRoot,
		Structure:   structure,
		MainFiles:   mainFiles,
		Locale:      configRepo.GetToolLocale(),
	})
}

func configureInitializedWorkspace(projectRoot, seedPath, projectName string, configRepo *config.Repository, opts initializeSkillOptions) error {
	projects := workspacediscovery.DiscoverProjects(projectRoot)
	workspaceConfig := configRepo.GetWorkspaceConfig()
	workspaceConfig.Projects = projects
	if err := configRepo.SetWorkspaceConfig(workspaceConfig); err != nil {
		return err
	}
	if opts.agentTotalParallelism > 0 {
		workspaceParallelism, unitParallelism := allocateWorkspaceParallelism(opts.agentTotalParallelism, len(projects))
		cfg := configRepo.Get()
		cfg.Agent.Parallelism = workspaceParallelism
		cfg.Learning.Current.Parallelism = unitParallelism
		if err := configRepo.Update(cfg); err != nil {
			return err
		}
	}
	return ensureWorkspaceContextFiles(seedPath, projectRoot, projectName, configRepo)
}

func saveInitialRuntimeState(seedPath, mode string) error {
	stateRepo := statestore.NewRepository(seedPath)
	return stateRepo.Save(context.Background(), &domain.RuntimeState{
		Mode:       mode,
		ModeLocked: false,
		UpdatedAt:  time.Now().Format(time.RFC3339),
	})
}

func gitOriginRemote(projectRoot string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}
	return gitOriginRemoteFromConfig(filepath.Join(projectRoot, ".git", "config"))
}

func gitOriginRemoteFromConfig(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	inOrigin := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if !inOrigin {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) == "url" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ensureSkillsTarget(cfg *config.Config, target string, projectName string, mode string) {
	cfg.Skills.Target = target
	if cfg.Skills.Paths == nil {
		cfg.Skills.Paths = map[string]string{}
	}
	if cfg.Skills.Paths[target] == "" {
		cfg.Skills.Paths[target] = defaultSkillsPathForProjectTarget(target, projectName, mode)
	}
}

func setInitializedProjectSkillsPaths(configRepo *config.Repository, projectName string, mode string) error {
	cfg := configRepo.Get()
	if cfg.Skills.Paths == nil {
		cfg.Skills.Paths = map[string]string{}
	}
	for _, target := range []string{"claude", "codex"} {
		cfg.Skills.Paths[target] = defaultSkillsPathForProjectTarget(target, projectName, mode)
	}
	if target := strings.TrimSpace(cfg.Skills.Target); target != "" {
		cfg.Skills.Paths[target] = defaultSkillsPathForProjectTarget(target, projectName, mode)
	}
	return configRepo.Update(cfg)
}

func defaultSkillsPathForProjectTarget(target string, projectName string, mode string) string {
	skillName := skillgen.GeneratedSkillName(projectName)
	if mode == domain.ModeWorkspace {
		skillName = skillgen.GeneratedWorkspaceSkillName(projectName)
	}
	return skillsPathForTargetAndName(target, skillName)
}

func skillsPathForTargetAndName(target string, skillName string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "claude":
		return filepath.ToSlash(filepath.Join(".claude", "skills", skillName))
	case "codex":
		return filepath.ToSlash(filepath.Join(".agents", "skills", skillName))
	default:
		return filepath.ToSlash(filepath.Join(".skills", skillName))
	}
}

func relativeSeedPath(projectRoot, seedPath string) string {
	relPath, err := filepath.Rel(projectRoot, seedPath)
	if err != nil || relPath == "" || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." || filepath.IsAbs(relPath) {
		return seedPath
	}
	return filepath.ToSlash(relPath)
}

func allocateWorkspaceParallelism(totalParallelism, projectCount int) (workspaceParallelism, unitParallelism int) {
	if totalParallelism <= 0 {
		return 0, 1
	}
	if projectCount <= 0 {
		return 0, totalParallelism
	}
	workspaceParallelism = totalParallelism
	if workspaceParallelism > projectCount {
		workspaceParallelism = projectCount
	}
	if workspaceParallelism < 1 {
		workspaceParallelism = 1
	}
	unitParallelism = totalParallelism / workspaceParallelism
	if unitParallelism < 1 {
		unitParallelism = 1
	}
	return workspaceParallelism, unitParallelism
}

func versionedReadmeURL() string {
	version := strings.TrimSpace(metadata.ProgramVersion)
	if version == "" {
		version = "main"
	}
	return "https://github.com/silaswei-io/skills-seed/blob/" + version + "/README.md"
}

func initializeWorkspaceChildrenWithRepo(workspaceRoot, locale string, rootConfigRepo *config.Repository) error {
	configLocale := locale
	if configLocale == "" {
		configLocale = rootConfigRepo.GetToolLocale()
	}
	if err := i18n.Init(configLocale); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
	}

	if rootConfigRepo.GetProjectConfig().Mode != domain.ModeWorkspace {
		return fmt.Errorf("%s", i18n.Get("InitChildrenRequireWorkspaceMode"))
	}
	workspaceConfig := rootConfigRepo.GetWorkspaceConfig()
	if len(workspaceConfig.Projects) == 0 {
		return nil
	}

	for _, project := range workspaceConfig.Projects {
		if err := initializeWorkspaceChildAt(workspaceRoot, project, rootConfigRepo, configLocale); err != nil {
			return err
		}
	}
	logger.Info(i18n.Get("InitChildrenComplete"))
	return nil
}

// EnsureWorkspaceChildInitializedAt 初始化缺失的 workspace 子项目 seed。
// 已存在的子项目 seed 会被保留，包括配置了不同 agent 的子项目。
func EnsureWorkspaceChildInitializedAt(workspaceRoot string, project config.WorkspaceProjectConfig, rootConfigRepo *config.Repository, locale string) error {
	return initializeWorkspaceChildAt(workspaceRoot, project, rootConfigRepo, locale)
}

func initializeWorkspaceChildAt(workspaceRoot string, project config.WorkspaceProjectConfig, rootConfigRepo *config.Repository, locale string) error {
	projectRootPath := workspacediscovery.ProjectRoot(workspaceRoot, project)
	childSeedPath := filepath.Join(projectRootPath, ".skills-seed")
	childConfigPath := filepath.Join(childSeedPath, "config.yaml")

	if _, err := os.Stat(childSeedPath); err == nil {
		return reportExistingWorkspaceChild(project, childSeedPath, childConfigPath, rootConfigRepo)
	} else if !os.IsNotExist(err) {
		return err
	}

	logger.Info(i18n.GetWithParams("InitChildrenProjectInitializing", map[string]interface{}{
		"ProjectName": project.ID,
		"Path":        projectRootPath,
	}))
	if err := initializeSkillWithOptions(projectRootPath, locale, domain.ModeProject, initializeSkillOptions{
		initLogger:      false,
		showUserSummary: false,
		language:        project.Language,
		agentEngine:     rootConfigRepo.GetAgentConfig().Engine,
		skillsLocale:    rootConfigRepo.GetSkillsLocale(),
	}); err != nil {
		return err
	}
	childConfigRepo, err := config.NewRepository(childSeedPath, locale)
	if err != nil {
		return err
	}
	childConfig := childConfigRepo.Get()
	childConfig.Agent = rootConfigRepo.GetAgentConfig()
	childConfig.Agent.Parallelism = 0
	inheritWorkspaceChildSkills(childConfig, rootConfigRepo.GetSkillsConfig(), rootConfigRepo.GetSkillsLocale())
	childConfig.Learning.Current = rootConfigRepo.GetCurrentLearningConfig()
	if err := childConfigRepo.Update(childConfig); err != nil {
		return err
	}
	logger.Info(i18n.GetWithParams("InitChildrenProjectInitialized", map[string]interface{}{
		"ProjectName": project.ID,
		"Path":        projectRootPath,
	}))
	return nil
}

func inheritWorkspaceChildSkills(childConfig *config.Config, rootSkills config.SkillsConfig, rootSkillsLocale string) {
	childConfig.Skills.Target = rootSkills.Target
	childConfig.Skills.Locale = rootSkillsLocale
	if childConfig.Skills.Paths == nil {
		childConfig.Skills.Paths = map[string]string{}
	}
	if rootSkills.Paths != nil {
		for target := range rootSkills.Paths {
			if childConfig.Skills.Paths[target] == "" {
				childConfig.Skills.Paths[target] = defaultSkillsPathForProjectTarget(target, childConfig.Project.Name, domain.ModeProject)
			}
		}
	}
	if childConfig.Skills.Target != "" && childConfig.Skills.Paths[childConfig.Skills.Target] == "" {
		childConfig.Skills.Paths[childConfig.Skills.Target] = defaultSkillsPathForProjectTarget(childConfig.Skills.Target, childConfig.Project.Name, domain.ModeProject)
	}
}

func reportExistingWorkspaceChild(project config.WorkspaceProjectConfig, childSeedPath, childConfigPath string, rootConfigRepo *config.Repository) error {
	if _, err := os.Stat(childConfigPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s", i18n.GetWithParams("InitChildrenProjectSeedWithoutConfig", map[string]interface{}{
				"ProjectName": project.ID,
				"Path":        childSeedPath,
			}))
		}
		return err
	}

	childConfigRepo, err := config.NewRepository(childSeedPath, rootConfigRepo.GetToolLocale())
	if err != nil {
		return err
	}
	rootAgent := rootConfigRepo.GetAgentConfig().Engine
	childAgent := childConfigRepo.GetAgentConfig().Engine
	params := map[string]interface{}{
		"ProjectName": project.ID,
		"Path":        childSeedPath,
		"RootAgent":   rootAgent,
		"ChildAgent":  childAgent,
	}
	if rootAgent == childAgent {
		logger.Info(i18n.GetWithParams("InitChildrenProjectAlreadyInitializedSameAgent", params))
		return nil
	}
	logger.Warn(i18n.GetWithParams("InitChildrenProjectAlreadyInitializedDifferentAgent", params))
	return nil
}

func resetSkill(locale, skillsLocale, mode string) error {
	return resetSkillWithOptions(locale, skillsLocale, mode, "", "", 0, "", "")
}

func resetSkillWithOptions(locale, skillsLocale, mode, agentEngine, skillsTarget string, agentTotalParallelism int, learningMode config.LearningMode, learningScope config.LearningScope) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}

	seedPath := filepath.Join(projectRoot, ".skills-seed")
	if _, err := os.Stat(seedPath); err == nil {
		backupRoot := filepath.Join(projectRoot, ".skills-seed.backup")
		if err := os.MkdirAll(backupRoot, 0755); err != nil {
			return err
		}
		backupPath := filepath.Join(backupRoot, time.Now().Format("20060102-150405"))
		if err := os.Rename(seedPath, backupPath); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("ResetBackupFailed"), err)
		}
	}
	return initializeSkillWithOptions(projectRoot, locale, mode, initializeSkillOptions{
		initLogger:            true,
		showUserSummary:       true,
		agentEngine:           agentEngine,
		skillsTarget:          skillsTarget,
		skillsLocale:          skillsLocale,
		agentTotalParallelism: agentTotalParallelism,
		learningMode:          learningMode,
		learningScope:         learningScope,
	})
}

func normalizeInitMode(mode string) string {
	switch mode {
	case "", domain.ModeProject:
		return domain.ModeProject
	case domain.ModeWorkspace:
		return domain.ModeWorkspace
	default:
		return domain.ModeProject
	}
}

func ensureWorkspaceContextFiles(seedPath, projectRoot, projectName string, configRepo *config.Repository) error {
	workspaceConfig := configRepo.GetWorkspaceConfig()
	projects := make([]prompts.WorkspaceContextProject, 0, len(workspaceConfig.Projects))
	for _, project := range workspaceConfig.Projects {
		projects = append(projects, prompts.WorkspaceContextProject{
			ID:       project.ID,
			Path:     project.Path,
			Type:     project.Type,
			Language: project.Language,
		})
	}

	return prompts.EnsureWorkspaceContext(seedPath, prompts.WorkspaceContextData{
		WorkspaceName: projectName,
		WorkspaceRoot: projectRoot,
		Projects:      projects,
		Locale:        configRepo.GetToolLocale(),
	})
}
