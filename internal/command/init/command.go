package initcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	localeFlag    string // locale 参数
	modeFlag      string // mode 参数
	workspaceFlag bool   // workspace 快捷参数
)

// Cmd 返回 init 命令
func Cmd() *cobra.Command {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: i18n.Get("InitShort"),
		Long:  i18n.Get("InitLongDesc"),
		Run: func(cmd *cobra.Command, args []string) {
			// 验证 locale 参数
			if localeFlag != "" && localeFlag != "zh-CN" && localeFlag != "en-US" {
				fmt.Fprintln(os.Stderr, i18n.Get("InitLocaleInvalid"))
				os.Exit(1)
			}

			effectiveMode := modeFlag
			if workspaceFlag {
				effectiveMode = domain.ModeWorkspace
			}

			if err := initializeSkill(localeFlag, effectiveMode); err != nil {
				// 错误信息直接输出（此时 logger 可能未初始化）
				fmt.Fprintln(os.Stderr, i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
				os.Exit(1)
			}
		},
	}

	// 添加 --locale 参数
	initCmd.Flags().StringVarP(&localeFlag, "locale", "l", "", "Configuration file language: zh-CN (Chinese) or en-US (English). Auto-detected if not specified.")
	initCmd.Flags().StringVar(&modeFlag, "mode", domain.ModeProject, i18n.Get("InitFlagMode"))
	initCmd.Flags().BoolVar(&workspaceFlag, "workspace", false, i18n.Get("InitFlagWorkspace"))

	return initCmd
}

// ResetCmd 返回 reset 命令
func ResetCmd() *cobra.Command {
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: i18n.Get("ResetShort"),
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
	resetCmd.Flags().StringVarP(&localeFlag, "locale", "l", "", "Configuration file language: zh-CN (Chinese) or en-US (English). Auto-detected if not specified.")
	resetCmd.Flags().StringVar(&modeFlag, "mode", domain.ModeProject, i18n.Get("InitFlagMode"))
	resetCmd.Flags().BoolVar(&workspaceFlag, "workspace", false, i18n.Get("InitFlagWorkspace"))
	return resetCmd
}

func initializeSkill(locale, mode string) error {
	mode = normalizeInitMode(mode)
	// 如果指定了 locale，先初始化 i18n
	if locale != "" {
		if err := i18n.Init(locale); err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("InitI18nInitFailed"), err)
		}
	}

	// 获取项目根目录
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitGetCurrentDirFailed"), err)
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

	// 初始化日志
	loggingConfig := configRepo.GetLoggingConfig()
	// 配置中的路径是相对于 .skills-seed 目录的
	logDir := filepath.Join(seedPath, loggingConfig.LogsPath)
	logLevel := logger.ParseLevel(loggingConfig.Level)

	if err := logger.InitWithRetention(logDir, "init", logLevel, loggingConfig.MaxLogFiles); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("InitLoggerInitFailed"), err)
	}
	defer logger.Close()

	logger.Info(i18n.Get("InitStart"))

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

	logger.Info(i18n.Get("InitSuccess"))
	logger.Info(i18n.GetWithParams("InitSkillLocation", map[string]interface{}{"Path": seedPath}))

	logger.Info(i18n.Get("InitNextSteps"))
	logger.Info(i18n.Get("InitStepLearn"))
	logger.Info(i18n.Get("InitStepWatch"))
	logger.Info(i18n.Get("InitStepPatterns"))
	logger.Info(i18n.Get("InitStepRules"))

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
	return initializeSkill(locale, mode)
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

		projectRootPath := filepath.Join(projectRoot, filepath.FromSlash(project.Path))
		analyzerSvc := analyzer.NewAnalyzerService(nil, configRepo)
		structure, _ := analyzerSvc.GetProjectStructure(projectRootPath)
		mainFiles := analyzerSvc.FindMainFiles(projectRootPath)
		if err := prompts.EnsureProjectPromptsAt(filepath.Join(seedPath, "prompts", "projects", project.ID), prompts.ProjectPromptData{
			ProjectName: project.ID,
			Language:    project.Language,
			ProjectRoot: projectRootPath,
			Structure:   structure,
			MainFiles:   mainFiles,
			Locale:      configRepo.GetProjectConfig().Locale,
		}); err != nil {
			return err
		}
	}

	return prompts.EnsureWorkspacePrompts(seedPath, prompts.WorkspacePromptData{
		WorkspaceName: projectName,
		WorkspaceRoot: projectRoot,
		Projects:      projects,
		Locale:        configRepo.GetProjectConfig().Locale,
	})
}
