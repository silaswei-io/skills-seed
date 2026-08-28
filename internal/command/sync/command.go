package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	stdsync "sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/silaswei-io/skills-seed/internal/service/syncflow"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	"github.com/silaswei-io/skills-seed/internal/terminal/progress"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

type syncRunMode string

const (
	syncRunAuto    syncRunMode = "auto"
	syncRunResume  syncRunMode = "resume"
	syncRunRestart syncRunMode = "restart"
)

// Dependencies 描述 sync 命令需要调用的应用用例。
type Dependencies struct {
	LearnCurrent                func(cont *container.Container, req syncflow.LearnCurrentRequest, opts LearnCurrentOptions) (domain.LearnCurrentResult, error)
	Generate                    func(cont *container.Container) error
	GenerateChild               func(cont *container.Container, opts GenerateChildOptions) error
	LearnWorkspaceRelationships func(cont *container.Container, userContext string) (bool, error)
	GenerateWorkspaceRoot       func(cont *container.Container) error
}

// LearnCurrentOptions 描述 sync 命令层对子项目学习过程的展示控制。
type LearnCurrentOptions struct {
	Quiet          bool
	ScopeKind      runjournal.ScopeKind
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

// GenerateChildOptions 描述工作区同步时子项目生成阶段的进度回调。
type GenerateChildOptions struct {
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

const syncWorkspaceChildStepTotal = 13 // 子项目 learn current 8 步 + 子项目 skill 生成 5 步。

// Cmd 返回 sync 命令
func Cmd(cont *container.Container, deps ...Dependencies) *cobra.Command {
	dependencies := Dependencies{}
	if len(deps) > 0 {
		dependencies = deps[0]
	}
	userContext := ""
	contextPath := []string{}
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
			startedAt := time.Now()
			ctx := cmd.Context()
			stateScope := commandutil.CommandStateScopeForCobra(cmd)
			resolvedContext, err := commandutil.ResolveRuntimeContext(userContext, contextPath...)
			if err != nil {
				return err
			}
			inputs, err := normalizeSyncInputs(syncInputs{
				UserContext: resolvedContext,
			})
			if err != nil {
				return err
			}
			resolvedMode, err := syncModeFromFlags(resume, restart)
			if err != nil {
				return err
			}
			if shouldRunInteractiveSync(cmd, inputs.UserContext, noInteractive) {
				mode, err := resolveInteractiveSync(ctx, cmd, cont, stateScope)
				if err != nil {
					if errors.Is(err, interactive.ErrCanceled) {
						return nil
					}
					return err
				}
				resolvedMode = mode
			}
			if resolvedMode == syncRunRestart {
				if err := clearSyncCommandStates(cont, stateScope); err != nil {
					return err
				}
			}
			if resolvedMode == syncRunResume {
				resumable, err := hasResumableSyncCommandStateForTarget(ctx, cont, stateScope)
				if err != nil {
					return err
				}
				if !resumable {
					return fmt.Errorf("%s", i18n.Get("SyncResumeStateMissing"))
				}
			}
			change := changelog.Start(cont.SeedPath, "sync")
			result, err := syncLearn(ctx, cont, stateScope, inputs.UserContext, resolvedMode, change, dependencies)
			if err != nil {
				return err
			}
			if err := recordSyncJournal(cont, result, startedAt, runjournal.ScopeWorkspace); err != nil {
				logger.Warn(i18n.GetWithParams("SyncJournalWriteFailed", map[string]interface{}{"Error": err.Error()}))
			}
			return change.Save(i18n.Get("ChangeLogSummarySync"))
		},
	}

	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("SyncFlagContext"))
	cmd.Flags().StringArrayVar(&contextPath, "context-path", nil, i18n.Get("SyncFlagContextPath"))
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
	UserContext string
}

func normalizeSyncInputs(inputs syncInputs) (syncInputs, error) {
	inputs.UserContext = strings.TrimSpace(inputs.UserContext)
	return inputs, nil
}

func syncCommandStateSeedPaths(cont *container.Container) ([]string, error) {
	if cont == nil {
		return nil, nil
	}
	seedPaths := []string{cont.SeedPath}
	if cont.ConfigRepo == nil || cont.ConfigRepo.GetProjectConfig().Mode != domain.ModeWorkspace {
		return seedPaths, nil
	}
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	if strings.TrimSpace(projectRoot) == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
		if err != nil {
			return nil, err
		}
		seedPaths = append(seedPaths, filepath.Join(projectRootPath, ".skills-seed"))
	}
	return seedPaths, nil
}

