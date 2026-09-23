package learncurrent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runtimeclean"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

type currentFileSelectionPlan struct {
	Candidates []string
	SkipReason string
}

func runLearnCurrentProjectWithOptions(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions) (*learnCurrentProjectResult, error) {
	run := newLearnCurrentProjectRun(ctx, cont, opts)
	if !run.hasDecisionCheckpoint() {
		if err := commandutil.RequireAgentAvailable(cont); err != nil {
			return nil, err
		}
	}
	return run.execute()
}

func (r *learnCurrentProjectRun) hasDecisionCheckpoint() bool {
	state, err := r.stateRepo.Load(r.ctx)
	return err == nil && state.Decision != nil
}

func newLearnCurrentProjectRun(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions) *learnCurrentProjectRun {
	ctx = agent.WithCallBudget(ctx, cont.ConfigRepo.GetAgentConfig().Parallelism)
	ctx = runtimecontext.WithSeedPath(ctx, cont.SeedPath)
	ctx = runtimecontext.WithUserContext(ctx, opts.userContext)
	agentCalls := runtimecontext.NewAgentCallCounter()
	ctx = runtimecontext.WithAgentCallCounter(ctx, agentCalls)
	steps := commandutil.NewConsoleStepRunner(commandutil.ConsoleStepRunnerOptions{
		TotalSteps:     learnCurrentProjectStepTotal,
		ShowProgress:   opts.showProgress,
		OnStepStart:    opts.onStepStart,
		OnStepComplete: opts.onStepComplete,
		OnStepUpdate:   opts.onStepUpdate,
	})
	ctx = steps.WithContext(ctx)

	return &learnCurrentProjectRun{
		learnDeps: learnDeps{
			cont:      cont,
			opts:      opts,
			stateRepo: learnCurrentStateRepo(cont.SeedPath, opts.stateScope),
			ctx:       ctx,
			startedAt: time.Now(),
			steps:     steps,
			observer:  newLearnRunObserver(agentCalls),
		},
		learnAgendaCtx: learnAgendaCtx{
			conversations: make(map[string]agent.Conversation),
		},
	}
}

// runObservedStage 统一记录顶层阶段耗时，避免各阶段散落计时逻辑。
func (r *learnCurrentProjectRun) runObservedStage(name string, fn func() error) error {
	r.observer.startStage(name)
	defer r.observer.endStage(name)
	return fn()
}

func (r *learnCurrentProjectRun) execute() (*learnCurrentProjectResult, error) {
	if r.opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentStart"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationStart"),
		"operation", "command.learn_current",
		"agent", r.cont.Agent.Name(),
		"seed_path", r.cont.SeedPath,
	)

	if err := r.runObservedStage(stagePrepare, r.prepareProject); err != nil {
		return nil, err
	}
	if err := r.runObservedStage(stageDetect, r.detectChanges); err != nil {
		return nil, err
	}
	if err := r.runObservedStage(stagePlan, r.runPlanningStage); err != nil {
		return nil, err
	}
	if !r.incrementalChanges.HasChanges() {
		return r.finishWithoutChanges()
	}
	if err := r.runObservedStage(stageAnalyze, r.analyzeCodebase); err != nil {
		return nil, err
	}
	if r.opts.profileMode == learnCurrentProfileAuto && r.profileRefreshRecommended.Needed {
		r.refreshProfile = true
	}
	if err := r.runObservedStage(stageNormalize, r.normalizeAndSavePatternsStep); err != nil {
		return nil, err
	}
	if err := r.runObservedStage(stageProfile, r.saveProfileIfNeeded); err != nil {
		return nil, err
	}

	if r.opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentComplete"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current",
		"duration", time.Since(r.startedAt),
		"patterns_count", len(r.patterns),
		"saved_count", r.savedCount,
		"retired_count", r.retiredCount,
		"skipped_stages", r.observer.skippedList(),
		"agent_calls", r.observer.agentCallTotal(),
	)
	if err := commandutil.MarkLearned(r.ctx, r.cont); err != nil {
		return nil, err
	}
	if r.analysisState != nil {
		if err := r.stateRepo.Clear(); err != nil {
			return nil, err
		}
	}
	result := r.buildResult(false)
	if err := recordLearnJournal(r.cont, result, r.startedAt, r.opts.scopeKind); err != nil {
		logger.Warn(i18n.GetWithParams("LearnJournalWriteFailed", map[string]interface{}{"Error": err.Error()}))
	}
	return result, nil
}

