package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

var unresolvedI18nKeyPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*(Short|LongDesc|Example|Flag[A-Za-z0-9]*)$`)

func TestCommandHelpCoverage(t *testing.T) {
	for _, locale := range []string{"en-US", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			require.NoError(t, i18n.Init(locale))

			rootCmd := createRootCmd()
			registerCommands(rootCmd, nil)

			walkCommands(t, rootCmd, func(t *testing.T, cmd *cobra.Command) {
				if isBuiltinHelpCommand(cmd) {
					return
				}

				require.NotEmpty(t, strings.TrimSpace(cmd.Use), "command %q must define Use", commandPath(cmd))
				requireHelpText(t, "Short", commandPath(cmd), cmd.Short)
				requireHelpText(t, "Long", commandPath(cmd), cmd.Long)
				requireHelpText(t, "Example", commandPath(cmd), cmd.Example)

				cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
					requireHelpText(t, "flag --"+flag.Name, commandPath(cmd), flag.Usage)
				})
			})
		})
	}
}

func TestRootHelpUsesConciseIntro(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	rootCmd := createRootCmd()
	registerCommands(rootCmd, nil)
	rootCmd.SetArgs([]string{"help"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	require.NoError(t, rootCmd.Execute())

	helpText := out.String()
	require.Contains(t, helpText, "Usage:")
	require.Contains(t, helpText, "Available Commands:")
	require.Contains(t, helpText, "check")
	require.Contains(t, helpText, "generate")
	require.Contains(t, helpText, "sync")
	require.Contains(t, helpText, "workspace")
	require.Contains(t, helpText, "从代码库学习项目规范，并生成 AI Agent skills。")
	require.Contains(t, helpText, "skills-seed init --mode project --agent codex --skills codex --locale zh-CN")
	require.NotContains(t, helpText, "此工具可集成配置的 AI Agent")
	require.NotContains(t, helpText, "初始化当前 Git 仓库")
	require.NotContains(t, helpText, "学习当前代码并生成 skills")
}

func TestSubcommandHelpKeepsDetailedContent(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	rootCmd := createRootCmd()
	registerCommands(rootCmd, nil)
	rootCmd.SetArgs([]string{"learn", "current", "--help"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	require.NoError(t, rootCmd.Execute())

	helpText := out.String()
	require.Contains(t, helpText, "分析当前代码库，提取编码模式")
	require.Contains(t, helpText, "Examples:")
	require.Contains(t, helpText, "skills-seed learn current --focus internal/service --profile skip")
	require.Contains(t, helpText, "--context string")
}

func TestRuntimeErrorsDoNotPrintUsage(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	rootCmd := createRootCmd()
	registerCommands(rootCmd, nil)
	rootCmd.SetArgs([]string{"learn", "current"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	require.Error(t, rootCmd.Execute())

	output := out.String()
	require.NotContains(t, output, "Usage:")
	require.NotContains(t, output, "Flags:")
}

func TestProjectIndependentCommandsDoNotRequireRuntime(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "root help by empty args", args: nil, want: false},
		{name: "root help flag", args: []string{"--help"}, want: false},
		{name: "root version flag", args: []string{"--version"}, want: false},
		{name: "builtin help command", args: []string{"help"}, want: false},
		{name: "help reset", args: []string{"help", "reset"}, want: false},
		{name: "subcommand help", args: []string{"learn", "current", "--help"}, want: false},
		{name: "completion", args: []string{"completion", "bash"}, want: false},
		{name: "init", args: []string{"init"}, want: false},
		{name: "reset", args: []string{"reset"}, want: true},
		{name: "reset help positional arg", args: []string{"reset", "help"}, want: true},
		{name: "reset help flag", args: []string{"reset", "--help"}, want: false},
		{name: "hook install", args: []string{"hook", "install"}, want: false},
		{name: "unknown command", args: []string{"unknown"}, want: false},
		{name: "add", args: []string{"workspace", "add", "."}, want: true},
		{name: "check", args: []string{"check"}, want: true},
		{name: "generate skills", args: []string{"generate", "skills"}, want: true},
		{name: "sync", args: []string{"sync"}, want: true},
		{name: "learn parent help", args: []string{"learn"}, want: false},
		{name: "learn current", args: []string{"learn", "current"}, want: true},
		{name: "learn history", args: []string{"learn", "history"}, want: true},
		{name: "patterns parent help", args: []string{"patterns"}, want: false},
		{name: "patterns stats", args: []string{"patterns", "stats"}, want: true},
		{name: "patterns merge", args: []string{"patterns", "merge"}, want: true},
		{name: "patterns show", args: []string{"patterns", "show"}, want: true},
		{name: "profile parent help", args: []string{"profile"}, want: false},
		{name: "profile show", args: []string{"profile", "show"}, want: true},
		{name: "profile refresh", args: []string{"profile", "refresh"}, want: true},
		{name: "review parent help", args: []string{"review"}, want: false},
		{name: "review import", args: []string{"review", "import", "--from-file", "comments.json"}, want: true},
		{name: "review stats", args: []string{"review", "stats"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, commandNeedsProjectRuntime(tt.args))
		})
	}
}

func TestNoArgCommandsRejectPositionalHelpArgument(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	rootCmd := createRootCmd()
	registerCommands(rootCmd, nil)

	commandPaths := [][]string{
		{"init"},
		{"reset"},
		{"check"},
		{"generate", "skills"},
		{"learn", "current"},
		{"learn", "history"},
		{"patterns", "stats"},
		{"patterns", "merge"},
		{"profile", "show"},
		{"profile", "refresh"},
		{"review", "import"},
		{"review", "stats"},
		{"hook"},
		{"hook", "install"},
		{"hook", "uninstall"},
		{"hook", "run"},
	}

	for _, path := range commandPaths {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			cmd, _, err := rootCmd.Find(path)
			require.NoError(t, err)
			require.NotNil(t, cmd.Args, "command %q must reject positional args before Run/RunE", cmd.CommandPath())
			require.Error(t, cmd.Args(cmd, []string{"help"}))
		})
	}
}

func TestRunHelpDoesNotOpenLockedProjectRuntime(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	projectRoot := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(projectRoot, ".git"), 0755))
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.RootPath = projectRoot
	require.NoError(t, configRepo.Update(cfg))

	lockedRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "memory", "project.db"))
	require.NoError(t, err)
	defer lockedRepo.Close()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectRoot))
	defer func() {
		require.NoError(t, os.Chdir(origDir))
	}()

	origArgs := os.Args
	os.Args = []string{"skills-seed", "help"}
	defer func() {
		os.Args = origArgs
	}()

	require.NoError(t, Run())
}

func requireHelpText(t *testing.T, fieldName, commandName, value string) {
	t.Helper()

	text := strings.TrimSpace(value)
	require.NotEmpty(t, text, "command %q must define %s help", commandName, fieldName)
	require.False(t, unresolvedI18nKeyPattern.MatchString(text), "command %q has unresolved %s i18n key", commandName, fieldName)
}

func walkCommands(t *testing.T, cmd *cobra.Command, visit func(*testing.T, *cobra.Command)) {
	t.Helper()

	visit(t, cmd)
	for _, child := range cmd.Commands() {
		walkCommands(t, child, visit)
	}
}

func commandPath(cmd *cobra.Command) string {
	if cmd.CommandPath() != "" {
		return cmd.CommandPath()
	}
	return cmd.Use
}

func isBuiltinHelpCommand(cmd *cobra.Command) bool {
	return cmd.Name() == "help" || cmd.Name() == "completion"
}
