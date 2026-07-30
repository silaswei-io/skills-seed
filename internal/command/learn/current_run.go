package learn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

type currentFileSelectionPlan struct {
	Candidates []string
	SkipReason string
}

type learnCurrentProjectRun struct {
	cont      *container.Container
	opts      learnCurrentProjectOptions
	stateRepo *commandstate.Repository
	ctx       context.Context
	startedAt time.Time
	steps     *commandutil.ConsoleStepRunner

	projectRoot        string
	projectName        string
	currentLanguage    string
	learningMode       string
	learningScope      string
	resolvedFocusPaths []string
	refreshProfile     bool
	existingProfile    *domain.ProjectProfile

	incrementalChanges  *fileanalysis.FileChanges
	effectiveFocusPaths []string
	selectedFiles       []domain.FileInfo
	selectionSummary    fileSelectionSummary
	selectionPlan       currentFileSelectionPlan
	stateSession        *currentStateSession
	stateInvalidated    bool
	resumeSummary       *learnCurrentResumeSummary
	changeProfile       currentChangeProfile
	analysisState       *commandstate.State
	plannedFocuses      []domain.EvidenceFocus

	patterns                   []domain.Pattern
	profileRefreshRecommended  agent.ProfileRefreshRecommendation
	codebaseRunContext         *analyzer.CodebaseRunContext
	savedCount                 int
	completedEvidenceFocuses   []domain.EvidenceFocus
	importedCuration           *agent.CuratePatternsResult
	activeLearningSession      agent.LearningSession
	learningSessionCache       *currentLearningSessionCache
	progressDetailMu           sync.Mutex
	fileSelectionSummaryLogged bool
}

const (
	learningStagePlanning       = "planning"
	learningStagePackAnalysis   = "pack-analysis"
	learningStageDeltaAnalysis  = "delta-analysis"
	learningStageProfileRefresh = "profile-refresh"
	learningStageGlobalCuration = "global-curation"
)

func runLearnCurrentProjectWithOptions(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions) (*learnCurrentProjectResult, error) {
	run := newLearnCurrentProjectRun(ctx, cont, opts)
	if err := run.loadImportedCuration(); err != nil {
		return nil, err
	}
	if !run.hasCurationDecision() {
		if err := commandutil.RequireAgentAvailable(cont); err != nil {
			return nil, err
		}
	}
	return run.execute()
}

func newLearnCurrentProjectRun(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions) *learnCurrentProjectRun {
	ctx = runtimecontext.WithSeedPath(ctx, cont.SeedPath)
	ctx = runtimecontext.WithUserContext(ctx, opts.userContext)
	steps := commandutil.NewConsoleStepRunner(commandutil.ConsoleStepRunnerOptions{
		TotalSteps:     learnCurrentProjectStepTotal,
		ShowProgress:   opts.showProgress,
		OnStepStart:    opts.onStepStart,
		OnStepComplete: opts.onStepComplete,
		OnStepUpdate:   opts.onStepUpdate,
	})
	ctx = steps.WithContext(ctx)

	return &learnCurrentProjectRun{
		cont:      cont,
		opts:      opts,
		stateRepo: learnCurrentStateRepo(cont.SeedPath, opts.stateScope),
		ctx:       ctx,
		startedAt: time.Now(),
		steps:     steps,
	}
}

func (r *learnCurrentProjectRun) hasCurationDecision() bool {
	if r.importedCuration != nil {
		return true
	}
	state, err := r.stateRepo.Load(r.ctx)
	return err == nil && state.Curation != nil
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

	if err := r.prepareProject(); err != nil {
		return nil, err
	}
	if err := r.detectChanges(); err != nil {
		return nil, err
	}
	if err := r.runPlanningStage(); err != nil {
		return nil, err
	}
	if !r.incrementalChanges.HasChanges() {
		return r.finishWithoutChanges()
	}
	if err := r.analyzeCodebase(); err != nil {
		return nil, err
	}
	if r.opts.profileMode == learnCurrentProfileAuto && r.profileRefreshRecommended.Needed {
		r.refreshProfile = true
	}
	if err := r.curateAndSavePatternsStep(); err != nil {
		return nil, err
	}
	if err := r.saveProfileIfNeeded(); err != nil {
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
	)
	if err := commandutil.MarkLearned(r.ctx, r.cont); err != nil {
		return nil, err
	}
	if r.analysisState != nil {
		if err := r.stateRepo.Clear(); err != nil {
			return nil, err
		}
	}
	if err := clearCurrentLearningSessionCache(r.cont.SeedPath, r.stateRepo.Command()); err != nil {
		return nil, err
	}

	return r.buildResult(false), nil
}