func (r *learnCurrentProjectRun) runPlanningStage() error {
	if r.stateSession != nil {
		// 恢复运行只消费已持久化的候选和议程，禁止重新调用议程规划。
		r.applyLearningCandidates()
		developmentFocuses, coverageFocuses := focusKindCounts(r.stateSession.State.Agenda.Focuses)
		planLabel := i18n.GetWithParams("ProgressLearnCurrentPlanFocusesRestored", map[string]interface{}{
			"Focuses":            len(r.stateSession.State.Agenda.Focuses),
			"DevelopmentFocuses": developmentFocuses,
			"CoverageFocuses":    coverageFocuses,
		})
		return r.steps.Run(planLabel, func() error {
			r.analysisState = r.stateSession.State
			r.plannedFocuses = pendingEvidenceFocuses(r.analysisState, r.incrementalChanges)
			return nil
		})
	}
	// 候选已由增量检测确定，直接进入议程规划，不再单独占用控制台步骤。
	r.applyLearningCandidates()
	r.logFileSelectionSummaryOnce()
	if !r.incrementalChanges.HasChanges() {
		return nil
	}
	return r.planLearningAgenda()
}

func (r *learnCurrentProjectRun) prepareProject() error {
	// 解析项目上下文可能访问 Git 和配置文件，单独作为第一步展示，避免用户以为命令无响应
	prepareStartedAt := time.Now()
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentPrepareProject"), func() error {
		var err error
		r.projectRoot, err = r.cont.GitRepo.GetProjectRoot(r.ctx)
		if err != nil {
			r.projectRoot = r.cont.ConfigRepo.GetProjectConfig().RootPath
		}
		if r.projectRoot == "" {
			r.projectRoot, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGetCurrentDir", map[string]interface{}{"Error": err.Error()}))
			}
		}

		r.projectName = filepath.Base(r.projectRoot)
		if configuredName := r.cont.ConfigRepo.GetProjectConfig().Name; configuredName != "" {
			r.projectName = configuredName
		}

		r.currentLanguage = r.opts.language
		if r.currentLanguage == "" {
			r.currentLanguage = r.cont.ConfigRepo.GetProjectConfig().Language
		}
		if r.currentLanguage == "" {
			r.currentLanguage = "unknown"
		}
		currentLearningConfig := r.cont.ConfigRepo.GetCurrentLearningConfig()
		r.learningMode = string(currentLearningConfig.Mode)

		r.resolvedFocusPaths, err = resolveFocusPaths(r.projectRoot, r.opts.focusPaths)
		if err != nil {
			return err
		}
		profileExists := false
		if r.cont.ProfileRepo != nil {
			if profile, getErr := r.cont.ProfileRepo.Get(r.ctx); getErr == nil {
				r.existingProfile = profile
				profileExists = true
			}
		}
		authorityRevision, revisionErr := analyzer.EngineeringKnowledgeRevision(r.projectRoot)
		if revisionErr != nil {
			return fmt.Errorf("%s: %w", i18n.Get("LearnCurrentAuthorityRevisionFailed"), revisionErr)
		}
		r.refreshProfile, err = shouldRefreshProfile(r.opts.profileMode, profileExists, profileAuthorityRevision(r.existingProfile), authorityRevision)
		return err
	}); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.prepare_project",
			"duration", time.Since(prepareStartedAt),
			"error", err,
		)
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.prepare_project",
		"duration", time.Since(prepareStartedAt),
		"project_root", r.projectRoot,
		"project_name", r.projectName,
		"language", r.currentLanguage,
		"focus_paths", strings.Join(projectpath.Relative(r.projectRoot, r.resolvedFocusPaths), ","),
		"profile_mode", r.opts.profileMode,
		"refresh_profile", r.refreshProfile,
	)
	if r.opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentInfo", map[string]interface{}{
			"ProjectRoot": r.projectRoot,
			"ProjectName": r.projectName,
			"Language":    r.currentLanguage,
		}))
		if len(r.resolvedFocusPaths) > 0 {
			logger.Info(i18n.GetWithParams("LearnCurrentFocusInfo", map[string]interface{}{
				"Focus":       strings.Join(projectpath.Relative(r.projectRoot, r.resolvedFocusPaths), ", "),
				"ProfileMode": r.opts.profileMode,
			}))
		}
	}
	return nil
}

