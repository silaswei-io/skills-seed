package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	gencmd "github.com/silaswei-io/skills-seed/internal/command/generate"
	learncmd "github.com/silaswei-io/skills-seed/internal/command/learn"
	patterncmd "github.com/silaswei-io/skills-seed/internal/command/patterns"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/silaswei-io/skills-seed/internal/pkg/changelog"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/spf13/cobra"
)

type syncRunMode string

const (
	syncRunAuto    syncRunMode = "auto"
	syncRunResume  syncRunMode = "resume"
	syncRunRestart syncRunMode = "restart"
)

// Cmd 返回 sync 命令
func Cmd(cont *container.Container) *cobra.Command {
	var addDesc string
	var category string
	var files []string
	userContext := ""
	resume := false
	restart := false
	noInteractive := false

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
			stateScope := commandutil.CommandStateScopeForCobra(cmd)
			resolvedMode, err := syncModeFromFlags(resume, restart)
			if err != nil {
				return err
			}
			if shouldRunInteractiveSync(cmd, addDesc, category, files, userContext, noInteractive) {
				mode, err := resolveInteractiveSync(ctx, cmd, cont, stateScope)
				if err != nil {
					if errors.Is(err, interactive.ErrCanceled) {
						return nil
					}
					return err
				}
				resolvedMode = mode
			}
			if resolvedMode == syncRunRestart && addDesc == "" {
				if err := commandstate.NewRepository(cont.SeedPath, stateScope).Clear(); err != nil {
					return err
				}
			}

			change := changelog.Start(cont.SeedPath, "sync")
			if addDesc != "" {
				if err := syncWithUserPattern(ctx, cont, addDesc, category, files, userContext, change); err != nil {
					return err
				}
				return change.Save(i18n.Get("ChangeLogSummaryUserPatternSync"))
			}
			if err := syncLearn(ctx, cont, stateScope, userContext, change); err != nil {
				return err
			}
			return change.Save(i18n.Get("ChangeLogSummarySync"))
		},
	}

	cmd.Flags().StringVar(&addDesc, "add", "", i18n.Get("SyncFlagAdd"))
	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("SyncFlagCategory"))
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, i18n.Get("SyncFlagFiles"))
	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("SyncFlagContext"))
	cmd.Flags().BoolVar(&resume, "resume", false, i18n.Get("SyncFlagResume"))
	cmd.Flags().BoolVar(&restart, "restart", false, i18n.Get("SyncFlagRestart"))
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, i18n.Get("InteractiveFlagNoInteractive"))

	return cmd
}

func syncModeFromFlags(resume, restart bool) (syncRunMode, error) {
	if resume && restart {
		return syncRunAuto, fmt.Errorf("%s", i18n.Get("SyncRunModeConflict"))
	}
	if resume {
		return syncRunResume, nil
	}
	if restart {
		return syncRunRestart, nil
	}
	return syncRunAuto, nil
}

// syncLearn 路径 A：学习当前代码 → 生成 Skills。
func syncLearn(ctx context.Context, cont *container.Container, stateScope string, userContext string, change *changelog.Builder) error {
	// 步骤 1：学习当前代码。
	logger.Info(i18n.Get("SyncStepLearn"))
	result, err := learncmd.RunLearnCurrentWithStateScope(cont, stateScope, userContext)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
	}
	recordLearnSummary(change, result)

	return syncLearnAfterLearn(result, func() error {
		return gencmd.RunGenerate(cont)
	}, change)
}

func syncLearnAfterLearn(result domain.LearnCurrentResult, generate func() error, change *changelog.Builder) error {
	// 步骤 2：生成 Skills。
	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := generate(); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}
	change.Detail(i18n.Get("ChangeLogGenerateCompletedAll"))

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}

// syncWithUserPattern 路径 B：添加模式 → 生成 Skills。
func syncWithUserPattern(ctx context.Context, cont *container.Container, description, category string, files []string, userContext string, change *changelog.Builder) error {
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
	if cont.CuratorSvc == nil {
		return fmt.Errorf("%s: %s", i18n.Get("SyncSavePatternFailed"), "pattern curator is not configured")
	}

	written, err := patterncmd.StoreUserDefinedPattern(ctx, cont, description, *result.Pattern)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncSavePatternFailed"), err)
	}
	result.Pattern = &written

	logger.Info(i18n.GetWithParams("SyncPatternAdded", map[string]interface{}{
		"PatternID":   result.Pattern.ID,
		"PatternName": result.Pattern.Name,
		"Category":    string(result.Pattern.Category),
	}))
	change.Detail(i18n.GetWithParams("ChangeLogPatternAdded", map[string]interface{}{
		"PatternName": result.Pattern.Name,
		"Category":    string(result.Pattern.Category),
	}))

	// 步骤 2：生成 Skills。
	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := gencmd.RunGenerate(cont); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}
	change.Detail(i18n.Get("ChangeLogGenerateCompletedAll"))

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}

func recordLearnSummary(change *changelog.Builder, result domain.LearnCurrentResult) {
	summary := result.Summary
	if summary.Projects > 0 {
		change.Detail(i18n.GetWithParams("ChangeLogLearnWorkspaceSummary", map[string]interface{}{
			"Projects":        summary.Projects,
			"ChangedProjects": summary.ChangedProjects,
		}))
		if summary.WorkspaceChanged {
			change.Detail(i18n.Get("ChangeLogWorkspaceRelationshipsChanged"))
		}
		return
	}
	if summary.NoFileChanges {
		change.Detail(i18n.Get("ChangeLogLearnNoFileChanges"))
		return
	}
	change.Detail(i18n.GetWithParams("ChangeLogLearnProjectSummary", map[string]interface{}{
		"Changed":  summary.ChangedFiles,
		"Deleted":  summary.DeletedFiles,
		"Skipped":  summary.SkippedFiles,
		"Patterns": summary.PatternsFound,
		"Saved":    summary.PatternsSaved,
	}))
}
