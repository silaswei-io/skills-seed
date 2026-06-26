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

func TestCommandDocsGeneratedSectionsAreCurrent(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		path   string
	}{
		{
			name:   "zh",
			locale: "zh-CN",
			path:   filepath.Join("..", "..", "docs", "COMMANDS.md"),
		},
		{
			name:   "en",
			locale: "en-US",
			path:   filepath.Join("..", "..", "docs", "COMMANDS.EN.md"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generated, err := RenderCommandTreeDocs(tt.locale)
			require.NoError(t, err)
			document, err := os.ReadFile(tt.path)
			require.NoError(t, err)
			actual, ok := extractGeneratedCommandDocsSection(string(document))
			require.True(t, ok, "%s must contain generated command docs markers", tt.path)
			require.Equal(t, generated, actual, "%s generated command docs section is stale; refresh the content between %s and %s", tt.path, commandDocsStartMarker, commandDocsEndMarker)
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

func extractGeneratedCommandDocsSection(document string) (string, bool) {
	start := strings.Index(document, commandDocsStartMarker)
	if start < 0 {
		return "", false
	}
	start += len(commandDocsStartMarker)
	end := strings.Index(document[start:], commandDocsEndMarker)
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(document[start : start+end]), true
}

func TestRootHelpRemovesCompletionAndLocalizesBuiltins(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	rootCmd := createRootCmd()
	registerCommands(rootCmd, nil)
	rootCmd.SetArgs([]string{"--help"})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	require.NoError(t, rootCmd.Execute())

	helpText := out.String()
	require.NotContains(t, helpText, "completion")
	require.NotContains(t, helpText, "Generate the autocompletion script")
	require.NotContains(t, helpText, "Help about any command")
	require.Contains(t, helpText, "help")
	require.Contains(t, helpText, "查看命令帮助")

	cmd, _, err := rootCmd.Find([]string{"completion"})
	require.Error(t, err)
	require.Equal(t, rootCmd, cmd)
}

func TestChineseHelpDoesNotExposeEnglishCommandDescriptions(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	tests := []struct {
		name    string
		args    []string
		want    []string
		notWant []string
	}{
		{
			name:    "preview",
			args:    []string{"preview", "--help"},
			want:    []string{"预览 skills-seed 将分析的文件"},
			notWant: []string{"Preview analysis inputs"},
		},
		{
			name:    "preview files",
			args:    []string{"preview", "files", "--help"},
			want:    []string{"预览 full 或 incremental 分析会选中的源文件", "只预览这些路径下的文件", "最大输出文件数量"},
			notWant: []string{"Preview files selected for analysis", "only preview files under these paths", "maximum included files to print"},
		},
		{
			name:    "patterns show",
			args:    []string{"patterns", "show", "--help"},
			want:    []string{"无参数时查看已学习 pattern 概览", "输出格式：table 或 json"},
			notWant: []string{"Show learned pattern overview or full details", "output format: table or json"},
		},
		{
			name:    "patterns stats",
			args:    []string{"patterns", "stats", "--help"},
			want:    []string{"查看已学习 pattern 的质量指标和 check 命中统计"},
			notWant: []string{"Show learned pattern quality"},
		},
		{
			name:    "review",
			args:    []string{"review", "--help"},
			want:    []string{"导入本地评审评论，并与已记录的 pattern 命中"},
			notWant: []string{"Import review comments and show prevention statistics"},
		},
		{
			name:    "review import",
			args:    []string{"review", "import", "--help"},
			want:    []string{"从 JSON 数组文件导入本地评审评论", "包含评审评论数组的 JSON 文件"},
			notWant: []string{"Import review comments from a JSON file", "JSON file containing review comments"},
		},
		{
			name:    "review stats",
			args:    []string{"review", "stats", "--help"},
			want:    []string{"查看已导入评审评论在指定行号窗口内命中", "匹配评审评论与 pattern 命中时允许的行号距离"},
			notWant: []string{"Show review comment prevention statistics", "Line distance used to match review comments"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := createRootCmd()
			registerCommands(rootCmd, nil)
			rootCmd.SetArgs(tt.args)
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)

			require.NoError(t, rootCmd.Execute())

			helpText := out.String()
			for _, want := range tt.want {
				require.Contains(t, helpText, want)
			}
			for _, notWant := range tt.notWant {
				require.NotContains(t, helpText, notWant)
			}
		})
	}
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
		{name: "init", args: []string{"init"}, want: false},
		{name: "reset", args: []string{"reset"}, want: true},
		{name: "reset help positional arg", args: []string{"reset", "help"}, want: true},
		{name: "reset help flag", args: []string{"reset", "--help"}, want: false},
		{name: "hook install", args: []string{"hook", "install"}, want: false},
		{name: "log", args: []string{"log"}, want: false},
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
		{name: "patterns compact", args: []string{"patterns", "compact"}, want: true},
		{name: "patterns delete", args: []string{"patterns", "delete", "plugin-source-edit-rule"}, want: true},
		{name: "patterns rm alias", args: []string{"patterns", "rm", "plugin-source-edit-rule"}, want: true},
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
		{"log"},
		{"patterns", "stats"},
		{"patterns", "compact"},
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

	lockedRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "store", "project.db"))
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

func TestInitContainerAndLoggerReportsLockedPatternDBHint(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	projectRoot := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(projectRoot, ".git"), 0755))
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.RootPath = projectRoot
	require.NoError(t, configRepo.Update(cfg))

	lockedRepo, err := boltdb.NewPatternRepository(filepath.Join(seedPath, "store", "project.db"))
	require.NoError(t, err)
	defer lockedRepo.Close()

	cont, err := initContainerAndLogger(seedPath)

	require.Nil(t, cont)
	require.Error(t, err)
	require.Contains(t, err.Error(), "创建容器失败")
	require.Contains(t, err.Error(), "创建模式仓储失败")
	require.Contains(t, err.Error(), "数据库文件可能正在被其他 skills-seed 命令使用，请等待当前命令结束后重试")
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
	return cmd.Name() == "help"
}