func (r *learnCurrentProjectRun) runPlanningStage() error {
	if !r.incrementalChanges.HasChanges() || r.stateSession != nil {
		if err := r.narrowLearningCandidates(); err != nil {
			return err
		}
		r.logFileSelectionSummaryOnce()
		if !r.incrementalChanges.HasChanges() {
			return nil
		}
		return r.planLearningAgenda()
	}
	return r.withLearningSession(learningStagePlanning, func(agent.LearningSession) error {
		if err := r.narrowLearningCandidates(); err != nil {
			return err
		}
		r.logFileSelectionSummaryOnce()
		return r.planLearningAgenda()
	})
}

func (r *learnCurrentProjectRun) startLearningSession(stage string) error {
	resumeSessionID := ""
	if r.stateSession != nil {
		cache, err := loadCurrentLearningSessionCache(r.ctx, r.cont.SeedPath, r.stateRepo.Command())
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.Get("LearnCurrentLoadLearningSessionCacheFailed"), err)
		}
		if cache.matches(r.cont.Agent.Name(), r.currentStateInvocationHash()) && cache.Step == stage {
			r.learningSessionCache = cache
			resumeSessionID = cache.SessionID
		}
	}
	session, err := r.newLearningSession(r.ctx, stage, resumeSessionID)
	if err != nil {
		return err
	}
	r.activeLearningSession = session
	r.markLearningSessionStep(stage)
	return nil
}

func (r *learnCurrentProjectRun) newLearningSession(ctx context.Context, stage, resumeSessionID string) (agent.LearningSession, error) {
	session, err := r.cont.Agent.StartLearningSession(ctx, agent.LearningSessionRequest{
		ProjectName:     r.projectName,
		RootPath:        r.projectRoot,
		Language:        r.currentLanguage,
		Stage:           stage,
		LearningMode:    r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
		LearningScope:   r.cont.ConfigRepo.GetCurrentLearningConfig().Scope,
		ChangeProfile:   string(r.changeProfile),
		UserContext:     r.opts.userContext,
		UserContextPath: "",
		ResumeSessionID: resumeSessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("LearnCurrentStartLearningSessionFailed"), err)
	}
	if session == nil {
		return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentLearningSessionMissing", map[string]interface{}{"Agent": r.cont.Agent.Name()}))
	}
	return session, nil
}

func (r *learnCurrentProjectRun) ensureLearningSession(stage string) error {
	if r.activeLearningSession != nil {
		return nil
	}
	return r.startLearningSession(stage)
}

func (r *learnCurrentProjectRun) withLearningSession(stage string, run func(agent.LearningSession) error) (err error) {
	if r.activeLearningSession != nil {
		return run(r.activeLearningSession)
	}
	if err = r.ensureLearningSession(stage); err != nil {
		return err
	}
	defer func() {
		r.closeLearningSession()
		if err == nil {
			if clearErr := clearCurrentLearningSessionCache(r.cont.SeedPath, r.stateRepo.Command()); clearErr != nil {
				err = clearErr
			}
		}
	}()
	return run(r.activeLearningSession)
}

func (r *learnCurrentProjectRun) markLearningSessionStep(step string) {
	if r.activeLearningSession == nil {
		return
	}
	cache := currentLearningSessionCache{
		AgentName:      r.cont.Agent.Name(),
		SessionID:      r.activeLearningSession.SessionID(),
		Step:           step,
		InvocationHash: r.currentStateInvocationHash(),
	}
	if err := saveCurrentLearningSessionCache(r.ctx, r.cont.SeedPath, r.stateRepo.Command(), cache); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.save_learning_session_cache",
			"step", step,
			"error", err,
		)
		return
	}
	r.learningSessionCache = &cache
}

func (r *learnCurrentProjectRun) closeLearningSession() {
	if r.activeLearningSession == nil {
		return
	}
	if err := r.activeLearningSession.Close(r.ctx); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.close_learning_session",
			"error", err,
		)
	}
	r.activeLearningSession = nil
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
				return err
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
		r.learningScope = string(currentLearningConfig.Scope)

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
		r.refreshProfile, err = shouldRefreshProfile(r.projectRoot, r.resolvedFocusPaths, r.opts.profileMode, profileExists)
		return err
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.prepare_project",
			"duration", time.Since(prepareStartedAt),
			"error", err,
		)
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGetCurrentDir", map[string]interface{}{"Error": err.Error()}))
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
		r.changeProfile = currentChangeProfile(r.stateSession.State.ChangeProfile)
	}
	r.selectionPlan = r.buildFileSelectionPlan()
	return nil
}

