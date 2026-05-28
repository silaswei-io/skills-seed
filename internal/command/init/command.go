package initcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	localeFlag    string // locale 参数
	modeFlag      string // mode 参数
	agentFlag     string // agent provider 参数
	workspaceFlag bool   // workspace 快捷参数
	childrenFlag  bool   // 初始化 workspace 子项目
)

// Cmd 返回 init 命令
func Cmd() *cobra.Command {
	initCmd := &cobra.Command{
		Use:     "init",
		Short:   i18n.Get("InitShort"),
		Long:    i18n.Get("InitLongDesc"),
		Example: i18n.Get("InitExample"),
		Run: func(cmd *cobra.Command, args []string) {
			// 验证 locale 参数
			if !isValidLocale(localeFlag) {
				fmt.Fprintln(os.Stderr, i18n.Get("InitLocaleInvalid"))
				os.Exit(1)
			}

			effectiveMode := modeFlag
			if workspaceFlag {
				effectiveMode = domain.ModeWorkspace
			}

			if err := initializeSkillWithWorkspaceChildren(localeFlag, effectiveMode, agentFlag, childrenFlag); err != nil {
				// 错误信息直接输出（此时 logger 可能未初始化）
				fmt.Fprintln(os.Stderr, i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
				os.Exit(1)
			}
		},
	}

	// 添加 --locale 参数
	initCmd.Flags().StringVarP(&localeFlag, "locale", "l", "", i18n.Get("InitFlagLocale"))
	initCmd.Flags().StringVar(&modeFlag, "mode", domain.ModeProject, i18n.Get("InitFlagMode"))
	initCmd.Flags().StringVar(&agentFlag, "agent", "", i18n.Get("InitFlagAgent"))
	initCmd.Flags().BoolVar(&workspaceFlag, "workspace", false, i18n.Get("InitFlagWorkspace"))
	initCmd.Flags().BoolVar(&childrenFlag, "children", false, i18n.Get("InitFlagChildren"))
	initCmd.AddCommand(childrenCmd())

	return initCmd
}

func childrenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "children",
		Short:   i18n.Get("InitChildrenShort"),
		Long:    i18n.Get("InitChildrenLongDesc"),
		Example: i18n.Get("InitChildrenExample"),
		Run: func(cmd *cobra.Command, args []string) {
			if !isValidLocale(localeFlag) {
				fmt.Fprintln(os.Stderr, i18n.Get("InitLocaleInvalid"))
				os.Exit(1)
			}
			if err := initializeWorkspaceChildren(localeFlag); err != nil {
				fmt.Fprintln(os.Stderr, i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&localeFlag, "locale", "l", "", i18n.Get("InitFlagLocale"))
	return cmd
}

func isValidLocale(locale string) bool {
	return locale == "" || locale == "zh-CN" || locale == "en-US"
}

// ResetCmd 返回 reset 命令
func ResetCmd() *cobra.Command {
	resetCmd := &cobra.Command{
		Use:     "reset",
		Short:   i18n.Get("ResetShort"),
		Long:    i18n.Get("ResetLongDesc"),
		Example: i18n.Get("ResetExample"),
		Run: func(cmd *cobra.Command, args []string) {
			effectiveMode := modeFlag
			if workspaceFlag {
				effectiveMode = domain.ModeWorkspace
			}
			if err := resetSkill(localeFlag, effectiveMode); err != nil {
				fmt.Fprintln(os.Stderr, i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
				os.Exit(1)
			}
		},
	}
	resetCmd.Flags().StringVarP(&localeFlag, "locale", "l", "", i18n.Get("InitFlagLocale"))
	resetCmd.Flags().StringVar(&modeFlag, "mode", domain.ModeProject, i18n.Get("InitFlagMode"))
	resetCmd.Flags().BoolVar(&workspaceFlag, "workspace", false, i18n.Get("InitFlagWorkspace"))
	return resetCmd
}

func initializeSkill(locale, mode string) error {
	return initializeSkillWithWorkspaceChildren(locale, mode, "", false)
}

func initializeSkillWithWorkspaceChildren(locale, mode, agentProvider string, initWorkspaceChildren bool) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}
	return initializeSkillWithOptions(projectRoot, locale, mode, initializeSkillOptions{
		initLogger:            true,
		showUserSummary:       true,
		agentProvider:         agentProvider,
		initWorkspaceChildren: initWorkspaceChildren,
	})
}

type initializeSkillOptions struct {
	initLogger            bool
	showUserSummary       bool
	language              string
	agentProvider         string
	initWorkspaceChildren bool
}

func initializeSkillAt(projectRoot, locale, mode string) error {
	return initializeSkillWithOptions(projectRoot, locale, mode, initializeSkillOptions{
		initLogger:      true,
		showUserSummary: true,
	})
}

func initializeSkillWithOptions(projectRoot, locale, mode string, opts initializeSkillOptions) error {
	mode = normalizeInitMode(mode)
	// 如果指定了 locale，先初始化 i18n
	if locale != "" {
		if err := i18n.Init(locale); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
		}
	}

	// 检查是否是 Git 仓库
	gitDir := filepath.Join(projectRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.GetWithParams("ErrInitNotGitRepo", map[string]interface{}{"Path": projectRoot}))
	}

	// 检查是否已经初始化
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	if _, err := os.Stat(seedPath); err == nil {
		return fmt.Errorf("%s", i18n.Get("ErrInitAlreadyInitialized"))
	}

	// 创建目录结构
	dirs := []string{
		seedPath,
		filepath.Join(seedPath, "memory"),
		filepath.Join(seedPath, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("%s: %w", i18n.GetWithParams("InitCreateDirFailed", map[string]interface{}{"Path": dir}), err)
		}
	}

	// 生成配置
	configRepo, err := config.NewRepository(seedPath, locale)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitCreateConfigFailed"), err)
	}

	// 如果指定了 locale 但配置还未初始化，确保 i18n 使用正确的语言
	if locale != "" {
		configLocale := configRepo.Get().Project.Locale
		if configLocale != locale {
			// 更新配置中的 locale
			if err := configRepo.SetLocale(locale); err != nil {
				return fmt.Errorf("%s: %w", i18n.Get("InitSetLocaleFailed"), err)
			}
		}
		// 重新初始化 i18n 以使用正确的语言
		if err := i18n.Init(locale); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
		}
	} else {
		// 从配置中读取 locale 并初始化 i18n
		configLocale := configRepo.Get().Project.Locale
		if configLocale == "" {
			configLocale = domain.DefaultLocale
		}
		if err := i18n.Init(configLocale); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
		}
	}

	if opts.initLogger {
		loggingConfig := configRepo.GetLoggingConfig()
		logDir := filepath.Join(seedPath, loggingConfig.LogsPath)
		logLevel := logger.ParseLevel(loggingConfig.Level)

		if err := logger.InitWithRetention(logDir, "init", logLevel, loggingConfig.MaxLogFiles); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("InitLoggerInitFailed"), err)
		}
		defer logger.Close()
	}

	if opts.showUserSummary {
		logger.Info(i18n.Get("InitStart"))
	}

	// 获取项目名称（从目录名）
	projectName := filepath.Base(projectRoot)
	if err := configRepo.SetProjectMode(mode); err != nil {
		return err
	}
	if err := configRepo.SetProjectName(projectName); err != nil {
		logger.Error(i18n.Get("InitSetProjectNameFailed"), "error", err)
		return err
	}

	if err := configRepo.SetRootPath(projectRoot); err != nil {
		logger.Error(i18n.Get("InitSetRootPathFailed"), "error", err)
		return err
	}
	if opts.language != "" {
		if err := configRepo.SetProjectLanguage(opts.language); err != nil {
			return err
		}
	}
	if opts.agentProvider != "" {
		cfg := configRepo.Get()
		cfg.Agent.Provider = opts.agentProvider
		if cfg.Agent.Commands == nil {
			cfg.Agent.Commands = map[string]string{}
		}
		if cfg.Agent.Commands[opts.agentProvider] == "" {
			cfg.Agent.Commands[opts.agentProvider] = opts.agentProvider
		}
		if cfg.Output.SkillsPaths == nil {
			cfg.Output.SkillsPaths = map[string]string{}
		}
		if cfg.Output.SkillsPaths[opts.agentProvider] == "" {
			cfg.Output.SkillsPaths[opts.agentProvider] = config.DefaultSkillsPathForProvider(opts.agentProvider)
		}
		if err := configRepo.Update(cfg); err != nil {
			return err
		}
	}

	projectLanguage := configRepo.Get().Project.Language
	if projectLanguage == "" {
		projectLanguage = "go"
	}

	analyzerSvc := analyzer.NewAnalyzerService(nil, configRepo)
	structure, _ := analyzerSvc.GetProjectStructure(projectRoot)
	mainFiles := analyzerSvc.FindMainFiles(projectRoot)

	promptData := prompts.ProjectPromptData{
		ProjectName: projectName,
		Language:    projectLanguage,
		ProjectRoot: projectRoot,
		Structure:   structure,
		MainFiles:   mainFiles,
		Locale:      configRepo.Get().Project.Locale,
	}
	if err := prompts.EnsureProjectPrompts(seedPath, promptData); err != nil {
		logger.Error(i18n.Get("InitCreateProjectPromptsFailed"), "error", err)
		return err
	}

	if mode == domain.ModeWorkspace {
		projects := workspacediscovery.DiscoverProjects(projectRoot)
		workspaceConfig := configRepo.GetWorkspaceConfig()
		if len(workspaceConfig.Projects) == 0 {
			workspaceConfig.Projects = projects
			if err := configRepo.SetWorkspaceConfig(workspaceConfig); err != nil {
				return err
			}
		}
		if err := ensureWorkspacePromptFiles(seedPath, projectRoot, projectName, configRepo); err != nil {
			logger.Error(i18n.Get("InitCreateProjectPromptsFailed"), "error", err)
			return err
		}
	}

	stateRepo := statestore.NewRepository(seedPath)
	if err := stateRepo.Save(context.Background(), &domain.RuntimeState{
		Mode:       mode,
		ModeLocked: false,
		UpdatedAt:  time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	if mode == domain.ModeWorkspace && opts.initWorkspaceChildren {
		if err := initializeWorkspaceChildrenWithRepo(projectRoot, locale, configRepo); err != nil {
			return err
		}
	}

	if opts.showUserSummary {
		logger.Info(i18n.GetWithParams("InitSuccess", map[string]interface{}{"Path": relativeSeedPath(projectRoot, seedPath)}))
		logger.Info(i18n.GetWithParams("InitDocumentation", map[string]interface{}{"URL": versionedReadmeURL()}))
	}

	return nil
}

func relativeSeedPath(projectRoot, seedPath string) string {
	relPath, err := filepath.Rel(projectRoot, seedPath)
	if err != nil || relPath == "" || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." || filepath.IsAbs(relPath) {
		return seedPath
	}
	return filepath.ToSlash(relPath)
}

