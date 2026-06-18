package bootstrap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	commandDocsStartMarker = "<!-- COMMAND_TREE_START -->"
	commandDocsEndMarker   = "<!-- COMMAND_TREE_END -->"
)

// RenderCommandTreeDocs 渲染由 Cobra 命令树生成的命令索引区块。
func RenderCommandTreeDocs(locale string) (string, error) {
	if err := i18n.Init(locale); err != nil {
		return "", err
	}
	rootCmd := createRootCmd()
	registerCommands(rootCmd, nil)
	rootCmd.InitDefaultHelpCmd()

	var b strings.Builder
	if strings.EqualFold(locale, "en-US") {
		renderCommandTreeDocsEn(&b, rootCmd)
	} else {
		renderCommandTreeDocsZh(&b, rootCmd)
	}
	return strings.TrimSpace(b.String()), nil
}

func renderCommandTreeDocsZh(b *strings.Builder, rootCmd *cobra.Command) {
	b.WriteString("## 自动生成命令索引\n\n")
	b.WriteString("> 本节由 Cobra command tree 生成，用于校验命令、子命令和参数默认值是否与 CLI 实现一致；详细场景说明仍以各命令章节为准。\n\n")
	renderCommandSummaryTable(b, rootCmd, "命令", "摘要", "子命令", "参数")
}

func renderCommandTreeDocsEn(b *strings.Builder, rootCmd *cobra.Command) {
	b.WriteString("## Generated Command Index\n\n")
	b.WriteString("> This section is generated from the Cobra command tree to keep commands, subcommands, and flag defaults aligned with the CLI implementation. Detailed usage notes remain in the command sections below.\n\n")
	renderCommandSummaryTable(b, rootCmd, "Command", "Summary", "Subcommands", "Flags")
}

func renderCommandSummaryTable(b *strings.Builder, rootCmd *cobra.Command, commandHeader, summaryHeader, subcommandsHeader, flagsHeader string) {
	b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", commandHeader, summaryHeader, subcommandsHeader, flagsHeader))
	b.WriteString("|---|---|---|---|\n")
	for _, cmd := range availableCommands(rootCmd) {
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n",
			escapeMarkdownTable(commandUsePath(cmd)),
			escapeMarkdownTable(oneLine(cmd.Short)),
			escapeMarkdownTable(commandChildrenSummary(cmd)),
			escapeMarkdownTable(commandFlagsSummary(cmd)),
		))
	}
}

func availableCommands(rootCmd *cobra.Command) []*cobra.Command {
	var commands []*cobra.Command
	walkAvailableCommands(rootCmd, func(cmd *cobra.Command) {
		commands = append(commands, cmd)
	})
	return commands
}

func walkAvailableCommands(cmd *cobra.Command, visit func(*cobra.Command)) {
	if cmd == nil || cmd.Hidden || !cmd.IsAvailableCommand() {
		return
	}
	visit(cmd)
	for _, child := range sortedCommands(cmd.Commands()) {
		walkAvailableCommands(child, visit)
	}
}

func sortedCommands(commands []*cobra.Command) []*cobra.Command {
	result := append([]*cobra.Command(nil), commands...)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

func commandUsePath(cmd *cobra.Command) string {
	parts := []string{cmd.Use}
	for parent := cmd.Parent(); parent != nil; parent = parent.Parent() {
		parts = append([]string{parent.Use}, parts...)
	}
	return strings.Join(parts, " ")
}

func commandChildrenSummary(cmd *cobra.Command) string {
	children := sortedCommands(cmd.Commands())
	names := make([]string, 0, len(children))
	for _, child := range children {
		if child.Hidden || !child.IsAvailableCommand() {
			continue
		}
		names = append(names, "`"+child.Use+"`")
	}
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ", ")
}

func commandFlagsSummary(cmd *cobra.Command) string {
	flags := make([]string, 0)
	cmd.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, formatFlagSummary(flag))
	})
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, formatFlagSummary(flag))
	})
	sort.Strings(flags)
	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, "<br>")
}

func formatFlagSummary(flag *pflag.Flag) string {
	name := "--" + flag.Name
	if flag.Shorthand != "" {
		name += ", -" + flag.Shorthand
	}
	return fmt.Sprintf("`%s` = `%s`", name, flag.DefValue)
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func escapeMarkdownTable(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}