func (r *learnCurrentProjectRun) detectChanges() error {
	if err := r.maybeCleanRuntimeBeforeFreshAnalysis(); err != nil {
		return err
	}
	detectStartedAt := time.Now()
	detectLabel := i18n.Get("ProgressLearnCurrentDetectChanges")
	if r.hasRestorableCurrentState() {
		detectLabel = i18n.Get("ProgressLearnCurrentResumeState")
	}
	if err := r.steps.Run(detectLabel, func() error {
		return r.restoreOrDetectChanges(detectLabel)
	}); err != nil {
		return err
	}
	r.logDetectedChanges(detectStartedAt)
	r.changeProfile = classifyCurrentChangeProfile(r.incrementalChanges)
	if r.stateSession != nil && r.stateSession.State != nil && strings.TrimSpace(r.stateSession.State.ChangeProfile) != "" {
		r.changeProfile = normalizeCurrentChangeProfile(r.stateSession.State.ChangeProfile)
	}
	return nil
}

func (r *learnCurrentProjectRun) maybeCleanRuntimeBeforeFreshAnalysis() error {
	if r.opts.skipRuntimeCleanup {
		return nil
	}
	if !r.cont.ConfigRepo.GetRuntimeConfig().CleanupBeforeReanalysis {
		return nil
	}
	if r.hasRestorableCurrentState() {
		return nil
	}
	if err := logger.Close(); err != nil {
		return err
	}
	if err := runtimeclean.Clear(r.cont.SeedPath); err != nil {
		return err
	}
	loggingConfig := r.cont.ConfigRepo.GetLoggingConfig()
	logDir := filepath.Join(r.cont.SeedPath, loggingConfig.LogsPath)
	return logger.InitWithRetention(logDir, r.stateRepo.Command(), logger.ParseLevel(loggingConfig.Level), loggingConfig.MaxLogFiles)
}

func (r *learnCurrentProjectRun) hasRestorableCurrentState() bool {
	if r.opts.force {
		return false
	}
	state, err := r.stateRepo.Load(r.ctx)
	if err != nil {
		return false
	}
	return canResumeCurrentState(state, r.projectName, r.currentLanguage, r.learningMode, r.opts.userContext, r.currentStateInvocationHash())
}

func (r *learnCurrentProjectRun) restoreOrDetectChanges(detectLabel string) error {
	r.detail(detectLabel, "ProgressLearnCurrentDetectRestoreState", nil)
	var session *currentStateSession
	var err error
	if !r.opts.force {
		session, err = restoreCurrentState(r.ctx, r.stateRepo, r.cont.FileTracker, r.projectName, r.currentLanguage, r.learningMode, r.opts.userContext, r.currentStateInvocationHash())
	}
	if err != nil {
		return err
	}
	var detected *fileanalysis.FileChanges
	if session != nil {
		detected, err = r.detectCurrentChanges(false)
		if err != nil {
			return err
		}
		if !currentStateInputsMatchProject(r.projectRoot, session.State.Files, session.State.Deleted) || !currentChangesCoveredByState(session.State, detected) {
			if err := r.stateRepo.Clear(); err != nil {
				return err
			}
			session = nil
			r.stateInvalidated = true
		}
	}
	if session != nil {
		r.stateSession = session
		r.analysisState = session.State
		r.restoreKnowledgeCommitCheckpoint()
		r.incrementalChanges = session.Changes
		focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
		r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, focusRelPaths)
		r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, r.incrementalChanges.AddedOrModified))
		r.resumeSummary = buildLearnCurrentResumeSummary(session)
		pendingFocuses := pendingEvidenceFocuses(session.State, session.Changes)
		completedFocuses := 0
		if session.State.Analysis != nil {
			completedFocuses = len(session.State.Analysis.FocusKnowledge)
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.resume_state",
			"state_scope", r.stateRepo.Command(),
			"inputs_count", currentStateInputCount(session.State),
			"pending_count", len(pendingFocuses),
			"pending_input_count", len(r.incrementalChanges.AddedOrModified)+len(r.incrementalChanges.Deleted),
			"completed_focus_count", completedFocuses,
			"pending_focus_count", len(pendingFocuses),
			"focuses_count", len(session.State.Agenda.Focuses),
		)
		return nil
	}

	r.detail(detectLabel, "ProgressLearnCurrentDetectScanFiles", nil)
	if detected == nil {
		detected, err = r.detectCurrentChanges(r.opts.force)
		if err != nil {
			return err
		}
	}
	r.incrementalChanges = detected
	focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
	r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, focusRelPaths)
	r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, r.incrementalChanges.AddedOrModified))
	return nil
}