func (r *learnCurrentProjectRun) hasRestorableCurrentState() bool {
	state, err := r.stateRepo.Load(r.ctx)
	if err != nil {
		return false
	}
	return canResumeCurrentState(state, r.projectName, r.currentLanguage, learnCurrentStateMode(r.learningMode, r.learningScope), r.opts.userContext, r.currentStateInvocationHash())
}

func (r *learnCurrentProjectRun) restoreOrDetectChanges(detectLabel string) error {
	r.detail(detectLabel, "ProgressLearnCurrentDetectRestoreState", nil)
	session, err := restoreCurrentState(r.ctx, r.stateRepo, r.cont.FileTracker, r.projectName, r.currentLanguage, learnCurrentStateMode(r.learningMode, r.learningScope), r.opts.userContext, r.currentStateInvocationHash())
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
			if err := clearCurrentLearningSessionCache(r.cont.SeedPath, r.stateRepo.Command()); err != nil {
				return err
			}
			session = nil
			r.stateInvalidated = true
			if r.opts.force {
				detected = nil
			}
		}
	}
	if session != nil {
		r.stateSession = session
		r.incrementalChanges = session.Changes
		focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
		r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, focusRelPaths)
		r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(focusRelPaths, r.incrementalChanges.AddedOrModified))
		r.resumeSummary = buildLearnCurrentResumeSummary(session)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.resume_state",
			"state_scope", r.stateRepo.Command(),
			"inputs_count", currentStateInputCount(session.State),
			"pending_count", len(r.incrementalChanges.AddedOrModified)+len(r.incrementalChanges.Deleted),
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
	return learnCurrentInvocationHash(r.cont.ConfigRepo, focusPaths, r.opts.profileMode, r.opts.force)
}

func (r *learnCurrentProjectRun) buildFileSelectionPlan() currentFileSelectionPlan {
	focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
	if r.stateSession != nil {
		return currentFileSelectionPlan{
			Candidates: focusRelPaths,
			SkipReason: i18n.Get("ProgressLearnCurrentFileSelectionSkipRestored"),
		}
	}
	if len(focusRelPaths) == 0 {
		return currentFileSelectionPlan{
			Candidates: focusRelPaths,
			SkipReason: i18n.Get("ProgressLearnCurrentFileSelectionSkipNoCandidates"),
		}
	}
	return currentFileSelectionPlan{Candidates: focusRelPaths}
}

func (r *learnCurrentProjectRun) narrowLearningCandidates() error {
	if r.stateSession != nil || len(r.selectionPlan.Candidates) == 0 {
		r.selectionSummary = fileSelectionSummary{
			Status: i18n.GetWithParams("LearnCurrentFileSelectionSkipped", map[string]interface{}{
				"Reason": r.selectionPlan.SkipReason,
			}),
		}
		return nil
	}
	selectStartedAt := time.Now()
	selectLabel := r.candidateSelectionProgressLabel()
	var selectionResult fileanalysis.LearningCandidateSelectionResult
	if err := r.steps.Run(selectLabel, func() error {
		selectionResult = r.selectLearningCandidates()
		return nil
	}); err != nil {
		return err
	}
	r.effectiveFocusPaths = resolveIncrementalFocusPaths(r.projectRoot, selectionResult.SelectedPaths)
	r.selectedFiles = fileanalysis.PathsToFileInfos(intersectPaths(selectionResult.SelectedPaths, r.incrementalChanges.AddedOrModified))
	r.incrementalChanges.ApplyLearningSelection(selectionResult.SelectedPaths, selectionResult.Reason)
	selectionStatus := strings.TrimSpace(selectionResult.Reason)
	if selectionStatus == "" {
		selectionStatus = i18n.Get("LearnCurrentFileSelectionApplied")
	}
	r.selectionSummary = fileSelectionSummary{
		Applied:        true,
		CandidateCount: len(r.selectionPlan.Candidates),
		SelectedCount:  len(selectionResult.SelectedPaths),
		SkippedCount:   len(selectionResult.SkippedPaths),
		Reason:         selectionResult.Reason,
		Status:         selectionStatus,
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.select_learning_candidates",
		"duration", time.Since(selectStartedAt),
		"candidate_count", len(r.selectionPlan.Candidates),
		"selected_count", len(selectionResult.SelectedPaths),
		"skipped_count", len(selectionResult.SkippedPaths),
		"fingerprint_record_count", len(r.incrementalChanges.Records),
	)
	return nil
}

