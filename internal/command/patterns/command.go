package patterns

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	"github.com/spf13/cobra"
)

// Cmd 返回 patterns 命令
func Cmd(cont *container.Container) *cobra.Command {
	patternsCmd := &cobra.Command{
		Use:     "patterns",
		Short:   i18n.Get("PatternsShort"),
		Long:    i18n.Get("PatternsLongDesc"),
		Example: i18n.Get("PatternsExample"),
	}

	patternsCmd.AddCommand(addCmd(cont))
	patternsCmd.AddCommand(mergeCmd(cont))
	patternsCmd.AddCommand(statsCmd(cont))
	return patternsCmd
}

func addCmd(cont *container.Container) *cobra.Command {
	var category string
	var files []string
	var userContext string

	cmd := &cobra.Command{
		Use:     "add <description>",
		Short:   i18n.Get("PatternsAddShort"),
		Long:    i18n.Get("PatternsAddLongDesc"),
		Example: i18n.Get("PatternsAddExample"),
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if err := commandutil.RequireAgentAvailable(cont); err != nil {
				return err
			}

			description := strings.Join(args, " ")
			req := &agent.UserDefinePatternRequest{
				Description: description,
				Category:    category,
				Files:       files,
				UserContext: userContext,
				WorkDir:     cont.Config.Project.RootPath,
				Language:    cont.Config.Project.Language,
			}

			tracker := progress.New(1)
			retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
			ctx := retryProgress.WithContext(cmd.Context())
			label := i18n.Get("ProgressUserDefinePatternAI")
			var result *agent.UserDefinePatternResult
			err := tracker.RunStep(label, func() error {
				retryProgress.StartStep(label)
				var callErr error
				result, callErr = cont.Agent.UserDefinePattern(ctx, req)
				retryProgress.FinishStep(label, callErr == nil)
				return callErr
			})
			if err != nil {
				return err
			}

			if err := cont.PatternRepo.Save(ctx, result.Pattern); err != nil {
				return err
			}

			logger.Info(i18n.GetWithParams("PatternsAddComplete", map[string]interface{}{
				"PatternID":   result.Pattern.ID,
				"PatternName": result.Pattern.Name,
				"Category":    string(result.Pattern.Category),
				"Source":      string(result.Pattern.Source),
			}))
			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("PatternsAddFlagCategory"))
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, i18n.Get("PatternsAddFlagFiles"))
	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("PatternsAddFlagContext"))
	return cmd
}

func statsCmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stats",
		Short:   "Show learned pattern quality and check hit statistics",
		Long:    "Show learned pattern quality metrics and check hit statistics, including specificity, confidence, effective score, hit count, and last hit time.",
		Example: "skills-seed patterns stats",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil || cont.PatternStats == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			stats, err := cont.PatternStats.GetPatternHitStats(context.Background())
			if err != nil {
				return err
			}
			return writeStats(cmd.OutOrStdout(), stats)
		},
	}
	return cmd
}

func writeStats(w io.Writer, stats []domain.PatternHitStats) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PATTERN\tCATEGORY\tSPECIFICITY\tCONFIDENCE\tEFFECTIVE\tHITS\tLAST_HIT"); err != nil {
		return err
	}
	for _, stat := range stats {
		lastHit := "-"
		if !stat.LastHitAt.IsZero() {
			lastHit = stat.LastHitAt.Format("2006-01-02 15:04:05")
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%.2f\t%.2f\t%.2f\t%d\t%s\n",
			stat.Pattern.ID,
			stat.Pattern.Category,
			stat.Pattern.Metrics.SpecificityScore,
			stat.Pattern.Confidence,
			stat.Pattern.Metrics.EffectiveScore,
			stat.HitCount,
			lastHit,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func mergeCmd(cont *container.Container) *cobra.Command {
	var category string
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "merge",
		Short:   i18n.Get("PatternsMergeShort"),
		Long:    i18n.Get("PatternsMergeLongDesc"),
		Example: i18n.Get("PatternsMergeExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if err := commandutil.RequireAgentAvailable(cont); err != nil {
				return err
			}
			result, err := cont.MergerSvc.MergePatterns(context.Background(), &merger.MergePatternsRequest{
				Category: category,
				DryRun:   dryRun,
			})
			if err != nil {
				return err
			}
			logger.Info(i18n.GetWithParams("PatternsMergeComplete", map[string]interface{}{
				"TotalInput":     result.Summary.TotalInput,
				"TotalMerged":    result.Summary.TotalMerged,
				"TotalUnchanged": result.Summary.TotalUnchanged,
				"MergeCount":     result.Summary.MergeCount,
			}))
			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("PatternsMergeFlagCategory"))
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.Get("PatternsMergeFlagDryRun"))
	return cmd
}