func versionedReadmeURL() string {
	version := strings.TrimSpace(metadata.ProgramVersion)
	if version == "" {
		version = "main"
	}
	return "https://github.com/silaswei-io/skills-seed/blob/" + version + "/README.md"
}

func initializeWorkspaceChildren(locale string) error {
	workspaceRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
	}
	return initializeWorkspaceChildrenAt(workspaceRoot, locale)
}

func initializeWorkspaceChildrenAt(workspaceRoot, locale string) error {
	rootSeedPath := filepath.Join(workspaceRoot, ".skills-seed")
	rootConfigPath := filepath.Join(rootSeedPath, "config.yaml")
	if _, err := os.Stat(rootConfigPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
		}
		return err
	}

	rootConfigRepo, err := config.NewRepository(rootSeedPath, locale)
	if err != nil {
		return err
	}
	return initializeWorkspaceChildrenWithRepo(workspaceRoot, locale, rootConfigRepo)
}

func initializeWorkspaceChildrenWithRepo(workspaceRoot, locale string, rootConfigRepo *config.Repository) error {
	configLocale := locale
	if configLocale == "" {
		configLocale = rootConfigRepo.GetProjectConfig().Locale
	}
	if configLocale == "" {
		configLocale = domain.DefaultLocale
	}
	if err := i18n.Init(configLocale); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
	}

	if rootConfigRepo.GetProjectConfig().Mode != domain.ModeWorkspace {
		return fmt.Errorf("%s", i18n.Get("InitChildrenRequireWorkspaceMode"))
	}
	workspaceConfig := rootConfigRepo.GetWorkspaceConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}

	for _, project := range workspaceConfig.Projects {
		if err := initializeWorkspaceChildAt(workspaceRoot, project, rootConfigRepo, configLocale); err != nil {
			return err
		}
	}
	logger.Info(i18n.Get("InitChildrenComplete"))
	return nil
}

