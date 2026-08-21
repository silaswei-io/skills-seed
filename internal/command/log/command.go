package log

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

// Cmd 返回 log 命令。
func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "log",
		Short:   i18n.Get("LogShort"),
		Long:    i18n.Get("LogLongDesc"),
		Example: i18n.Get("LogExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd)
		},
	}
}

func run(cmd *cobra.Command) error {
	seedPath, err := projectpath.FindSeedPath()
	if err != nil {
		return err
	}
	entries, err := runjournal.Recent(seedPath, 0)
	if err != nil {
		return err
	}
	if workspaceEntries, workspaceErr := workspaceJournalEntries(seedPath); workspaceErr == nil && len(workspaceEntries) > 0 {
		entries = mergeEntries(entries, workspaceEntries)
	}
	if len(entries) == 0 {
		legacyEntries, legacyErr := changelog.Recent(seedPath, 0)
		if legacyErr != nil {
			return legacyErr
		}
		if len(legacyEntries) == 0 {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), i18n.GetWithParams("LogNoChanges", map[string]interface{}{"Path": runjournal.Path(seedPath)}))
			return err
		}
		entries = legacyEntriesToRunJournalEntries(legacyEntries)
	}
	return printEntries(cmd.OutOrStdout(), entries)
}

func workspaceJournalEntries(seedPath string) ([]runjournal.Entry, error) {
	cfg, err := projectpath.LoadConfig(seedPath)
	if err != nil {
		return nil, err
	}
	if cfg == nil || cfg.Project.Mode != domain.ModeWorkspace {
		return nil, nil
	}
	projectRoot := cfg.Project.RootPath
	if strings.TrimSpace(projectRoot) == "" {
		return nil, nil
	}
	entries := make([]runjournal.Entry, 0)
	for _, project := range cfg.Workspace.Projects {
		childRoot, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
		if err != nil {
			continue
		}
		childSeed := filepath.Join(childRoot, ".skills-seed")
		childEntries, err := runjournal.Recent(childSeed, 0)
		if err != nil || len(childEntries) == 0 {
			continue
		}
		entries = append(entries, childEntries...)
	}
	return entries, nil
}

func mergeEntries(primary, secondary []runjournal.Entry) []runjournal.Entry {
	if len(secondary) == 0 {
		return primary
	}
	merged := append(append([]runjournal.Entry(nil), primary...), secondary...)
	sort.SliceStable(merged, func(i, j int) bool {
		left := merged[i].FinishedAt
		if left.IsZero() {
			left = merged[i].StartedAt
		}
		right := merged[j].FinishedAt
		if right.IsZero() {
			right = merged[j].StartedAt
		}
		if left.Equal(right) {
			return merged[i].ID > merged[j].ID
		}
		return left.After(right)
	})
	return merged
}

func printEntries(out io.Writer, entries []runjournal.Entry) error {
	for i, entry := range entries {
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if err := printEntry(out, entry, 0); err != nil {
			return err
		}
	}
	return nil
}

func printEntry(out io.Writer, entry runjournal.Entry, depth int) error {
	pad := strings.Repeat("  ", depth)
	if depth == 0 {
		if _, err := fmt.Fprintf(out, "%s%s %s\n", pad, i18n.Get("LogRecord"), entry.ID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s%s: %s\n", pad, i18n.Get("LogRecordDate"), formatRunTime(entry.FinishedAt, entry.StartedAt)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s%s: %s\n", pad, i18n.Get("LogRecordCommand"), entry.Command); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s%s: %s\n", pad, i18n.Get("LogRecordScope"), formatScope(entry.Scope)); err != nil {
			return err
		}
		if entry.LogPath != "" {
			if _, err := fmt.Fprintf(out, "%s%s: %s\n", pad, i18n.Get("LogRecordLogPath"), entry.LogPath); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}
	if entry.Summary != "" {
		if depth == 0 {
			if _, err := fmt.Fprintf(out, "%s%s\n", pad, entry.Summary); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(out, "%s• %s · %s\n", pad, formatScope(entry.Scope), entry.Command); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "%s  %s\n", pad, entry.Summary); err != nil {
				return err
			}
		}
	}
	for _, detail := range entry.Details {
		detail = strings.TrimSpace(detail)
		if detail == "" || detail == entry.Summary {
			continue
		}
		if _, err := fmt.Fprintf(out, "%s  - %s\n", pad, detail); err != nil {
			return err
		}
	}
	if len(entry.Children) > 0 {
		if _, err := fmt.Fprintf(out, "%s%s\n", pad, i18n.Get("LogRecordChildren")); err != nil {
			return err
		}
		for _, child := range entry.Children {
			if err := printEntry(out, child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatRunTime(finishedAt, startedAt time.Time) string {
	if finishedAt.IsZero() {
		finishedAt = startedAt
	}
	if startedAt.IsZero() {
		startedAt = finishedAt
	}
	duration := finishedAt.Sub(startedAt).Round(time.Second)
	return fmt.Sprintf("%s (%s)", finishedAt.Format("2006-01-02 15:04:05"), duration)
}

func formatScope(scope runjournal.Scope) string {
	label := i18n.Get("LogScopeProject")
	switch scope.Kind {
	case runjournal.ScopeWorkspace:
		label = i18n.Get("LogScopeWorkspace")
	case runjournal.ScopeChild:
		label = i18n.Get("LogScopeChild")
	}
	name := strings.TrimSpace(scope.Name)
	if name != "" {
		label += " " + name
	}
	if path := strings.TrimSpace(scope.ProjectPath); path != "" {
		label += " (" + path + ")"
	}
	return label
}

func legacyEntriesToRunJournalEntries(entries []changelog.Entry) []runjournal.Entry {
	out := make([]runjournal.Entry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, runjournal.Entry{
			ID:         entry.ID,
			Command:    entry.Command,
			Scope:      runjournal.Scope{Kind: runjournal.ScopeProject},
			Summary:    entry.Summary,
			Details:    append([]string(nil), entry.Details...),
			StartedAt:  entry.CreatedAt,
			FinishedAt: entry.CreatedAt,
		})
	}
	return out
}
