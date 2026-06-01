package bootstrap

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
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
	require.Contains(t, helpText, "generate-skills")
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