// EnsureWorkspaceChildInitializedAt initializes a missing workspace child seed and
// preserves existing child seeds, including children configured with a different agent.
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
		agentProvider:   rootConfigRepo.GetAgentConfig().Provider,
	}); err != nil {
		return err
	}
	childConfigRepo, err := config.NewRepository(childSeedPath, locale)
	if err != nil {
		return err
	}
	childConfig := childConfigRepo.Get()
	childConfig.Agent = rootConfigRepo.GetAgentConfig()
	childConfig.Output = rootConfigRepo.GetOutputConfig()
	if err := childConfigRepo.Update(childConfig); err != nil {
		return err
	}
	logger.Info(i18n.GetWithParams("InitChildrenProjectInitialized", map[string]interface{}{
		"ProjectName": project.ID,
		"Path":        projectRootPath,
	}))
	return nil
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

	childConfigRepo, err := config.NewRepository(childSeedPath, rootConfigRepo.GetProjectConfig().Locale)
	if err != nil {
		return err
	}
	rootAgent := rootConfigRepo.GetAgentConfig().Provider
	childAgent := childConfigRepo.GetAgentConfig().Provider
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

func resetSkill(locale, mode string) error {
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
	return initializeSkillAt(projectRoot, locale, mode)
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

func ensureWorkspacePromptFiles(seedPath, projectRoot, projectName string, configRepo *config.Repository) error {
	workspaceConfig := configRepo.GetWorkspaceConfig()
	projects := make([]prompts.WorkspacePromptProject, 0, len(workspaceConfig.Projects))
	for _, project := range workspaceConfig.Projects {
		projects = append(projects, prompts.WorkspacePromptProject{
			ID:       project.ID,
			Path:     project.Path,
			Type:     project.Type,
			Language: project.Language,
		})
	}

	return prompts.EnsureWorkspacePrompts(seedPath, prompts.WorkspacePromptData{
		WorkspaceName: projectName,
		WorkspaceRoot: projectRoot,
		Projects:      projects,
		Locale:        configRepo.GetProjectConfig().Locale,
	})
}
