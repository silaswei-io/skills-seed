package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// Action 用户选择的操作
type Action int

const (
	ActionAutoFix Action = iota
	ActionManualFix
	ActionLearnAndCommit
)

// Choice 选项
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

// item 实现list.Item接口
type item struct {
	choice Choice
}

func (i item) FilterValue() string {
	return i.choice.TitleText
}

func (i item) Title() string {
	return i.choice.TitleText
}

func (i item) Description() string {
	return i.choice.DescriptionText
}

// model 选择器模型
type model struct {
	list     list.Model
	choice   Action
	quitting bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			m.choice = ActionManualFix
			return m, tea.Quit

		case "enter":
			if selectedItem, ok := m.list.SelectedItem().(item); ok {
				m.choice = selectedItem.choice.ID
			}
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

// SelectAction 显示交互式选择器
func SelectAction(issuesCount int) (Action, error) {
	// 显示标题
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Render(i18n.GetWithParams("InteractiveFoundIssues", map[string]interface{}{"Count": issuesCount}))

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(i18n.Get("InteractiveSelectAction"))

	fmt.Println(title)
	fmt.Println(subtitle)

	// 创建列表项
	items := make([]list.Item, len(choices))
	for i, choice := range choices {
		items[i] = item{choice: choice}
	}

	// 创建列表
	const defaultWidth = 80
	const listHeight = 10

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	// 自定义样式
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("170"))
	l.SetDelegate(delegate)

	m := model{list: l}

	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return ActionManualFix, err
	}

	return finalModel.(model).choice, nil
}

// FormatIssues 格式化问题列表
func FormatIssues(issues []string) string {
	var sb strings.Builder
	for _, issue := range issues {
		sb.WriteString("  • ")
		sb.WriteString(issue)
		sb.WriteString("\n")
	}
	return sb.String()
}

// StrategyChoice 策略选项
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

// strategyItem 实现list.Item接口
type strategyItem struct {
	choice StrategyChoice
}

func (i strategyItem) FilterValue() string {
	return i.choice.TitleText
}

func (i strategyItem) Title() string {
	return i.choice.TitleText
}

func (i strategyItem) Description() string {
	return i.choice.DescriptionText
}

// strategyModel 策略选择器模型
type strategyModel struct {
	list     list.Model
	strategy string
	quitting bool
}

func (m strategyModel) Init() tea.Cmd {
	return nil
}

func (m strategyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			m.strategy = ""
			return m, tea.Quit

		case "enter":
			if selectedItem, ok := m.list.SelectedItem().(strategyItem); ok {
				m.strategy = selectedItem.choice.ID
			}
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m strategyModel) View() string {
	if m.quitting {
		return ""
	}
	return "\n" + m.list.View()
}

// SelectStrategy 显示策略选择器
func SelectStrategy(defaultStrategy string) (string, error) {
	// 显示标题
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Render(i18n.Get("InteractiveSelectStrategy"))

	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(i18n.Get("InteractiveSelectStrategyHint"))

	fmt.Println(title)
	fmt.Println(subtitle)

	// 创建列表项
	items := make([]list.Item, len(strategyChoices))
	defaultIndex := 0
	for i, choice := range strategyChoices {
		items[i] = strategyItem{choice: choice}
		if choice.ID == defaultStrategy {
			defaultIndex = i
		}
	}

	// 创建列表
	const defaultWidth = 80
	const listHeight = 12

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	// 自定义样式
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("170")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("170"))
	l.SetDelegate(delegate)

	// 设置默认选中项
	l.Select(defaultIndex)

	m := strategyModel{list: l}

	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return defaultStrategy, err
	}

	selectedStrategy := finalModel.(strategyModel).strategy
	if selectedStrategy == "" {
		return defaultStrategy, nil
	}

	return selectedStrategy, nil
}