func clearSyncCommandStates(cont *container.Container, stateScope string) error {
	seedPaths, err := syncCommandStateSeedPaths(cont)
	if err != nil {
		return err
	}
	for _, seedPath := range seedPaths {
		if err := commandstate.NewRepository(seedPath, stateScope).Clear(); err != nil {
			return err
		}
	}
	return nil
}

// syncLearn 路径 A：学习当前代码 → 生成 Skills。
func syncLearn(ctx context.Context, cont *container.Container, stateScope string, userContext string, mode syncRunMode, change *changelog.Builder, deps ...Dependencies) (domain.LearnCurrentResult, error) {
	dependencies := Dependencies{}
	if len(deps) > 0 {
		dependencies = deps[0]
	}
	if cont != nil && cont.ConfigRepo != nil && cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return syncWorkspaceLearn(ctx, cont, stateScope, userContext, mode, change, dependencies)
	}
	var learnCurrent syncflow.LearnCurrentFunc
	if dependencies.LearnCurrent != nil {
		learnCurrent = func(ctx context.Context, req syncflow.LearnCurrentRequest) (domain.LearnCurrentResult, error) {
			return dependencies.LearnCurrent(cont, req, LearnCurrentOptions{ScopeKind: runjournal.ScopeProject})
		}
	}
	var generate syncflow.GenerateFunc
	if dependencies.Generate != nil {
		generate = func(ctx context.Context) error {
			return dependencies.Generate(cont)
		}
	}
	service := syncflow.Service{
		LearnCurrent: learnCurrent,
		Generate:     generate,
		OutputMissing: func() bool {
			return syncGeneratedSkillMissing(cont)
		},
	}
	result, err := service.Run(ctx, syncflow.Request{
		Learn: syncflow.LearnCurrentRequest{
			StateScope:  stateScope,
			UserContext: userContext,
			Force:       mode == syncRunRestart,
		},
		Change: change,
	})
	if err != nil {
		return domain.LearnCurrentResult{}, err
	}
	return result, nil
}

