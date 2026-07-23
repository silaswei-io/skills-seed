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

type learnWorkspaceCurrentRun struct {
	cont *container.Container
	opts learnCurrentOptions
	ctx  context.Context

	startedAt       time.Time
	workspaceConfig config.WorkspaceConfig
	projectRoot     string
	workspaceName   string
	parallelism     int
	unitParallelism int
	showDetails     bool
	tracker         *progress.MultiTracker

	consoleMu       sync.Mutex
	resultMu        sync.Mutex
	changedProjects []string
	tokenContexts   []context.Context
}

type workspaceProjectProgress struct {
	tracker   *progress.MultiTracker
	name      string
	startedAt time.Time
}

func runLearnWorkspaceCurrent(cont *container.Container, opts learnCurrentOptions) (domain.LearnCurrentResult, error) {
	run := &learnWorkspaceCurrentRun{
		cont:      cont,
		opts:      opts,
		ctx:       runtimecontext.WithUserContext(runtimecontext.WithSeedPath(context.Background(), cont.SeedPath), opts.userContext),
		startedAt: time.Now(),
	}
	return run.execute()
}

func (r *learnWorkspaceCurrentRun) execute() (domain.LearnCurrentResult, error) {
	if err := r.prepare(); err != nil {
		return domain.LearnCurrentResult{}, err
	}
	if r.tracker != nil {
		defer r.tracker.Stop()
	}
	r.logStart()
	if err := workspacediscovery.RunProjectTasks(r.ctx, r.workspaceConfig.Projects, r.parallelism, r.runProject); err != nil {
		return domain.LearnCurrentResult{}, err
	}
	r.flushTokenUsage()

	relationshipsChanged, err := saveWorkspaceRelationshipArtifacts(r.ctx, r.cont, r.workspaceName, r.projectRoot, r.workspaceConfig, workspaceRelationshipOptions{
		ChangedProjectIDs: r.changedProjects,
		ProfileMode:       r.opts.profileMode,
	})
	if err != nil {
		return domain.LearnCurrentResult{}, err
	}
	if err := commandutil.MarkLearned(r.ctx, r.cont); err != nil {
		return domain.LearnCurrentResult{}, err
	}

	logger.Info(i18n.Get("LearnWorkspaceComplete"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_workspace_current",
		"duration", time.Since(r.startedAt),
		"projects_count", len(r.workspaceConfig.Projects),
	)
	return domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{
		Projects:         len(r.workspaceConfig.Projects),
		ChangedProjects:  len(r.changedProjects),
		WorkspaceChanged: relationshipsChanged,
		NoFileChanges:    !relationshipsChanged && len(r.changedProjects) == 0,
	}}, nil
}

