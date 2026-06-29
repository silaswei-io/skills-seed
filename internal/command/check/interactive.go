package check

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/interactive"
)

// Action 用户选择的操作。
type Action int

const (
	ActionAutoFix Action = iota
	ActionManualFix
	ActionLearnAndCommit
)

// Choice 选项。
type Choice struct {
	ID              Action
	TitleText       string
	DescriptionText string
}

var choices = []Choice{
	{
		ID:              ActionAutoFix,
		TitleText:       i18n.Get("InteractiveAutoFix"),
		DescriptionText: i18n.Get("InteractiveAutoFixDesc"),
	},
	{
		ID:              ActionManualFix,
		TitleText:       i18n.Get("InteractiveManualFix"),
		DescriptionText: i18n.Get("InteractiveManualFixDesc"),
	},
	{
		ID:              ActionLearnAndCommit,
		TitleText:       i18n.Get("InteractiveLearnAndCommit"),
		DescriptionText: i18n.Get("InteractiveLearnAndCommitDesc"),
	},
}

// SelectAction 显示交互式选择器。
func SelectAction(issuesCount int) (Action, error) {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Render(i18n.GetWithParams("InteractiveFoundIssues", map[string]interface{}{"Count": issuesCount}))
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(i18n.Get("InteractiveSelectAction"))

	fmt.Println(title)
	fmt.Println(subtitle)

	options := make([]interactive.Option[Action], 0, len(choices))
	for _, choice := range choices {
		options = append(options, interactive.Option[Action]{
			Value:       choice.ID,
			Title:       choice.TitleText,
			Description: choice.DescriptionText,
		})
	}
	return interactive.Select("", options, ActionAutoFix)
}

// FormatIssues 格式化问题列表。
func FormatIssues(issues []string) string {
	var sb strings.Builder
	for _, issue := range issues {
		sb.WriteString("  • ")
		sb.WriteString(issue)
		sb.WriteString("\n")
	}
	return sb.String()
}

// StrategyChoice 策略选项。
type StrategyChoice struct {
	ID              string
	TitleText       string
	DescriptionText string
}

var strategyChoices = []StrategyChoice{
	{
		ID:              "patch",
		TitleText:       i18n.Get("StrategyPatch"),
		DescriptionText: i18n.Get("StrategyPatchDesc"),
	},
	{
		ID:              "backup",
		TitleText:       i18n.Get("StrategyBackup"),
		DescriptionText: i18n.Get("StrategyBackupDesc"),
	},
	{
		ID:              "stash",
		TitleText:       i18n.Get("StrategyStash"),
		DescriptionText: i18n.Get("StrategyStashDesc"),
	},
	{
		ID:              "branch",
		TitleText:       i18n.Get("StrategyBranch"),
		DescriptionText: i18n.Get("StrategyBranchDesc"),
	},
}

// SelectStrategy 显示策略选择器。
func SelectStrategy(defaultStrategy string) (string, error) {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("33")).
		Render(i18n.Get("InteractiveSelectStrategy"))
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(i18n.Get("InteractiveSelectStrategyHint"))

	fmt.Println(title)
	fmt.Println(subtitle)

	options := make([]interactive.Option[string], 0, len(strategyChoices))
	for _, choice := range strategyChoices {
		options = append(options, interactive.Option[string]{
			Value:       choice.ID,
			Title:       choice.TitleText,
			Description: choice.DescriptionText,
		})
	}
	return interactive.Select("", options, defaultStrategy)
}
