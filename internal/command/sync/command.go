package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

var (
	errSyncInvalidUserPattern = errors.New("invalid user pattern result")
)

// Cmd 返回 sync 命令
func Cmd(cont *container.Container) *cobra.Command {
	var category string
	var files []string
	var patternDescription string
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
			inputs, err := normalizeSyncInputs(syncInputs{
				Category:           category,
				Files:              files,
				PatternDescription: patternDescription,
				UserContext:        userContext,
			})
			if err != nil {
				return err
			}
			resolvedMode, err := syncModeFromFlags(resume, restart)
			if err != nil {
				return err
			}
			if shouldRunInteractiveSync(cmd, inputs.Category, inputs.Files, inputs.PatternDescription, inputs.UserContext, noInteractive) {
				mode, err := resolveInteractiveSync(ctx, cmd, cont, stateScope)
				if err != nil {
					if errors.Is(err, interactive.ErrCanceled) {
						return nil
					}
					return err
				}
				resolvedMode = mode
			}
			if inputs.PatternDescription != "" && resolvedMode == syncRunResume {
				return fmt.Errorf("%s", i18n.Get("SyncPatternResumeConflict"))
			}
			if resolvedMode == syncRunRestart {
				if err := commandstate.NewRepository(cont.SeedPath, stateScope).Clear(); err != nil {
					return err
				}
			}
			if resolvedMode == syncRunResume {
				resumable, err := hasResumableSyncCommandState(ctx, cont.SeedPath, stateScope)
				if err != nil {
					return err
				}
				if !resumable {
					return fmt.Errorf("%s", i18n.Get("SyncResumeStateMissing"))
				}
			}
			if inputs.PatternDescription == "" && (inputs.Category != "" || len(inputs.Files) > 0) {
				return fmt.Errorf("%s", i18n.Get("SyncPatternOptionsRequirePattern"))
			}

			change := changelog.Start(cont.SeedPath, "sync")
			if inputs.PatternDescription != "" {
				if err := syncWithUserPattern(ctx, cont, inputs.PatternDescription, inputs.Category, inputs.Files, inputs.UserContext, change); err != nil {
					return err
				}
				return change.Save(i18n.Get("ChangeLogSummaryUserPatternSync"))
			}
			if err := syncLearn(ctx, cont, stateScope, inputs.UserContext, change); err != nil {
				return err
			}
			return change.Save(i18n.Get("ChangeLogSummarySync"))
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("SyncFlagCategory"))
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, i18n.Get("SyncFlagFiles"))
	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("SyncFlagContext"))
	cmd.Flags().StringVar(&patternDescription, "pattern", "", i18n.Get("SyncFlagPattern"))
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

type syncInputs struct {
	Category           string
	Files              []string
	PatternDescription string
	UserContext        string
}

func normalizeSyncInputs(inputs syncInputs) (syncInputs, error) {
	inputs.Category = strings.TrimSpace(inputs.Category)
	inputs.PatternDescription = strings.TrimSpace(inputs.PatternDescription)
	inputs.UserContext = strings.TrimSpace(inputs.UserContext)
	files := make([]string, 0, len(inputs.Files))
	for _, file := range inputs.Files {
		file = cleanSyncRelativePath(file)
		if file == "" {
			continue
		}
		if filepath.IsAbs(file) || strings.HasPrefix(file, "../") || strings.Contains(file, "/../") {
			return syncInputs{}, fmt.Errorf("%s", i18n.GetWithParams("SyncInvalidPatternFile", map[string]interface{}{"Path": file}))
		}
		files = append(files, file)
	}
	inputs.Files = files
	return inputs, nil
}

func cleanSyncRelativePath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "./")
	path = strings.Trim(path, "/")
	if path == "" || path == "." {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
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

	return syncLearnAfterLearn(result, syncGeneratedSkillMissing(cont), func() error {
		return gencmd.RunGenerate(cont)
	}, change)
}

func syncLearnAfterLearn(result domain.LearnCurrentResult, outputMissing bool, generate func() error, change *changelog.Builder) error {
	if !syncShouldGenerateAfterLearn(result) && !outputMissing {
		if change != nil {
			change.Detail(i18n.Get("ChangeLogGenerateSkippedNoChanges"))
		}
		logger.Info(i18n.Get("SyncGenerateSkippedNoChanges"))
		logger.Info(i18n.Get("SyncComplete"))
		return nil
	}
	// 步骤 2：生成 Skills。
	logger.Info(i18n.Get("SyncStepGenerate"))
	if err := generate(); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
	}
	if change != nil {
		change.Detail(i18n.Get("ChangeLogGenerateCompletedAll"))
	}

	logger.Info(i18n.Get("SyncComplete"))
	return nil
}

func syncShouldGenerateAfterLearn(result domain.LearnCurrentResult) bool {
	summary := result.Summary
	if summary.Projects > 0 {
		return summary.ChangedProjects > 0 || summary.WorkspaceChanged
	}
	if summary.NoFileChanges {
		return false
	}
	return summary.ChangedFiles > 0 || summary.DeletedFiles > 0 || summary.PatternsFound > 0 || summary.PatternsSaved > 0
}

func syncGeneratedSkillMissing(cont *container.Container) bool {
	if cont == nil || cont.ConfigRepo == nil {
		return false
	}
	outputPath := strings.TrimSpace(cont.ConfigRepo.GetEffectiveSkillsPath())
	if outputPath == "" {
		return false
	}
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(cont.Config.Project.RootPath, filepath.FromSlash(outputPath))
	}
	_, err := os.Stat(filepath.Join(outputPath, "SKILL.md"))
	return errors.Is(err, os.ErrNotExist)
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
		return fmt.Errorf("%s: %s", i18n.Get("SyncSavePatternFailed"), i18n.Get("PatternCuratorNotConfigured"))
	}
	if result == nil || result.Pattern == nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncAddPatternFailed"), errSyncInvalidUserPattern)
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