func (r *learnWorkspaceCurrentRun) prepare() error {
	r.workspaceConfig = r.cont.ConfigRepo.GetWorkspaceConfig()
	if len(r.workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}
	projectConfig := r.cont.ConfigRepo.GetProjectConfig()
	r.projectRoot = projectConfig.RootPath
	if r.projectRoot == "" {
		var err error
		r.projectRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	r.workspaceName = projectConfig.Name
	if r.workspaceName == "" {
		r.workspaceName = filepath.Base(r.projectRoot)
	}
	if err := commandutil.LockConfiguredMode(r.ctx, r.cont); err != nil {
		return err
	}

	r.parallelism = workspacediscovery.EffectiveParallelism(domain.ModeWorkspace, r.cont.ConfigRepo.GetAgentConfig().Parallelism, len(r.workspaceConfig.Projects))
	r.unitParallelism = r.cont.ConfigRepo.GetCurrentLearningConfig().Parallelism
	if r.unitParallelism <= 0 {
		r.unitParallelism = 1
	}
	r.showDetails = r.parallelism == 1
	if !r.showDetails {
		r.tracker = progress.NewMulti(commandutil.WorkspaceProjectProgressNames(r.workspaceConfig.Projects))
		r.tracker.SetLabel(i18n.Get("ProgressLearnWorkspaceProjects"))
		r.tracker.SetTaskTotal(learnCurrentProjectStepTotal)
	}
	return nil
}

func (r *learnWorkspaceCurrentRun) logStart() {
	logger.Info(i18n.GetWithParams("LearnWorkspaceStart", map[string]interface{}{
		"Projects":        len(r.workspaceConfig.Projects),
		"Parallelism":     r.parallelism,
		"UnitParallelism": r.unitParallelism,
	}), "projects", len(r.workspaceConfig.Projects), "parallelism", r.parallelism, "unit_parallelism", r.unitParallelism)
}

func (r *learnWorkspaceCurrentRun) runProject(ctx context.Context, project config.WorkspaceProjectConfig) error {
	projectRoot, err := workspacediscovery.ResolveProjectRoot(r.projectRoot, project)
	if err != nil {
		return err
	}
	childCont, err := commandutil.OpenWorkspaceChildContainer(ctx, projectRoot, project, commandutil.WorkspaceChildErrorKeys{
		NotInitialized: "LearnWorkspaceChildNotInitialized",
		NotGitRepo:     "LearnWorkspaceChildNotGitRepo",
		ModeInvalid:    "LearnWorkspaceChildModeInvalid",
	})
	if err != nil {
		return err
	}
	defer childCont.Close()
	r.logProjectStarted(project, childCont)

	scope := project.ID
	if scope == "" {
		scope = project.Path
	}
	projectProgress := &workspaceProjectProgress{
		tracker: r.tracker,
		name:    commandutil.WorkspaceProjectProgressName(project),
	}
	result, logPath, err := r.runChild(ctx, childCont, scope, projectProgress)
	if err != nil {
		projectProgress.fail()
		return err
	}
	projectProgress.finish(result)
	r.recordProjectResult(project, scope, logPath, result)
	return nil
}

func (r *learnWorkspaceCurrentRun) runChild(ctx context.Context, childCont *container.Container, scope string, projectProgress *workspaceProjectProgress) (*learnCurrentProjectResult, string, error) {
	loggingConfig := childCont.ConfigRepo.GetLoggingConfig()
	logDir := filepath.Join(childCont.SeedPath, loggingConfig.LogsPath)
	logLevel := logger.ParseLevel(loggingConfig.Level)

	var result *learnCurrentProjectResult
	var logPath string
	err := logger.WithScopedLog(ctx, logDir, "learn", logLevel, loggingConfig.MaxLogFiles, func(scopedCtx context.Context, scopedLogPath string) error {
		logPath = scopedLogPath
		var err error
		result, err = runLearnCurrentProjectWithOptions(scopedCtx, childCont, learnCurrentProjectOptions{
			tokenScope:       scope,
			showProgress:     r.showDetails,
			showDetailedLogs: r.showDetails,
			onStepStart:      projectProgress.start,
			onStepUpdate:     projectProgress.update,
			onStepComplete:   projectProgress.completeStep,
			userContext:      r.opts.userContext,
			language:         r.opts.language,
			focusPaths:       r.opts.focusPaths,
			profileMode:      r.opts.profileMode,
			stateScope:       r.opts.stateScope,
			curationOutput:   r.opts.curationOutput,
			force:            r.opts.force,
		})
		return err
	})
	return result, logPath, err
}

func (r *learnWorkspaceCurrentRun) logProjectStarted(project config.WorkspaceProjectConfig, childCont *container.Container) {
	if !r.showDetails {
		return
	}
	r.consoleMu.Lock()
	defer r.consoleMu.Unlock()
	logger.Info(i18n.GetWithParams("LearnWorkspaceProjectStarted", map[string]interface{}{
		"ProjectName": project.ID,
		"LogPath":     workspaceProjectLogDir(childCont),
	}))
}

func (r *learnWorkspaceCurrentRun) recordProjectResult(project config.WorkspaceProjectConfig, scope, logPath string, result *learnCurrentProjectResult) {
	r.resultMu.Lock()
	if result.savedCount > 0 {
		r.changedProjects = append(r.changedProjects, scope)
	}
	if !r.showDetails {
		r.tokenContexts = append(r.tokenContexts, result.tokenContext)
	}
	r.resultMu.Unlock()

	if !r.showDetails {
		return
	}
	r.consoleMu.Lock()
	defer r.consoleMu.Unlock()
	logLearnWorkspaceProjectSummary(project.ID, result)
	agent.FlushTokenUsageScope(result.tokenContext)
	logger.Info(i18n.GetWithParams("LearnWorkspaceProjectDelegated", map[string]interface{}{
		"ProjectName": project.ID,
		"LogPath":     logPath,
	}))
}

func (r *learnWorkspaceCurrentRun) flushTokenUsage() {
	if r.showDetails {
		return
	}
	for _, tokenContext := range r.tokenContexts {
		agent.FlushTokenUsageScope(tokenContext)
	}
}

func (p *workspaceProjectProgress) start(label string) {
	if p.tracker == nil {
		return
	}
	p.startedAt = time.Now()
	p.tracker.Start(p.name, label)
}

func (p *workspaceProjectProgress) update(label string) {
	if p.tracker != nil {
		p.tracker.Update(p.name, label)
	}
}

func (p *workspaceProjectProgress) completeStep(label string) {
	if p.tracker == nil {
		return
	}
	p.tracker.CompleteStep(p.name, label)
	pauseAfterFastWorkspaceChildStep(p.startedAt)
}

func (p *workspaceProjectProgress) finish(result *learnCurrentProjectResult) {
	if p.tracker != nil {
		p.tracker.Complete(p.name, i18n.GetWithParams("LearnWorkspaceProjectProgressComplete", map[string]interface{}{
			"Patterns": result.patternsCount,
			"Saved":    result.savedCount,
		}))
	}
}

func (p *workspaceProjectProgress) fail() {
	if p.tracker != nil {
		p.tracker.Fail(p.name, i18n.Get("LearnWorkspaceProjectProgressFailed"))
	}
}

func workspaceProjectLogDir(cont *container.Container) string {
	loggingConfig := cont.ConfigRepo.GetLoggingConfig()
	return filepath.Join(cont.SeedPath, loggingConfig.LogsPath)
}

func pauseAfterFastWorkspaceChildStep(startedAt time.Time) {
	progress.PauseAfterFastStep(startedAt, sleepAfterWorkspaceChildStep)
}
