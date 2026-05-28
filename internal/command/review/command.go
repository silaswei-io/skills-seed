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
		Short:   "Import review comments and show prevention statistics",
		Long:    "Import local review comments and compare them with recorded pattern hits.",
		Example: "skills-seed review import --from-file review-comments.json\nskills-seed review stats",
	}
	cmd.AddCommand(importCmd(cont))
	cmd.AddCommand(statsCmd(cont))
	return cmd
}

func importCmd(cont *container.Container) *cobra.Command {
	var fromFile string
	cmd := &cobra.Command{
		Use:     "import",
		Short:   "Import review comments from a JSON file",
		Long:    "Import local review comments from a JSON array file into the skills-seed memory database.",
		Example: "skills-seed review import --from-file review-comments.json",
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
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d review comments\n", len(comments))
			return err
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "JSON file containing review comments")
	_ = cmd.MarkFlagRequired("from-file")
	return cmd
}

func statsCmd(cont *container.Container) *cobra.Command {
	var lineWindow int
	cmd := &cobra.Command{
		Use:     "stats",
		Short:   "Show review comment prevention statistics",
		Long:    "Show how many imported review comments match recorded pattern hits within the configured line window.",
		Example: "skills-seed review stats",
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
	cmd.Flags().IntVar(&lineWindow, "line-window", defaultLineWindow, "Line distance used to match review comments to pattern hits")
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
	if _, err := fmt.Fprintln(tw, "TOTAL\tPREVENTED\tMISSED"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "%d\t%d\t%d\n", stats.TotalComments, stats.PreventedComments, stats.MissedComments); err != nil {
		return err
	}
	if len(stats.MatchedPatterns) > 0 {
		if _, err := fmt.Fprintln(tw, "\nPATTERN\tCOMMENTS"); err != nil {
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
