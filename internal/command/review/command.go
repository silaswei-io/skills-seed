package review

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/spf13/cobra"
)

const defaultLineWindow = 3

// Cmd 返回 review 命令。
func Cmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "review",
		Short:   i18n.Get("ReviewShort"),
		Long:    i18n.Get("ReviewLongDesc"),
		Example: i18n.Get("ReviewExample"),
	}
	cmd.AddCommand(importCmd(cont))
	cmd.AddCommand(statsCmd(cont))
	return cmd
}

func importCmd(cont *container.Container) *cobra.Command {
	var fromFile string
	cmd := &cobra.Command{
		Use:     "import",
		Short:   i18n.Get("ReviewImportShort"),
		Long:    i18n.Get("ReviewImportLongDesc"),
		Example: i18n.Get("ReviewImportExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := requireReviewRepository(cont)
			if err != nil {
				return err
			}
			comments, err := readReviewComments(fromFile)
			if err != nil {
				return err
			}
			if err := repo.ImportReviewComments(context.Background(), comments); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), i18n.GetWithParams("ReviewImportComplete", map[string]interface{}{"Count": len(comments)}))
			return err
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", i18n.Get("ReviewImportFlagFromFile"))
	_ = cmd.MarkFlagRequired("from-file")
	return cmd
}

func statsCmd(cont *container.Container) *cobra.Command {
	var lineWindow int
	cmd := &cobra.Command{
		Use:     "stats",
		Short:   i18n.Get("ReviewStatsShort"),
		Long:    i18n.Get("ReviewStatsLongDesc"),
		Example: i18n.Get("ReviewStatsExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := requireReviewRepository(cont)
			if err != nil {
				return err
			}
			stats, err := repo.GetReviewStats(context.Background(), lineWindow)
			if err != nil {
				return err
			}
			return writeReviewStats(cmd.OutOrStdout(), stats)
		},
	}
	cmd.Flags().IntVar(&lineWindow, "line-window", defaultLineWindow, i18n.Get("ReviewStatsFlagLineWindow"))
	return cmd
}

func requireReviewRepository(cont *container.Container) (domain.ReviewRepository, error) {
	if cont == nil || cont.ReviewRepo == nil {
		return nil, fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
	}
	return cont.ReviewRepo, nil
}

func readReviewComments(path string) ([]domain.ReviewComment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var comments []domain.ReviewComment
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func writeReviewStats(w io.Writer, stats domain.ReviewStats) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", i18n.Get("ReviewStatsOutputTotal"), i18n.Get("ReviewStatsOutputPrevented"), i18n.Get("ReviewStatsOutputMissed")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "%d\t%d\t%d\n", stats.TotalComments, stats.PreventedComments, stats.MissedComments); err != nil {
		return err
	}
	if len(stats.MatchedPatterns) > 0 {
		if _, err := fmt.Fprintf(tw, "\n%s\t%s\n", i18n.Get("ReviewStatsOutputPattern"), i18n.Get("ReviewStatsOutputComments")); err != nil {
			return err
		}
		for _, matched := range stats.MatchedPatterns {
			if _, err := fmt.Fprintf(tw, "%s\t%d\n", matched.PatternID, matched.CommentCount); err != nil {
				return err
			}
		}
	}
	return tw.Flush()
}
