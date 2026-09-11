package interactive

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestPrintBannerAndSummary(t *testing.T) {
	PrintBanner(nil, "ignored", "ignored", nil)

	var output bytes.Buffer
	PrintBanner(&output, "skills-seed", "Learn once", []BannerTag{{Label: "go"}, {Label: "codex"}})
	rendered := output.String()
	require.Contains(t, rendered, "Learn once")
	require.Contains(t, rendered, "go")
	require.Contains(t, rendered, "codex")
	for _, line := range logoLines("ignored") {
		require.Contains(t, rendered, line)
	}

	output.Reset()
	PrintSummary(&output, "Summary", []SummaryItem{{Label: "Files", Value: "2"}})
	require.Contains(t, output.String(), "Summary")
	require.Contains(t, output.String(), "- Files: 2")

	output.Reset()
	PrintSummary(&output, "", []SummaryItem{{Label: "Files", Value: "0"}})
	require.False(t, strings.HasPrefix(output.String(), "\n"))
}

func TestOptionHelpers(t *testing.T) {
	options := []Option[string]{{Value: "first"}, {Value: "second"}}
	require.True(t, containsOptionValue(options, "second"))
	require.False(t, containsOptionValue(options, "missing"))
	require.Equal(t, 1, optionIndex(options, "second"))
	require.Zero(t, optionIndex(options, "missing"))

	_, err := Select("empty", []Option[string]{}, "")
	require.Error(t, err)
}

func TestSelectModelNavigationSelectionAndCancellation(t *testing.T) {
	model := selectModel[string]{
		title: "Choose",
		options: []Option[string]{
			{Value: "first", Title: "First", Description: "first option"},
			{Value: "second", Title: "Second"},
		},
		value: "first",
	}
	require.Nil(t, model.Init())
	view := model.View()
	require.Contains(t, view, "Choose")
	require.Contains(t, view, "> First")
	require.Contains(t, view, "first option")

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Nil(t, cmd)
	model = updated.(selectModel[string])
	require.Equal(t, 1, model.cursor)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(selectModel[string])
	require.Equal(t, 1, model.cursor)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyHome})
	model = updated.(selectModel[string])
	require.Zero(t, model.cursor)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	model = updated.(selectModel[string])
	require.Equal(t, 1, model.cursor)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(selectModel[string])
	require.Zero(t, model.cursor)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(selectModel[string])
	require.Zero(t, model.cursor)

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	model = updated.(selectModel[string])
	require.True(t, model.selected)
	require.True(t, model.quitting)
	require.Equal(t, "first", model.value)
	require.Empty(t, model.View())

	canceled, cmd := (selectModel[string]{options: model.options}).Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	require.True(t, canceled.(selectModel[string]).quitting)
}

func TestPromptPresentationHelpers(t *testing.T) {
	require.NotNil(t, promptTheme())
	require.GreaterOrEqual(t, promptWidth(), 48)
	require.Len(t, logoLines("anything"), 6)
	require.Error(t, ErrCanceled)
	_ = IsTerminal()
}
