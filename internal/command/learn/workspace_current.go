package learn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

func logLearnWorkspaceProjectSummary(projectName string, result *learnCurrentProjectResult) {
	if result == nil {
		return
	}
	if result.projectName != "" {
		projectName = result.projectName
	}
	if result.skipped {
		logger.Info(i18n.GetWithParams("LearnWorkspaceProjectNoFileChanges", map[string]interface{}{
			"ProjectName": projectName,
			"Duration":    result.duration.Truncate(time.Second).String(),
		}))
		return
	}
	logger.Info(i18n.GetWithParams("LearnWorkspaceProjectSummary", map[string]interface{}{
		"ProjectName": projectName,
		"Changed":     result.changedCount,
		"Deleted":     result.deletedCount,
		"Skipped":     result.skippedCount,
		"Patterns":    result.patternsCount,
		"Saved":       result.savedCount,
		"Duration":    result.duration.Truncate(time.Second).String(),
	}))
}

func runLearnWorkspaceCurrent(cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	ctx := runtimecontext.WithSeedPath(context.Background(), cont.SeedPath)
	ctx = runtimecontext.WithUserContext(ctx, opts.userContext)
	startedAt := time.Now()
	workspaceConfig := cont.ConfigRepo.GetWorkspaceConfig()
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if len(workspaceConfig.Projects) == 0 {
		return domain.LearnCurrentResult{}, fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}

	projectRoot := projectConfig.RootPath
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return domain.LearnCurrentResult{}, err
		}
	}
	workspaceName := projectConfig.Name
	if workspaceName == "" {
		workspaceName = filepath.Base(projectRoot)
	}

	if err := commandutil.LockConfiguredMode(ctx, cont); err != nil {
		return domain.LearnCurrentResult{}, err
	}

	parallelism := workspacediscovery.EffectiveParallelism(domain.ModeWorkspace, cont.ConfigRepo.GetAgentConfig().Parallelism, len(workspaceConfig.Projects))
	unitParallelism := cont.ConfigRepo.GetCurrentLearningConfig().Parallelism
	if unitParallelism <= 0 {
		unitParallelism = 1
	}
	showChildDetails := parallelism == 1
	var consoleMu sync.Mutex
	var changedMu sync.Mutex
	var tokenContextsMu sync.Mutex
	var changedProjectIDs []string
	var tokenContexts []context.Context
	var multiTracker *progress.MultiTracker
	if !showChildDetails {
		multiTracker = progress.NewMulti(commandutil.WorkspaceProjectProgressNames(workspaceConfig.Projects))
		defer multiTracker.Stop()
		multiTracker.SetLabel(i18n.Get("ProgressLearnWorkspaceProjects"))
		multiTracker.SetTaskTotal(learnCurrentProjectStepTotal)
	}
	logger.Info(i18n.GetWithParams("LearnWorkspaceStart", map[string]interface{}{
		"Projects":        len(workspaceConfig.Projects),
		"Parallelism":     parallelism,
		"UnitParallelism": unitParallelism,
	}), "projects", len(workspaceConfig.Projects), "parallelism", parallelism, "unit_parallelism", unitParallelism)

	runProjects := func() error {
		return workspacediscovery.RunProjectTasks(ctx, workspaceConfig.Projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
			projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
			if err != nil {
				return err
			}
			childCont, err := commandutil.OpenWorkspaceChildContainer(ctx, projectRootPath, project, commandutil.WorkspaceChildErrorKeys{
				NotInitialized: "LearnWorkspaceChildNotInitialized",
				NotGitRepo:     "LearnWorkspaceChildNotGitRepo",
				ModeInvalid:    "LearnWorkspaceChildModeInvalid",
			})
			if err != nil {
				return err
			}
			defer childCont.Close()

			if showChildDetails {
				consoleMu.Lock()
				logger.Info(i18n.GetWithParams("LearnWorkspaceProjectStarted", map[string]interface{}{
					"ProjectName": project.ID,
					"LogPath":     workspaceProjectLogDir(childCont),
				}))
				consoleMu.Unlock()
			}

			scope := project.ID
			if scope == "" {
				scope = project.Path
			}
			progressName := commandutil.WorkspaceProjectProgressName(project)
			var childLogPath string
			stepStartedAt := time.Now()
			result, err := runLearnWorkspaceChildProject(ctx, childCont, scope, showChildDetails, func(label string) {
				if multiTracker != nil {
					stepStartedAt = time.Now()
					multiTracker.Start(progressName, label)
				}
			}, func(label string) {
				if multiTracker != nil {
					multiTracker.Update(progressName, label)
				}
			}, func(label string) {
				if multiTracker != nil {
					multiTracker.CompleteStep(progressName, label)
					pauseAfterFastWorkspaceChildStep(stepStartedAt)
				}
			}, &childLogPath, opts)
			if err != nil {
				if multiTracker != nil {
					multiTracker.Fail(progressName, i18n.Get("LearnWorkspaceProjectProgressFailed"))
				}
				return err
			}
			if multiTracker != nil {
				multiTracker.Complete(progressName, i18n.GetWithParams("LearnWorkspaceProjectProgressComplete", map[string]interface{}{
					"Patterns": result.patternsCount,
					"Saved":    result.savedCount,
				}))
			}
			consoleMu.Lock()
			if showChildDetails {
				logLearnWorkspaceProjectSummary(project.ID, result)
			}
			if result.savedCount > 0 {
				changedMu.Lock()
				changedProjectIDs = append(changedProjectIDs, scope)
				changedMu.Unlock()
			}
			if showChildDetails {
				agent.FlushTokenUsageScope(result.tokenContext)
				logger.Info(i18n.GetWithParams("LearnWorkspaceProjectDelegated", map[string]interface{}{
					"ProjectName": project.ID,
					"LogPath":     childLogPath,
				}))
			} else {
				tokenContextsMu.Lock()
				tokenContexts = append(tokenContexts, result.tokenContext)
				tokenContextsMu.Unlock()
			}
			consoleMu.Unlock()
			return nil
		})
	}
	if err := runProjects(); err != nil {
		return domain.LearnCurrentResult{}, err
	}
	if !showChildDetails {
		tokenContextsMu.Lock()
		pendingTokenContexts := append([]context.Context(nil), tokenContexts...)
		tokenContextsMu.Unlock()
		for _, tokenContext := range pendingTokenContexts {
			agent.FlushTokenUsageScope(tokenContext)
		}
	}

	relationshipsChanged, err := saveWorkspaceRelationshipArtifacts(ctx, cont, workspaceName, projectRoot, workspaceConfig, workspaceRelationshipOptions{
		ChangedProjectIDs: changedProjectIDs,
		ProfileMode:       opts.profileMode,
	})
	if err != nil {
		return domain.LearnCurrentResult{}, err
	}

	if err := commandutil.MarkLearned(ctx, cont); err != nil {
		return domain.LearnCurrentResult{}, err
	}

	logger.Info(i18n.Get("LearnWorkspaceComplete"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_workspace_current",
		"duration", time.Since(startedAt),
		"projects_count", len(workspaceConfig.Projects),
	)
	return domain.LearnCurrentResult{
		Summary: domain.LearnCurrentSummary{
			Projects:         len(workspaceConfig.Projects),
			ChangedProjects:  len(changedProjectIDs),
			WorkspaceChanged: relationshipsChanged,
			NoFileChanges:    !relationshipsChanged && len(changedProjectIDs) == 0,
		},
	}, nil
}