func (r *learnCurrentProjectRun) candidateSelectionProgressLabel() string {
	if r.shouldRunAICandidateSelection() {
		return i18n.Get("ProgressLearnCurrentAIFileSelection")
	}
	return i18n.Get("ProgressLearnCurrentLocalFileSelection")
}

func (r *learnCurrentProjectRun) selectLearningCandidates() fileanalysis.LearningCandidateSelectionResult {
	if r.shouldRunAICandidateSelection() {
		return r.selectLearningCandidatesWithAI()
	}
	result := fileanalysis.SelectLearningCandidates(fileanalysis.LearningCandidateSelectionOptions{
		Candidates:    r.selectionPlan.Candidates,
		Changes:       r.incrementalChanges,
		RequiredPaths: r.learningCandidateRequiredPaths(),
	})
	result.Reason = i18n.Get("LearnCurrentFileSelectionLocalReason")
	return result
}

func (r *learnCurrentProjectRun) learningCandidateRequiredPaths() []string {
	required := projectpath.Relative(r.projectRoot, r.resolvedFocusPaths)
	if r.stateInvalidated {
		required = append(required, r.selectionPlan.Candidates...)
	}
	return normalizeStatePaths(required)
}

func (r *learnCurrentProjectRun) shouldRunAICandidateSelection() bool {
	if r.cont == nil || r.cont.AnalyzerSvc == nil || r.activeLearningSession == nil {
		return false
	}
	cfg := r.cont.ConfigRepo.GetCurrentLearningConfig()
	return cfg.SelectRelevantFiles && len(r.selectionPlan.Candidates) >= cfg.SelectRelevantFilesMinCandidates
}

func (r *learnCurrentProjectRun) selectLearningCandidatesWithAI() fileanalysis.LearningCandidateSelectionResult {
	candidates := normalizeStatePaths(r.selectionPlan.Candidates)
	required := r.learningCandidateRequiredPaths()
	seedPaths := fileanalysis.SelectLearningContextSeeds(fileanalysis.LearningCandidateSelectionOptions{
		Candidates:    candidates,
		Changes:       r.incrementalChanges,
		RequiredPaths: required,
	})
	selectLabel := r.candidateSelectionProgressLabel()
	result, err := r.cont.AnalyzerSvc.SelectLearningCandidates(r.ctx, &analyzer.SelectLearningCandidatesRequest{
		ProjectName:         r.projectName,
		RootPath:            r.projectRoot,
		Language:            r.currentLanguage,
		LearningMode:        r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
		LearningScope:       r.cont.ConfigRepo.GetCurrentLearningConfig().Scope,
		CandidatePaths:      candidates,
		RequiredPaths:       required,
		StructuralSeedPaths: seedPaths,
		UserContext:         r.opts.userContext,
		Progress: func(stage analyzer.SelectLearningCandidatesStage) {
			switch stage {
			case analyzer.SelectLearningCandidatesStageStructuralContext:
				r.detail(selectLabel, "ProgressLearnCurrentCandidateSelectionStructuralContext", map[string]interface{}{
					"SeedPaths": len(seedPaths),
				})
			case analyzer.SelectLearningCandidatesStageCodeGraphIndex:
				r.detail(selectLabel, "ProgressLearnCurrentCandidateSelectionCodeGraphIndex", map[string]interface{}{
					"SeedPaths": len(seedPaths),
				})
			case analyzer.SelectLearningCandidatesStageCodeGraphContext:
				r.detail(selectLabel, "ProgressLearnCurrentCandidateSelectionCodeGraphContext", map[string]interface{}{
					"SeedPaths": len(seedPaths),
				})
			case analyzer.SelectLearningCandidatesStageCodeGraphRepair:
				r.detail(selectLabel, "ProgressLearnCurrentCandidateSelectionCodeGraphRepair", map[string]interface{}{
					"SeedPaths": len(seedPaths),
				})
			case analyzer.SelectLearningCandidatesStageTreeSitterContext:
				r.detail(selectLabel, "ProgressLearnCurrentCandidateSelectionTreeSitterContext", map[string]interface{}{
					"SeedPaths": len(seedPaths),
				})
			case analyzer.SelectLearningCandidatesStageAgent:
				r.detail(selectLabel, "ProgressLearnCurrentCandidateSelectionAgent", map[string]interface{}{
					"CandidatePaths": len(candidates),
				})
			}
		},
		LearningSession: r.activeLearningSession,
	})
	if err != nil {
		return fileanalysis.LearningCandidateSelectionResult{
			SelectedPaths: candidates,
			SkippedPaths:  nil,
			Reason: i18n.GetWithParams("LearnCurrentFileSelectionAIFailed", map[string]interface{}{
				"Error": err.Error(),
			}),
		}
	}
	selected := sanitizeSelectedLearningCandidates(candidates, result.SelectedPaths, required)
	if len(selected) == 0 {
		selected = candidates
	}
	return fileanalysis.LearningCandidateSelectionResult{
		SelectedPaths: selected,
		SkippedPaths:  subtractStatePaths(candidates, selected),
		Reason: i18n.GetWithParams("LearnCurrentFileSelectionAIReason", map[string]interface{}{
			"Reason": strings.TrimSpace(result.Reason),
		}),
	}
}

