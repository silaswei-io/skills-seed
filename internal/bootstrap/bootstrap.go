package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/check"
	"github.com/silaswei-io/skills-seed/internal/command/generate"
	"github.com/silaswei-io/skills-seed/internal/command/hook"
	initcmd "github.com/silaswei-io/skills-seed/internal/command/init"
	"github.com/silaswei-io/skills-seed/internal/command/learn"
	patternscmd "github.com/silaswei-io/skills-seed/internal/command/patterns"
	profilecmd "github.com/silaswei-io/skills-seed/internal/command/profile"
	"github.com/silaswei-io/skills-seed/internal/command/view"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/utils"
	"github.com/spf13/cobra"
)

// Run 初始化运行时依赖并执行命令行入口
func Run() error {
	seedPath, err := utils.GetSeedPath()
	hasSeedDir := err == nil && seedPath != ""

	if err := initI18n(seedPath, hasSeedDir); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", i18n.Get("BootstrapI18nInitWarn"), err)
	}

	rootCmd := createRootCmd()

	var cont *container.Container
	if hasSeedDir {
		cont, err = initContainerAndLogger(seedPath)
		if err != nil {
			return err
		}
		defer func() {
			agent.LogTokenUsageSummary()
			_ = cont.Close()
			_ = logger.Close()
		}()
	}

	registerCommands(rootCmd, cont)
	return rootCmd.Execute()
}

func initI18n(seedPath string, hasSeedDir bool) error {
	locale := getLocale(seedPath, hasSeedDir)
	return i18n.Init(locale)
}

func getLocale(seedPath string, hasSeedDir bool) string {
	locale := domain.DefaultLocale
	if !hasSeedDir {
		return locale
	}

	configData, err := utils.LoadConfig(seedPath)
	if err != nil || configData == nil || configData.Project.Locale == "" {
		return locale
	}

	return configData.Project.Locale
}

func initContainerAndLogger(seedPath string) (*container.Container, error) {
	cont, err := container.NewContainer(context.Background(), seedPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("InitCreateContainerFailed"), err)
	}

	if err := initLogger(seedPath, cont); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", i18n.Get("BootstrapLoggerInitWarn"), err)
	}

	return cont, nil
}

func initLogger(seedPath string, cont *container.Container) error {
	loggingConfig := cont.GetLoggingConfig()
	logDir := filepath.Join(seedPath, loggingConfig.LogsPath)
	logLevel := logger.ParseLevel(loggingConfig.Level)

	if err := logger.InitWithRetention(logDir, getCommandName(), logLevel, loggingConfig.MaxLogFiles); err != nil {
		return err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "bootstrap.init_logger",
		"seed_path", seedPath,
		"log_dir", logDir,
		"log_path", logger.CurrentLogPath(),
		"configured_level", loggingConfig.Level,
		"effective_level", logger.CurrentLevel().String(),
		"max_log_files", loggingConfig.MaxLogFiles,
	)
	return nil
}

func getCommandName() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return "skills-seed"
}

func createRootCmd() *cobra.Command {
	promptTemplatesHash := metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS))
	skillsTemplatesHash := metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS))

	cmd := &cobra.Command{
		Use:     "skills-seed",
		Short:   i18n.Get("RootShort"),
		Long:    i18n.Get("RootLong"),
		Version: metadata.ProgramVersion,
	}
	cmd.SetVersionTemplate("{{.Name}} version {{.Version}}\nprompt-templates-sha256: " + promptTemplatesHash + "\nskills-templates-sha256: " + skillsTemplatesHash + "\n")
	return cmd
}

func registerCommands(rootCmd *cobra.Command, cont *container.Container) {
	rootCmd.AddCommand(initcmd.Cmd())
	rootCmd.AddCommand(learn.Cmd(cont))
	rootCmd.AddCommand(check.Cmd(cont))
	rootCmd.AddCommand(generate.Cmd(cont))
	rootCmd.AddCommand(patternscmd.Cmd(cont))
	rootCmd.AddCommand(profilecmd.Cmd(cont))
	rootCmd.AddCommand(view.Cmd(cont))
	rootCmd.AddCommand(hook.Cmd())
}
