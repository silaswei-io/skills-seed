package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/spf13/cobra"
)

var (
	localeFlag string // locale 参数
)

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

			if err := initializeSkill(localeFlag); err != nil {
				// 错误信息直接输出（此时 logger 可能未初始化）
				fmt.Fprintln(os.Stderr, i18n.GetWithParams("InitFailed", map[string]interface{}{"Error": err.Error()}))
				os.Exit(1)
			}
		},
	}

	// 添加 --locale 参数
	initCmd.Flags().StringVarP(&localeFlag, "locale", "l", "", "Configuration file language: zh-CN (Chinese) or en-US (English). Auto-detected if not specified.")

	return initCmd
}

func initializeSkill(locale string) error {
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

	if err := prompts.EnsureProjectPrompts(seedPath, prompts.ProjectPromptData{
		ProjectName: projectName,
		Language:    projectLanguage,
		ProjectRoot: projectRoot,
		Structure:   structure,
		MainFiles:   mainFiles,
		Locale:      configRepo.Get().Project.Locale,
	}); err != nil {
		logger.Error(i18n.Get("InitCreateProjectPromptsFailed"), "error", err)
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
