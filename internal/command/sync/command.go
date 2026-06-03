package sync

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	gencmd "github.com/silaswei-io/skills-seed/internal/command/generate"
	learncmd "github.com/silaswei-io/skills-seed/internal/command/learn"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	"github.com/spf13/cobra"
)

// Cmd 返回 sync 命令
func Cmd(cont *container.Container) *cobra.Command {
	var addDesc string
	var category string
	var files []string
	userContext := ""

	cmd := &cobra.Command{
		Use:     "sync",
		Short:   i18n.Get("SyncShort"),
		Long:    i18n.Get("SyncLongDesc"),
		Example: i18n.Get("SyncExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if err := commandutil.RequireAgentAvailable(cont); err != nil {
				return err
			}
			ctx := cmd.Context()

			if addDesc != "" {
				return syncWithUserPattern(ctx, cont, addDesc, category, files, userContext)
			}
			return syncLearn(ctx, cont)
		},
	}

	cmd.Flags().StringVar(&addDesc, "add", "", i18n.Get("SyncFlagAdd"))
	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("SyncFlagCategory"))
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, i18n.Get("SyncFlagFiles"))
	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("SyncFlagContext"))

	return cmd
}

// syncLearn 路径 A: learn current → patterns merge → generate skills
func syncLearn(ctx context.Context, cont *container.Container) error {
	// Step 1: learn current
	logger.Info(i18n.Get("SyncStepLearn"))
	if err := learncmd.RunLearnCurrent(cont); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
	}

	// Step 2: patterns merge
	logger.Info(i18n.Get("SyncStepMerge"))
	if _, err := cont.MergerSvc.MergePatterns(ctx, &merger.MergePatternsRequest{}); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncMergeFailed"), err)
	}

	// Step 3: generate skills
	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := gencmd.RunGenerate(cont); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}

// syncWithUserPattern 路径 B: patterns add → patterns merge → generate skills
func syncWithUserPattern(ctx context.Context, cont *container.Container, description, category string, files []string, userContext string) error {
	// Step 1: patterns add
	logger.Info(i18n.Get("SyncStepAddPattern"))
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
	ctx = retryProgress.WithContext(ctx)
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
		return fmt.Errorf("%s: %w", i18n.Get("SyncAddPatternFailed"), err)
	}

	if err := cont.PatternRepo.Save(ctx, result.Pattern); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncSavePatternFailed"), err)
	}

	logger.Info(i18n.GetWithParams("SyncPatternAdded", map[string]interface{}{
		"PatternID":   result.Pattern.ID,
		"PatternName": result.Pattern.Name,
		"Category":    string(result.Pattern.Category),
	}))

	// Step 2: patterns merge
	logger.Info(i18n.Get("SyncStepMerge"))
	if _, err := cont.MergerSvc.MergePatterns(ctx, &merger.MergePatternsRequest{}); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncMergeFailed"), err)
	}

	// Step 3: generate skills
	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := gencmd.RunGenerate(cont); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}
