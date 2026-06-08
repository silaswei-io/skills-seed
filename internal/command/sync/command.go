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
			return syncLearn(ctx, cont, userContext)
		},
	}

	cmd.Flags().StringVar(&addDesc, "add", "", i18n.Get("SyncFlagAdd"))
	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("SyncFlagCategory"))
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, i18n.Get("SyncFlagFiles"))
	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("SyncFlagContext"))

	return cmd
}

// syncLearn 路径 A：学习当前代码 → 合并模式 → 生成 Skills。
func syncLearn(ctx context.Context, cont *container.Container, userContext string) error {
	// 步骤 1：学习当前代码。
	logger.Info(i18n.Get("SyncStepLearn"))
	if err := learncmd.RunLearnCurrentWithContext(cont, userContext); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
	}

	// 步骤 2：合并相似模式。
	logger.Info(i18n.Get("SyncStepMerge"))
	if _, err := cont.MergerSvc.MergePatterns(ctx, &merger.MergePatternsRequest{}); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncMergeFailed"), err)
	}

	// 步骤 3：生成 Skills。
	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := gencmd.RunGenerate(cont); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}

// syncWithUserPattern 路径 B：添加模式 → 合并模式 → 生成 Skills。
func syncWithUserPattern(ctx context.Context, cont *container.Container, description, category string, files []string, userContext string) error {
	// 步骤 1：添加用户自定义模式。
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

	// 步骤 2：合并相似模式。
	logger.Info(i18n.Get("SyncStepMerge"))
	if _, err := cont.MergerSvc.MergePatterns(ctx, &merger.MergePatternsRequest{}); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncMergeFailed"), err)
	}

	// 步骤 3：生成 Skills。
	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := gencmd.RunGenerate(cont); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}