func runLearnWorkspaceChildProject(ctx context.Context, childCont *container.Container, scope string, showDetails bool, onStepStart func(label string), onStepUpdate func(label string), onStepComplete func(label string), logPath *string, opts learnCurrentOptions) (*learnCurrentProjectResult, error) {
	loggingConfig := childCont.ConfigRepo.GetLoggingConfig()
	logDir := filepath.Join(childCont.SeedPath, loggingConfig.LogsPath)
	logLevel := logger.ParseLevel(loggingConfig.Level)

	var result *learnCurrentProjectResult
	err := logger.WithScopedLog(ctx, logDir, "learn", logLevel, loggingConfig.MaxLogFiles, func(scopedCtx context.Context, scopedLogPath string) error {
		if logPath != nil {
			*logPath = scopedLogPath
		}
		var err error
		result, err = runLearnCurrentProjectWithOptions(childCont, learnCurrentProjectOptions{
			tokenScope:       scope,
			showProgress:     showDetails,
			showDetailedLogs: showDetails,
			onStepStart:      onStepStart,
			onStepUpdate:     onStepUpdate,
			onStepComplete:   onStepComplete,
			userContext:      opts.userContext,
			language:         opts.language,
			focusPaths:       opts.focusPaths,
			profileMode:      opts.profileMode,
		})
		return err
	})
	return result, err
}

func workspaceProjectLogDir(cont *container.Container) string {
	loggingConfig := cont.ConfigRepo.GetLoggingConfig()
	return filepath.Join(cont.SeedPath, loggingConfig.LogsPath)
}

func pauseAfterFastWorkspaceChildStep(startedAt time.Time) {
	progress.PauseAfterFastStep(startedAt, sleepAfterWorkspaceChildStep)
}