func syncWorkspaceLearn(ctx context.Context, cont *container.Container, stateScope string, userContext string, mode syncRunMode, change *changelog.Builder, dependencies Dependencies) (domain.LearnCurrentResult, error) {
	if dependencies.LearnCurrent == nil {
		return domain.LearnCurrentResult{}, fmt.Errorf("sync learn dependency is not configured")
	}
	if dependencies.GenerateChild == nil {
		return domain.LearnCurrentResult{}, fmt.Errorf("sync child generate dependency is not configured")
	}
	if dependencies.LearnWorkspaceRelationships == nil {
		return domain.LearnCurrentResult{}, fmt.Errorf("sync workspace relationships dependency is not configured")
	}
	if dependencies.GenerateWorkspaceRoot == nil {
		return domain.LearnCurrentResult{}, fmt.Errorf("sync workspace root generate dependency is not configured")
	}

	workspaceConfig := cont.ConfigRepo.GetWorkspaceConfig()
	if len(workspaceConfig.Projects) == 0 {
		return domain.LearnCurrentResult{}, fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	projectRoot := projectConfig.RootPath
	if strings.TrimSpace(projectRoot) == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return domain.LearnCurrentResult{}, err
		}
	}

	parallelism := workspacediscovery.EffectiveParallelism(domain.ModeWorkspace, cont.ConfigRepo.GetAgentConfig().Parallelism, len(workspaceConfig.Projects))
	learnReq := syncflow.LearnCurrentRequest{
		StateScope:  stateScope,
		UserContext: userContext,
		Force:       mode == syncRunRestart,
	}
	var mu stdsync.Mutex
	changedProjects := map[string]bool{}
	childTotals := domain.LearnCurrentSummary{}
	childGenerated := false
	childProgress := progress.NewMulti(commandutil.WorkspaceProjectProgressNames(workspaceConfig.Projects))
	defer childProgress.Stop()
	childProgress.SetLabel(i18n.Get("ProgressLearnWorkspaceProjects"))
	childProgress.SetTaskTotal(syncWorkspaceChildStepTotal)

	logger.Info(i18n.Get("SyncStepLearn"))
	if err := workspacediscovery.RunProjectTasks(ctx, workspaceConfig.Projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
		childCont, err := syncOpenWorkspaceChild(ctx, projectRoot, project)
		if err != nil {
			return err
		}
		defer childCont.Close()

		progressName := commandutil.WorkspaceProjectProgressName(project)
		result, err := dependencies.LearnCurrent(childCont, learnReq, LearnCurrentOptions{
			Quiet:     true,
			ScopeKind: runjournal.ScopeChild,
			OnStepStart: func(label string) {
				childProgress.Start(progressName, workspacePhaseStepLabel("ProgressSyncWorkspacePhaseLearn", label))
			},
			OnStepUpdate: func(label string) {
				childProgress.Update(progressName, workspacePhaseStepLabel("ProgressSyncWorkspacePhaseLearn", label))
			},
			OnStepComplete: func(label string) {
				childProgress.CompleteStep(progressName, workspacePhaseStepLabel("ProgressSyncWorkspacePhaseLearn", label))
			},
		})
		if err != nil {
			childProgress.Fail(progressName, i18n.Get("LearnWorkspaceProjectProgressFailed"))
			return fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
		}
		learnChanged := syncflow.ShouldGenerateAfterLearn(result)
		outputMissing := syncGeneratedSkillMissing(childCont)
		shouldGenerate := learnChanged || outputMissing
		if shouldGenerate {
			if err := dependencies.GenerateChild(childCont, GenerateChildOptions{
				OnStepStart: func(label string) {
					childProgress.Start(progressName, workspacePhaseStepLabel("ProgressSyncWorkspacePhaseGenerate", label))
				},
				OnStepUpdate: func(label string) {
					childProgress.Update(progressName, workspacePhaseStepLabel("ProgressSyncWorkspacePhaseGenerate", label))
				},
				OnStepComplete: func(label string) {
					childProgress.CompleteStep(progressName, workspacePhaseStepLabel("ProgressSyncWorkspacePhaseGenerate", label))
				},
			}); err != nil {
				childProgress.Fail(progressName, i18n.Get("GenerateWorkspaceProjectProgressFailed"))
				return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
			}
		}
		childProgress.Complete(progressName, syncWorkspaceProjectProgressLabel(result, outputMissing))

		mu.Lock()
		mergeProjectLearnSummary(&childTotals, result.Summary)
		if learnChanged {
			changedProjects[workspaceProjectScope(project)] = true
		}
		if shouldGenerate {
			childGenerated = true
		}
		mu.Unlock()
		return nil
	}); err != nil {
		return domain.LearnCurrentResult{}, err
	}

	relationshipsChanged, err := dependencies.LearnWorkspaceRelationships(cont, userContext)
	if err != nil {
		return domain.LearnCurrentResult{}, fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
	}
	childTotals.Projects = len(workspaceConfig.Projects)
	childTotals.ChangedProjects = len(changedProjects)
	childTotals.WorkspaceChanged = relationshipsChanged
	childTotals.NoFileChanges = !relationshipsChanged && len(changedProjects) == 0
	result := domain.LearnCurrentResult{Summary: childTotals}
	logger.InfoAfterProgress(i18n.GetWithParams("SyncWorkspaceLearnCompleted", map[string]interface{}{
		"Projects":        childTotals.Projects,
		"ChangedProjects": childTotals.ChangedProjects,
		"Changed":         childTotals.ChangedFiles,
		"Deleted":         childTotals.DeletedFiles,
		"Patterns":        childTotals.PatternsFound,
		"Saved":           childTotals.PatternsSaved,
		"Retired":         childTotals.PatternsRetired,
	}))
	syncflow.RecordLearnSummary(change, result)

	rootMissing := syncSkillOutputMissing(projectConfig.RootPath, cont.ConfigRepo.GetEffectiveSkillsPath())
	if err := syncflow.RunAfterLearn(result, childGenerated || rootMissing, func() error {
		return dependencies.GenerateWorkspaceRoot(cont)
	}, change); err != nil {
		return domain.LearnCurrentResult{}, err
	}
	return result, nil
}

func syncOpenWorkspaceChild(ctx context.Context, projectRoot string, project config.WorkspaceProjectConfig) (*container.Container, error) {
	projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
	if err != nil {
		return nil, err
	}
	return commandutil.OpenWorkspaceChildContainer(ctx, projectRootPath, project, commandutil.WorkspaceChildErrorKeys{
		NotInitialized: "LearnWorkspaceChildNotInitialized",
		NotGitRepo:     "LearnWorkspaceChildNotGitRepo",
		ModeInvalid:    "LearnWorkspaceChildModeInvalid",
	})
}

func workspaceProjectScope(project config.WorkspaceProjectConfig) string {
	if strings.TrimSpace(project.ID) != "" {
		return strings.TrimSpace(project.ID)
	}
	return strings.TrimSpace(project.Path)
}

func workspacePhaseStepLabel(phaseKey, label string) string {
	phase := i18n.Get(phaseKey)
	if strings.TrimSpace(label) == "" {
		return phase
	}
	return phase + " · " + label
}

