package patterns

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
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

	patternsCmd.AddCommand(mergeCmd(cont))
	return patternsCmd
}

func mergeCmd(cont *container.Container) *cobra.Command {
	var category string
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "merge",
		Short:   i18n.Get("PatternsMergeShort"),
		Long:    i18n.Get("PatternsMergeLongDesc"),
		Example: i18n.Get("PatternsMergeExample"),
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
