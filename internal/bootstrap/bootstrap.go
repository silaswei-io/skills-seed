package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/check"
	"github.com/silaswei-io/skills-seed/internal/command/generate"
	"github.com/silaswei-io/skills-seed/internal/command/hook"
	initcmd "github.com/silaswei-io/skills-seed/internal/command/init"
	"github.com/silaswei-io/skills-seed/internal/command/learn"
	logcmd "github.com/silaswei-io/skills-seed/internal/command/log"
	patternscmd "github.com/silaswei-io/skills-seed/internal/command/patterns"
	previewcmd "github.com/silaswei-io/skills-seed/internal/command/preview"
	profilecmd "github.com/silaswei-io/skills-seed/internal/command/profile"
	reviewcmd "github.com/silaswei-io/skills-seed/internal/command/review"
	synccmd "github.com/silaswei-io/skills-seed/internal/command/sync"
	workflowcmd "github.com/silaswei-io/skills-seed/internal/command/workflow"
	workspacecmd "github.com/silaswei-io/skills-seed/internal/command/workspace"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
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

	args := os.Args[1:]
	if !commandNeedsProjectRuntime(args) {
		rootCmd := createRootCmd()
		registerCommands(rootCmd, nil)
		return rootCmd.Execute()
	}

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

	rootCmd := createRootCmd()
	registerCommands(rootCmd, cont)
	return rootCmd.Execute()
}

func initI18n(seedPath string, hasSeedDir bool) error {
	locale := getLocale(seedPath, hasSeedDir)
	return i18n.Init(locale)
}

func getLocale(seedPath string, hasSeedDir bool) string {
	locale := config.DefaultToolLocale
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
		Use:          "skills-seed",
		Short:        i18n.Get("RootShort"),
		Long:         i18n.Get("RootLong"),
		Example:      i18n.Get("RootExample"),
		Version:      metadata.ProgramVersion,
		SilenceUsage: true,
	}
	configureCobraDefaults(cmd)
	cmd.SetVersionTemplate("{{.Name}} version {{.Version}}\nprompt-templates-sha256: " + promptTemplatesHash + "\nskills-templates-sha256: " + skillsTemplatesHash + "\n")
	return cmd
}

func configureCobraDefaults(rootCmd *cobra.Command) {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().BoolP("help", "h", false, i18n.Get("CobraFlagHelp"))
	rootCmd.Flags().BoolP("version", "v", false, i18n.Get("CobraFlagVersion"))
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: i18n.Get("CobraHelpShort"),
		Long:  i18n.Get("CobraHelpLong"),
		Run: func(cmd *cobra.Command, args []string) {
			target, _, err := cmd.Root().Find(args)
			if target == nil || err != nil {
				cmd.Printf("%s %#q\n", i18n.Get("CobraHelpUnknownTopic"), args)
				cobra.CheckErr(cmd.Root().Usage())
				return
			}
			target.InitDefaultHelpFlag()
			target.InitDefaultVersionFlag()
			cobra.CheckErr(target.Help())
		},
	})
}

func registerCommands(rootCmd *cobra.Command, cont *container.Container) {
	rootCmd.AddCommand(initcmd.Cmd())
	rootCmd.AddCommand(initcmd.ResetCmd())
	rootCmd.AddCommand(workspacecmd.Cmd(cont))
	rootCmd.AddCommand(synccmd.Cmd(cont))
	rootCmd.AddCommand(workflowcmd.Cmd(cont))
	rootCmd.AddCommand(learn.Cmd(cont))
	rootCmd.AddCommand(check.Cmd(cont))
	rootCmd.AddCommand(generate.Cmd(cont))
	rootCmd.AddCommand(logcmd.Cmd())
	rootCmd.AddCommand(patternscmd.Cmd(cont))
	rootCmd.AddCommand(previewcmd.Cmd(cont))
	rootCmd.AddCommand(reviewcmd.Cmd(cont))
	rootCmd.AddCommand(profilecmd.Cmd(cont))
	rootCmd.AddCommand(hook.Cmd())
}

func commandNeedsProjectRuntime(args []string) bool {
	if len(args) == 0 {
		return false
	}

	cleaned := leadingNonHelpArgs(args)
	if len(cleaned) == 0 {
		return false
	}
	if containsHelpOrVersionFlag(args) {
		return false
	}

	switch cleaned[0] {
	case "help", "init", "hook", "log":
		return false
	case "add", "check", "reset", "sync":
		return true
	case "generate":
		return len(cleaned) >= 2 && cleaned[1] == "skills"
	case "learn":
		return len(cleaned) >= 2 && (cleaned[1] == "current" || cleaned[1] == "history")
	case "patterns":
		return len(cleaned) >= 2 && (cleaned[1] == "stats" || cleaned[1] == "compact" || cleaned[1] == "add" || cleaned[1] == "delete" || cleaned[1] == "remove" || cleaned[1] == "rm" || cleaned[1] == "show")
	case "preview":
		return len(cleaned) >= 2 && cleaned[1] == "files"
	case "profile":
		return len(cleaned) >= 2 && (cleaned[1] == "show" || cleaned[1] == "refresh")
	case "review":
		return len(cleaned) >= 2 && (cleaned[1] == "import" || cleaned[1] == "stats")
	case "workflow":
		return true
	case "workspace":
		return len(cleaned) >= 2 && cleaned[1] == "add"
	default:
		return false
	}
}

func leadingNonHelpArgs(args []string) []string {
	cleaned := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		cleaned = append(cleaned, arg)
	}
	return cleaned
}

func containsHelpOrVersionFlag(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "-v", "--version":
			return true
		}
	}
	return false
}