func sanitizeSelectedLearningCandidates(candidates, selected, required []string) []string {
	allowed := pathSet(candidates)
	out := make(map[string]bool, len(selected)+len(required))
	for _, path := range normalizeStatePaths(selected) {
		if allowed[path] {
			out[path] = true
		}
	}
	for _, path := range normalizeStatePaths(required) {
		if allowed[path] {
			out[path] = true
		}
	}
	return sortedBoolPaths(out)
}

func (r *learnCurrentProjectRun) finishWithoutChanges() (*learnCurrentProjectResult, error) {
	recoveredArtifacts := r.stateSession != nil && r.stateSession.State.ArtifactsCommitted
	needsProfileRefresh := r.refreshProfile && !r.profileCommitted()
	if r.opts.showDetailedLogs {
		logger.Info(i18n.Get("LearnCurrentNoFileChanges"))
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentPlanFocuses"), func() error { return nil }); err != nil {
		return nil, err
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentAnalyzeCodebase"), func() error { return nil }); err != nil {
		return nil, err
	}
	if err := r.steps.Run(i18n.Get("ProgressLearnCurrentCurateAndSavePatterns"), func() error { return nil }); err != nil {
		return nil, err
	}
	profileStartedAt := time.Now()
	profileStep := i18n.Get("ProgressLearnCurrentSkipProfile")
	if needsProfileRefresh {
		profileStep = i18n.Get("ProgressLearnCurrentSaveProfile")
	}
	if err := r.steps.Run(profileStep, func() error {
		if !needsProfileRefresh {
			return nil
		}
		profile, err := r.refreshProjectProfileWithSession()
		if err != nil {
			return err
		}
		return r.cont.ProfileRepo.Save(r.ctx, profile)
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		"patterns_count", 0,
		"saved_count", 0,
		"skipped", true,
	)
	if recoveredArtifacts {
		if err := commandutil.MarkLearned(r.ctx, r.cont); err != nil {
			return nil, err
		}
		if err := r.stateRepo.Clear(); err != nil {
			return nil, err
		}
	}
	if r.learningSessionCache != nil {
		if err := clearCurrentLearningSessionCache(r.cont.SeedPath, r.stateRepo.Command()); err != nil {
			return nil, err
		}
	}
	return r.buildResult(true), nil
}

func (r *learnCurrentProjectRun) buildResult(skipped bool) *learnCurrentProjectResult {
	result := &learnCurrentProjectResult{
		projectName:   r.projectName,
		changedCount:  len(r.incrementalChanges.AddedOrModified),
		deletedCount:  len(r.incrementalChanges.Deleted),
		skippedCount:  len(r.incrementalChanges.Skipped),
		patternsCount: len(r.patterns),
		savedCount:    r.savedCount,
		skipped:       skipped,
		duration:      time.Since(r.startedAt),
	}
	return result
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
	}))
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

func shouldRefreshProfile(projectRoot string, focusPaths []string, mode string, profileExists bool) (bool, error) {
	switch mode {
	case "", learnCurrentProfileAuto:
		return !profileExists, nil
	case learnCurrentProfileSkip:
		return false, nil
	case learnCurrentProfileRefresh:
		return true, nil
	default:
		return false, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileModeInvalid", map[string]interface{}{"Mode": mode}))
	}
}