func (r *learnCurrentProjectRun) detectCurrentChanges(force bool) (*fileanalysis.FileChanges, error) {
	return fileanalysis.PrepareCurrentChangesWithOptions(r.ctx, r.cont.FileTracker, r.cont.ConfigRepo, r.projectRoot, r.projectRoot, domain.FileAnalysisScope{}, r.resolvedFocusPaths, fileanalysis.CurrentChangeOptions{Force: force})
}

func (r *learnCurrentProjectRun) currentStateInvocationHash() string {
	focusPaths := projectpath.Relative(r.projectRoot, r.resolvedFocusPaths)
	return learnCurrentInvocationHash(focusPaths, r.opts.force)
}

// applyLearningCandidates 把增量检测得到的路径直接作为本轮学习候选。
// 不再单独占用控制台步骤；恢复场景只回填摘要，不重复改写变更集。
func (r *learnCurrentProjectRun) applyLearningCandidates() {
	focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
	r.selectionPlan = currentFileSelectionPlan{Candidates: focusRelPaths}
	if r.stateSession != nil {
		r.selectionSummary = fileSelectionSummary{
			CandidateCount: len(focusRelPaths),
			SelectedCount:  len(focusRelPaths),
			Status: i18n.GetWithParams("LearnCurrentFileSelectionSkipped", map[string]interface{}{
				"Reason": i18n.Get("ProgressLearnCurrentFileSelectionSkipRestored"),
			}),
		}
		r.selectionPlan.SkipReason = i18n.Get("ProgressLearnCurrentFileSelectionSkipRestored")
		return
	}
	if len(focusRelPaths) == 0 {
		r.selectionPlan.SkipReason = i18n.Get("ProgressLearnCurrentFileSelectionSkipNoCandidates")
		r.selectionSummary = fileSelectionSummary{
			Status: i18n.GetWithParams("LearnCurrentFileSelectionSkipped", map[string]interface{}{
				"Reason": r.selectionPlan.SkipReason,
			}),
		}
		return
	}
	selectedPaths := normalizeStatePaths(focusRelPaths)
	r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, selectedPaths)
	r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(selectedPaths, r.incrementalChanges.AddedOrModified))
	r.incrementalChanges.ApplyLearningSelection(selectedPaths, "")
	r.selectionSummary = fileSelectionSummary{
		Applied:        true,
		CandidateCount: len(focusRelPaths),
		SelectedCount:  len(selectedPaths),
		Status:         i18n.Get("LearnCurrentFileSelectionLocalReason"),
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.apply_learning_candidates",
		"candidate_count", len(focusRelPaths),
		"selected_count", len(selectedPaths),
		"fingerprint_record_count", len(r.incrementalChanges.Records),
	)
}

