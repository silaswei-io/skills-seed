package interactive

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// BannerTag 是启动横幅中的短标签。
type BannerTag struct {
	Label string
}

// PrintBanner 输出统一的命令启动横幅。
func PrintBanner(w io.Writer, title, subtitle string, tags []BannerTag) {
	if w == nil {
		return
	}
	muted := lipgloss.Color("240")
	subtitleStyle := lipgloss.NewStyle().
		Foreground(muted)
	tagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("245")).
		Padding(0, 1)
	metaStyle := lipgloss.NewStyle().Foreground(muted)
	_ = title

	renderedTags := make([]string, 0, len(tags))
	for i, tag := range tags {
		if i > 0 {
			renderedTags = append(renderedTags, " ")
		}
		renderedTags = append(renderedTags, tagStyle.Render(tag.Label))
	}

	fmt.Fprintln(w)
	for _, line := range logoLines(title) {
		fmt.Fprintln(w, line)
	}
	if subtitle != "" {
		fmt.Fprintln(w, subtitleStyle.Render(subtitle))
	}
	if len(renderedTags) > 0 {
		fmt.Fprintln(w, lipgloss.JoinHorizontal(lipgloss.Top, renderedTags...))
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		fmt.Fprintln(w, metaStyle.Render(wd))
	}
	fmt.Fprintln(w)
}

func logoLines(text string) []string {
	_ = text
	return []string{
		"███████╗██╗  ██╗██╗██╗     ██╗     ███████╗      ███████╗███████╗███████╗██████╗ ",
		"██╔════╝██║ ██╔╝██║██║     ██║     ██╔════╝      ██╔════╝██╔════╝██╔════╝██╔══██╗",
		"███████╗█████╔╝ ██║██║     ██║     ███████╗█████╗███████╗█████╗  █████╗  ██║  ██║",
		"╚════██║██╔═██╗ ██║██║     ██║     ╚════██║╚════╝╚════██║██╔══╝  ██╔══╝  ██║  ██║",
		"███████║██║  ██╗██║███████╗███████╗███████║      ███████║███████╗███████╗██████╔╝",
		"╚══════╝╚═╝  ╚═╝╚═╝╚══════╝╚══════╝╚══════╝      ╚══════╝╚══════╝╚══════╝╚═════╝ ",
	}
}
