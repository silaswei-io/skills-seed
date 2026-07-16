package interactive

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	term "github.com/charmbracelet/x/term"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// Option 表示一个可选择项。
type Option[T comparable] struct {
	Value       T
	Title       string
	Description string
}

// Select 显示单选列表并返回用户选择。
func Select[T comparable](title string, options []Option[T], defaultValue T) (T, error) {
	var zero T
	if len(options) == 0 {
		return zero, fmt.Errorf("%s", i18n.Get("InteractiveSelectRequiresOption"))
	}

	value := defaultValue
	cursor := optionIndex(options, defaultValue)
	if !containsOptionValue(options, defaultValue) {
		value = options[0].Value
		cursor = 0
	}

	program := tea.NewProgram(selectModel[T]{
		title:   title,
		options: options,
		cursor:  cursor,
		value:   value,
	})
	finalModel, err := program.Run()
	if err != nil {
		return zero, err
	}
	model := finalModel.(selectModel[T])
	if !model.selected {
		return zero, ErrCanceled
	}
	return model.value, nil
}

// Confirm 显示确认选择。
func Confirm(title, yesLabel, noLabel string, defaultYes bool) (bool, error) {
	value := defaultYes
	err := huh.NewConfirm().
		Title(title).
		Affirmative(yesLabel).
		Negative(noLabel).
		Value(&value).
		WithWidth(promptWidth()).
		WithTheme(promptTheme()).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, ErrCanceled
		}
		return false, err
	}
	return value, nil
}

// Int 显示整数输入框并返回用户输入值。
func Int(title string, defaultValue, minValue int) (int, error) {
	text := strconv.Itoa(defaultValue)
	err := huh.NewInput().
		Title(title).
		Value(&text).
		Validate(func(value string) error {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return fmt.Errorf("%s", i18n.Get("InteractiveInvalidInteger"))
			}
			if parsed < minValue {
				return fmt.Errorf("%s", i18n.GetWithParams("InteractiveMinimumInteger", map[string]interface{}{"Minimum": minValue}))
			}
			return nil
		}).
		WithWidth(promptWidth()).
		WithTheme(promptTheme()).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return 0, ErrCanceled
		}
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(text))
}

// SummaryItem 是执行前摘要中的一行键值。
type SummaryItem struct {
	Label string
	Value string
}

// PrintSummary 输出执行摘要。
func PrintSummary(w io.Writer, title string, items []SummaryItem) {
	if title != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, lipgloss.NewStyle().Bold(true).Render(title))
	}
	for _, item := range items {
		fmt.Fprintf(w, "- %s: %s\n", item.Label, item.Value)
	}
}

func containsOptionValue[T comparable](options []Option[T], value T) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func optionIndex[T comparable](options []Option[T], value T) int {
	for i, option := range options {
		if option.Value == value {
			return i
		}
	}
	return 0
}

type selectModel[T comparable] struct {
	title    string
	options  []Option[T]
	cursor   int
	value    T
	selected bool
	quitting bool
}

func (m selectModel[T]) Init() tea.Cmd {
	return nil
}

func (m selectModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			m.value = m.options[m.cursor].Value
			m.selected = true
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = len(m.options) - 1
		}
	}
	return m, nil
}

func (m selectModel[T]) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	if strings.TrimSpace(m.title) != "" {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(m.title))
		b.WriteString("\n\n")
	}
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	for i, option := range m.options {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		b.WriteString(prefix)
		b.WriteString(option.Title)
		b.WriteString("\n")
		if option.Description != "" {
			b.WriteString(descStyle.Render("  " + option.Description))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func promptTheme() *huh.Theme {
	theme := huh.ThemeBase()
	theme.Focused.Title = lipgloss.NewStyle().Bold(true)
	theme.Focused.Description = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	theme.Focused.Option = lipgloss.NewStyle()
	theme.Focused.UnselectedOption = lipgloss.NewStyle()
	theme.Focused.SelectedOption = lipgloss.NewStyle().Bold(true)
	theme.Focused.FocusedButton = lipgloss.NewStyle().Reverse(true).Padding(0, 2).MarginRight(1)
	theme.Blurred = huh.ThemeBase().Blurred
	return theme
}

func promptWidth() int {
	const (
		defaultWidth = 96
		minWidth     = 48
		margin       = 8
	)
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || width <= 0 {
		return defaultWidth
	}
	width -= margin
	if width < minWidth {
		return minWidth
	}
	return width
}