func (r *learnCurrentProjectRun) finishWithoutChanges() (*learnCurrentProjectResult, error) {
	recoveredKnowledge := r.stateSession != nil && r.stateSession.State.SourceBaselineCommitComplete()
	needsProfileRefresh := r.refreshProfile && !r.projectionsCommitted()
	if r.opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentNoFileChanges"))
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentPlanFocuses"), func() error { return nil }); err != nil {
		return nil, err
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error { return nil }); err != nil {
		return nil, err
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentNormalizeAndSavePatterns"), func() error { return nil }); err != nil {
		return nil, err
	}
	profileStartedAt := time.Now()
	profileStep := i18n.Get("ProgressLearnCurrentSkipProfile")
	if needsProfileRefresh {
		profileStep = i18n.Get("ProgressLearnCurrentSaveProfile")
	}
	if err := r.steps.Run(profileStep, func() error {
		var profile *domain.ProjectProfile
		if needsProfileRefresh {
			var err error
			profile, err = r.refreshProjectProfile()
			if err != nil {
				return err
			}
		} else if r.stateSession == nil {
			// 无文件变化且不是断点恢复时，已有 Profile 与知识快照仍然有效。
			// 跳过再次解析符号，避免重学在最后一步触发 CodeGraph 同步。
			return r.markProjectionsCommitted()
		} else if r.cont.ProfileRepo != nil {
			var err error
			profile, err = r.cont.ProfileRepo.Get(r.ctx)
			if err != nil && !errors.Is(err, profilestore.ErrProfileNotFound) {
				return err
			}
		}
		var err error
		profile, err = r.verifyProjectProfile(r.ctx, profile)
		if err != nil {
			return err
		}
		if profile != nil && r.cont.ProfileRepo != nil {
			if err := r.cont.ProfileRepo.Save(r.ctx, profile); err != nil {
				return err
			}
		}
		return r.markProjectionsCommitted()
	}); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.save_project_profile",
			"duration", time.Since(profileStartedAt),
			"error", err,
		)
		return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
	}
	if r.opts.showDetailedLogs {
		if needsProfileRefresh {
			logger.Info(i18n.Get("LearnCurrentProfileSaved"))
		} else {
			logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
		}
		logger.Info(i18n.Get("LearnCurrentComplete"))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current",
		"duration", time.Since(r.startedAt),
		"patterns_count", r.resultPatternCount(),
		"saved_count", r.savedCount,
		"retired_count", r.retiredCount,
		"skipped", true,
	)
	if recoveredKnowledge {
		if err := commandutil.MarkLearned(r.ctx, r.cont); err != nil {
			return nil, err
		}
		if err := r.stateRepo.Clear(); err != nil {
			return nil, err
		}
	}
	result := r.buildResult(!recoveredKnowledge)
	if err := recordLearnJournal(r.cont, result, r.startedAt, r.opts.scopeKind); err != nil {
		logger.Warn(i18n.GetWithParams("LearnJournalWriteFailed", map[string]interface{}{"Error": err.Error()}))
	}
	return result, nil
}

func (r *learnCurrentProjectRun) buildResult(skipped bool) *learnCurrentProjectResult {
	changedCount := len(r.incrementalChanges.AddedOrModified)
	deletedCount := len(r.incrementalChanges.Deleted)
	if state := r.recoveredSourceState(); state != nil {
		changedCount = len(state.Files)
		deletedCount = len(state.Deleted)
	}
	focusCount := 0
	if r.analysisState != nil {
		focusCount = len(r.analysisState.Agenda.Focuses)
	}
	result := &learnCurrentProjectResult{
		projectName:   r.projectName,
		changedCount:  changedCount,
		deletedCount:  deletedCount,
		skippedCount:  len(r.incrementalChanges.Skipped),
		patternsCount: r.resultPatternCount(),
		savedCount:    r.savedCount,
		retiredCount:  r.retiredCount,
		droppedCount:  len(r.dropped),
		dropped:       append([]patternnorm.Drop(nil), r.dropped...),
		skipped:       skipped,
		duration:      time.Since(r.startedAt),
		focusCount:    focusCount,
		changeProfile: string(r.changeProfile),
		learningMode:  r.learningMode,
		analysisMode:  r.observer.analysisModeValue(),
		resumed:       r.stateSession != nil,
		skippedStages: r.observer.skippedList(),
	}
	result.metrics = r.observer.buildJournalMetrics(result, result.duration)
	return result
}

func (r *learnCurrentProjectRun) recoveredSourceState() *commandstate.State {
	if r.stateSession == nil || r.stateSession.State == nil || !r.stateSession.State.SourceBaselineCommitComplete() {
		return nil
	}
	return r.stateSession.State
}

func resolveIncrementalFocusPaths(projectRoot string, relPaths []string) []string {
	paths := make([]string, 0, len(relPaths))
	for _, relPath := range relPaths {
		paths = append(paths, filepath.Join(projectRoot, filepath.FromSlash(relPath)))
	}
	return paths
}

func intersectPaths(paths []string, allowed []string) []string {
	allowedSet := make(map[string]bool, len(allowed))
	for _, path := range allowed {
		allowedSet[filepath.ToSlash(filepath.Clean(path))] = true
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized := filepath.ToSlash(filepath.Clean(path))
		if allowedSet[normalized] {
			out = append(out, path)
		}
	}
	return out
}