func syncWorkspaceProjectProgressLabel(result domain.LearnCurrentResult, outputMissing bool) string {
	if syncflow.ShouldGenerateAfterLearn(result) {
		summary := result.Summary
		return i18n.GetWithParams("SyncWorkspaceProjectProgressChanged", map[string]interface{}{
			"Changed":  summary.ChangedFiles,
			"Deleted":  summary.DeletedFiles,
			"Patterns": summary.PatternsFound,
			"Saved":    summary.PatternsSaved,
			"Retired":  summary.PatternsRetired,
		})
	}
	if outputMissing {
		return i18n.Get("SyncWorkspaceProjectProgressRestored")
	}
	return i18n.Get("SyncWorkspaceProjectProgressUnchanged")
}

func mergeProjectLearnSummary(total *domain.LearnCurrentSummary, project domain.LearnCurrentSummary) {
	total.ChangedFiles += project.ChangedFiles
	total.DeletedFiles += project.DeletedFiles
	total.SkippedFiles += project.SkippedFiles
	total.PatternsFound += project.PatternsFound
	total.PatternsSaved += project.PatternsSaved
	total.PatternsRetired += project.PatternsRetired
}

func syncGeneratedSkillMissing(cont *container.Container) bool {
	if cont == nil || cont.ConfigRepo == nil {
		return false
	}
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if projectConfig.Mode == domain.ModeWorkspace {
		return syncWorkspaceGeneratedSkillMissing(cont)
	}
	outputPath := strings.TrimSpace(cont.ConfigRepo.GetEffectiveSkillsPath())
	return syncSkillOutputMissing(projectConfig.RootPath, outputPath)
}

func syncWorkspaceGeneratedSkillMissing(cont *container.Container) bool {
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if syncSkillOutputMissing(projectConfig.RootPath, cont.ConfigRepo.GetEffectiveSkillsPath()) {
		return true
	}
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		target, err := workspacediscovery.ResolveChildSkillTarget(projectConfig.RootPath, project, cont.ConfigRepo)
		if err != nil {
			continue
		}
		if syncSkillFileMissing(target.OutputPath) {
			return true
		}
	}
	return false
}

func syncSkillOutputMissing(projectRoot, outputPath string) bool {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return false
	}
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(projectRoot, filepath.FromSlash(outputPath))
	}
	return syncSkillFileMissing(outputPath)
}

func syncSkillFileMissing(outputPath string) bool {
	_, err := os.Stat(filepath.Join(outputPath, "SKILL.md"))
	return errors.Is(err, os.ErrNotExist)
}

func recordSyncJournal(cont *container.Container, result domain.LearnCurrentResult, startedAt time.Time, scopeKind runjournal.ScopeKind) error {
	if cont == nil || cont.ConfigRepo == nil {
		return nil
	}
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	scope := runjournal.Scope{
		Kind:        runjournal.ScopeProject,
		Name:        projectConfig.Name,
		ProjectPath: projectConfig.RootPath,
	}
	if scopeKind != "" {
		scope.Kind = scopeKind
	} else if projectConfig.Mode == domain.ModeWorkspace {
		scope.Kind = runjournal.ScopeWorkspace
	}
	if strings.TrimSpace(scope.Name) == "" {
		scope.Name = projectConfig.Name
	}
	details := []string{}
	if result.Summary.Projects > 0 {
		details = append(details, i18n.GetWithParams("SyncJournalLearnSummary", map[string]interface{}{
			"Projects":        result.Summary.Projects,
			"ChangedProjects": result.Summary.ChangedProjects,
			"Changed":         result.Summary.ChangedFiles,
			"Deleted":         result.Summary.DeletedFiles,
			"Patterns":        result.Summary.PatternsFound,
			"Saved":           result.Summary.PatternsSaved,
			"Retired":         result.Summary.PatternsRetired,
		}))
	} else {
		details = append(details, i18n.GetWithParams("ChangeLogLearnProjectSummary", map[string]interface{}{
			"Changed":  result.Summary.ChangedFiles,
			"Deleted":  result.Summary.DeletedFiles,
			"Skipped":  result.Summary.SkippedFiles,
			"Patterns": result.Summary.PatternsFound,
			"Saved":    result.Summary.PatternsSaved,
		}))
	}
	if result.Summary.WorkspaceChanged {
		details = append(details, i18n.Get("SyncJournalWorkspaceRelationshipsChanged"))
	}
	return runjournal.Append(cont.SeedPath, runjournal.Entry{
		Command:    "sync",
		Scope:      scope,
		Summary:    i18n.Get("ChangeLogSummarySync"),
		Details:    details,
		LogPath:    logger.CurrentLogPath(),
		StartedAt:  startedAt,
		FinishedAt: time.Now(),
	})
}