func recordLearnCurrentSummary(change *changelog.Builder, result domain.LearnCurrentResult) {
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
		"Retired":  summary.PatternsRetired,
	}))
	if summary.PatternsDropped > 0 {
		change.Detail(i18n.GetWithParams("ChangeLogLearnDroppedPatterns", map[string]interface{}{
			"Count":   summary.PatternsDropped,
			"Reasons": strings.Join(summary.DropReasons, "; "),
		}))
	}
}

func recordLearnJournal(cont *container.Container, result *learnCurrentProjectResult, startedAt time.Time, scopeKind runjournal.ScopeKind) error {
	if cont == nil || cont.ConfigRepo == nil || result == nil {
		return nil
	}
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	scope := runjournal.Scope{
		Kind:        runjournal.ScopeProject,
		Name:        result.projectName,
		ProjectPath: projectConfig.RootPath,
	}
	if scopeKind != "" {
		scope.Kind = scopeKind
	} else if projectConfig.Mode == domain.ModeWorkspace {
		scope.Kind = runjournal.ScopeChild
	}
	summary := i18n.Get("ChangeLogSummaryLearnCurrent")
	details := []string{
		i18n.GetWithParams("LearnJournalSummaryCounts", map[string]interface{}{
			"Changed":  result.changedCount,
			"Deleted":  result.deletedCount,
			"Skipped":  result.skippedCount,
			"Patterns": result.patternsCount,
			"Saved":    result.savedCount,
			"Retired":  result.retiredCount,
		}),
	}
	if result.droppedCount > 0 {
		details = append(details, i18n.GetWithParams("LearnJournalDroppedPatterns", map[string]interface{}{
			"Count":   result.droppedCount,
			"Reasons": strings.Join(dropReasonSummaries(result.dropped), "; "),
		}))
	}
	if result.metrics != nil {
		if result.metrics.AgentCallTotal > 0 || len(result.metrics.SkippedStages) > 0 {
			details = append(details, i18n.GetWithParams("LearnJournalRunMetrics", map[string]interface{}{
				"WallMs":        result.metrics.WallMs,
				"AgentCalls":    result.metrics.AgentCallTotal,
				"AnalysisMode":  emptyMetricDash(result.metrics.AnalysisMode),
				"SkippedStages": emptyMetricDash(strings.Join(result.metrics.SkippedStages, ",")),
			}))
		}
	}
	return runjournal.Append(cont.SeedPath, runjournal.Entry{
		Command:    "learn current",
		Scope:      scope,
		Summary:    summary,
		Details:    details,
		LogPath:    currentRunLogPath(),
		Metrics:    result.metrics,
		StartedAt:  startedAt,
		FinishedAt: time.Now(),
	})
}

func emptyMetricDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func currentRunLogPath() string {
	if path := logger.CurrentScopedLogPath(); path != "" {
		return path
	}
	return logger.CurrentLogPath()
}

func resolveFocusPaths(projectRoot string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	projectAbs = filepath.Clean(projectAbs)

	resolved := make([]string, 0, len(paths))
	seen := make(map[string]bool)
	for _, rawPath := range paths {
		rawPath = strings.TrimSpace(rawPath)
		if rawPath == "" {
			continue
		}

		path, err := projectpath.Resolve(projectAbs, rawPath)
		if err != nil {
			return nil, err
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		path = filepath.Clean(path)

		relPath, err := filepath.Rel(projectAbs, path)
		if err != nil {
			return nil, err
		}
		if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentFocusOutsideRoot", map[string]interface{}{"Path": rawPath}))
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.GetWithParams("LearnCurrentFocusNotAccessible", map[string]interface{}{"Path": rawPath}), err)
		}
		if seen[path] {
			continue
		}
		resolved = append(resolved, path)
		seen[path] = true
	}
	return resolved, nil
}

func shouldRefreshProfile(mode string, profileExists bool, storedAuthorityRevision, currentAuthorityRevision string) (bool, error) {
	switch mode {
	case "", learnCurrentProfileAuto:
		return !profileExists || storedAuthorityRevision != currentAuthorityRevision, nil
	case learnCurrentProfileSkip:
		return false, nil
	case learnCurrentProfileRefresh:
		return true, nil
	default:
		return false, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileModeInvalid", map[string]interface{}{"Mode": mode}))
	}
}

func profileAuthorityRevision(profile *domain.ProjectProfile) string {
	if profile == nil {
		return ""
	}
	return strings.TrimSpace(profile.AuthorityRevision)
}
