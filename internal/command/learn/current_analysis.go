package learn

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/learner"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

type learnCurrentFocusResult struct {
	index            int
	focus            domain.EvidenceFocus
	patterns         []domain.Pattern
	refreshRecommend agent.ProfileRefreshRecommendation
	completed        bool
}

type learnCurrentBatch struct {
	index   int
	focuses []indexedEvidenceFocus
}

type indexedEvidenceFocus struct {
	index int
	focus domain.EvidenceFocus
}

func (r *learnCurrentProjectRun) planLearningAgenda() error {
	planStartedAt := time.Now()
	planLabel := i18n.Get("ProgressLearnCurrentPlanFocuses")
	if r.stateSession != nil {
		planLabel = i18n.GetWithParams("ProgressLearnCurrentPlanFocusesRestored", map[string]interface{}{
			"Focuses": len(r.stateSession.State.Agenda.Focuses),
		})
	}
	if err := r.steps.Run(planLabel, func() error {
		focusRelPaths := analysisCandidatePaths(r.incrementalChanges)
		state := (*commandstate.State)(nil)
		if r.stateSession != nil {
			state = r.stateSession.State
		}
		if state == nil {
			var err error
			state, err = loadOrCreateCurrentState(r.ctx, r.stateRepo, r.cont.AnalyzerSvc, r.activeLearningSession, r.projectName, r.projectRoot, r.currentLanguage, r.learningMode, r.learningScope, focusRelPaths, r.incrementalChanges, currentStateInputSummary(r.incrementalChanges, r.selectionPlan, r.selectionSummary), r.changeProfile, r.opts.userContext, r.currentStateInvocationHash())
			if err != nil {
				return err
			}
		}
		r.analysisState = state
		r.plannedFocuses = pendingEvidenceFocuses(state, r.incrementalChanges)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.plan_learning_agenda",
			"duration", time.Since(planStartedAt),
			"focuses_count", len(state.Agenda.Focuses),
			"pending_focuses_count", len(r.plannedFocuses),
			"candidate_count", len(focusRelPaths),
		)
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.plan_learning_agenda",
			"duration", time.Since(planStartedAt),
			"error", err,
		)
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToAnalyzeCodebase", map[string]interface{}{"Error": err.Error()}))
	}
	return nil
}

func (r *learnCurrentProjectRun) analyzeCodebase() error {
	// AI 分析是 learn current 最耗时的步骤，进度行会持续刷新当前耗时
	analyzeStartedAt := time.Now()
	analyzeLabel := i18n.Get("ProgressLearnCurrentAnalyzeCodebase")
	if err := r.steps.Run(analyzeLabel, func() error {
		if r.analysisState == nil {
			return nil
		}
		r.restoreAnalysisCheckpoint()
		if len(r.plannedFocuses) == 0 {
			return r.completeAnalysis()
		}
		if r.analysisArtifactsCommitted() {
			return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentArtifactsCommittedWithPendingFocuses", map[string]interface{}{"Count": len(r.plannedFocuses)}))
		}
		runContext, err := r.buildCodebaseRunContext()
		if err != nil {
			return err
		}
		r.codebaseRunContext = runContext
		_, err = r.analyzePlannedFocuses(analyzeLabel, r.analysisState, r.plannedFocuses)
		if err != nil {
			return err
		}
		return r.completeAnalysis()
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.analyze_codebase",
			"duration", time.Since(analyzeStartedAt),
			"error", err,
		)
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToAnalyzeCodebase", map[string]interface{}{"Error": err.Error()}))
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.analyze_codebase",
		"duration", time.Since(analyzeStartedAt),
		"patterns_count", len(r.patterns),
		"profile_refresh_recommended", r.profileRefreshRecommended.Needed,
	)

	if r.opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentResult", map[string]interface{}{
			"PatternsCount": len(r.patterns),
		}))
	}
	return nil
}

func (r *learnCurrentProjectRun) analyzePlannedFocuses(analyzeLabel string, state *commandstate.State, plannedFocuses []domain.EvidenceFocus) (int, error) {
	batches := r.planAnalysisBatches(plannedFocuses)
	completedFocuses, err := r.analyzePlannedBatchesSerial(analyzeLabel, state, batches)
	if err != nil {
		return completedFocuses, err
	}
	logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentAnalyzeFocusesSummary", map[string]interface{}{
		"Completed":   completedFocuses,
		"Total":       len(plannedFocuses),
		"Batches":     len(batches),
		"Parallelism": 1,
	}))
	return completedFocuses, nil
}

func (r *learnCurrentProjectRun) buildCodebaseRunContext() (*analyzer.CodebaseRunContext, error) {
	return r.cont.AnalyzerSvc.BuildCodebaseRunContext(r.ctx, r.projectRoot, r.currentLanguage, analyzer.AnalyzeCodebaseOptions{
		FocusPaths:       r.effectiveFocusPaths,
		SelectedFiles:    r.selectedFiles,
		SelectedFilesSet: true,
		UseSnapshotDiffs: true,
	})
}

func (r *learnCurrentProjectRun) planAnalysisBatches(plannedFocuses []domain.EvidenceFocus) []learnCurrentBatch {
	maxFocuses := r.maxFocusesPerBatch()
	if maxFocuses < 1 {
		maxFocuses = 1
	}
	batches := make([]learnCurrentBatch, 0, (len(plannedFocuses)+maxFocuses-1)/maxFocuses)
	for start := 0; start < len(plannedFocuses); start += maxFocuses {
		end := start + maxFocuses
		if end > len(plannedFocuses) {
			end = len(plannedFocuses)
		}
		batch := learnCurrentBatch{index: len(batches)}
		for i := start; i < end; i++ {
			batch.focuses = append(batch.focuses, indexedEvidenceFocus{index: i, focus: plannedFocuses[i]})
		}
		batches = append(batches, batch)
	}
	return batches
}

func (r *learnCurrentProjectRun) maxFocusesPerBatch() int {
	maxFocuses := r.cont.ConfigRepo.GetCurrentLearningConfig().MaxFocusesPerCall
	if maxFocuses < 1 {
		return 1
	}
	return maxFocuses
}

func (r *learnCurrentProjectRun) analyzePlannedBatchesSerial(analyzeLabel string, state *commandstate.State, batches []learnCurrentBatch) (int, error) {
	completedFocuses := 0
	for _, batch := range batches {
		results, err := r.analyzeBatch(r.ctx, analyzeLabel, state, batch, true)
		if err != nil {
			return completedFocuses, err
		}
		completedFocuses += r.mergeFocusResults(results)
		if err := r.saveAnalysisCheckpoint(); err != nil {
			return completedFocuses, err
		}
	}
	return completedFocuses, nil
}

func (r *learnCurrentProjectRun) focusProgressParams(state *commandstate.State, focus domain.EvidenceFocus, current, total int) map[string]interface{} {
	currentFocus, allFocuses := learnCurrentFocusProgress(state, current, total, focus)
	return map[string]interface{}{
		"Current": currentFocus,
		"Total":   allFocuses,
		"Name":    learnCurrentProgressSubject(focus),
	}
}

func (r *learnCurrentProjectRun) analyzeBatch(ctx context.Context, analyzeLabel string, state *commandstate.State, batch learnCurrentBatch, showDetails bool) ([]learnCurrentFocusResult, error) {
	var batchFocuses []analyzer.AnalyzeCurrentEvidenceFocus
	results := make([]learnCurrentFocusResult, 0, len(batch.focuses))
	pendingByID := make(map[string]indexedEvidenceFocus, len(batch.focuses))
	pendingByName := make(map[string]indexedEvidenceFocus, len(batch.focuses))
	progressLabelByID := make(map[string]string, len(batch.focuses))
	for _, indexed := range batch.focuses {
		focusRelPaths := evidenceFocusPaths(indexed.focus, r.incrementalChanges)
		if len(focusRelPaths) == 0 {
			results = append(results, learnCurrentFocusResult{index: indexed.index, focus: indexed.focus})
			continue
		}
		params := r.batchFocusProgressParams(state, indexed)
		progressLabel := learnCurrentProgressDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeFocus", params)
		if showDetails {
			progressLabel = r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeFocus", params)
		}
		batchFocuses = append(batchFocuses, analyzer.AnalyzeCurrentEvidenceFocus{
			EvidenceFocus: indexed.focus,
			FocusAbsPaths: resolveIncrementalFocusPaths(r.projectRoot, focusRelPaths),
		})
		pendingByID[indexed.focus.ID] = indexed
		pendingByName[indexed.focus.Name] = indexed
		progressLabelByID[indexed.focus.ID] = progressLabel
	}
	if len(batchFocuses) == 0 {
		return results, nil
	}
	stage := learningStagePackAnalysis
	if r.useDeltaAnalysis() {
		stage = learningStageDeltaAnalysis
	}

	batchLabel := r.analysisBatchRuntimeLabel(state, batch)
	var analyzeResult *analyzer.AnalyzeCurrentCodebaseBatchResult
	err := r.withLearningSession(stage, func(session agent.LearningSession) error {
		if r.useDeltaAnalysis() {
			deltaResults, err := r.analyzeDeltaBatch(ctx, batch, batchFocuses, session)
			if err != nil {
				return err
			}
			results = append(results, deltaResults...)
			return nil
		}
		var err error
		analyzeResult, err = r.cont.AnalyzerSvc.AnalyzeCurrentCodebaseBatch(ctx, r.projectRoot, r.projectName, r.currentLanguage, analyzer.AnalyzeCurrentCodebaseBatchOptions{
			RuntimeLabel:    batchLabel,
			LearningMode:    r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
			ChangeProfile:   string(r.changeProfile),
			RunContext:      r.codebaseRunContext,
			LearningSession: session,
			Focuses:         batchFocuses,
		})
		return err
	})
	if err != nil {
		if len(batchFocuses) == 1 {
			focusID := batchFocuses[0].EvidenceFocus.ID
			if progressLabel := progressLabelByID[focusID]; progressLabel != "" {
				return nil, fmt.Errorf("%s: %w", progressLabel, err)
			}
		}
		return nil, err
	}
	if r.useDeltaAnalysis() {
		sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
		return results, nil
	}

	seen := make(map[string]bool, len(analyzeResult.Focuses))
	for _, focusResult := range analyzeResult.Focuses {
		indexed, ok := pendingByID[focusResult.EvidenceFocus.ID]
		if !ok && focusResult.EvidenceFocus.Name != "" {
			indexed, ok = pendingByName[focusResult.EvidenceFocus.Name]
		}
		if !ok {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentAnalyzeBatchUnknownFocus", map[string]interface{}{"Focus": focusResult.EvidenceFocus.ID}))
		}
		result := buildAnalyzedFocusResult(indexed.focus, indexed.index, focusResult.Patterns, focusResult.ProfileRefreshRecommended)
		results = append(results, result)
		seen[indexed.focus.ID] = true
	}
	for _, indexed := range batch.focuses {
		focusRelPaths := evidenceFocusPaths(indexed.focus, r.incrementalChanges)
		if len(focusRelPaths) == 0 {
			continue
		}
		if !seen[indexed.focus.ID] {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentAnalyzeBatchMissedFocus", map[string]interface{}{"Focus": indexed.focus.ID}))
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
	return results, nil
}

func (r *learnCurrentProjectRun) batchFocusProgressParams(state *commandstate.State, indexed indexedEvidenceFocus) map[string]interface{} {
	total := indexed.index + 1
	if state != nil && len(state.Agenda.Focuses) > 0 {
		total = len(state.Agenda.Focuses)
	}
	return r.focusProgressParams(state, indexed.focus, indexed.index+1, total)
}

func (r *learnCurrentProjectRun) analysisBatchRuntimeLabel(state *commandstate.State, batch learnCurrentBatch) string {
	index := batch.index
	if state != nil && len(state.Agenda.Focuses) > 0 && len(batch.focuses) > 0 {
		minAgendaIndex := len(state.Agenda.Focuses)
		for _, item := range batch.focuses {
			for agendaIndex, focus := range state.Agenda.Focuses {
				if evidenceFocusSame(focus, item.focus) && agendaIndex < minAgendaIndex {
					minAgendaIndex = agendaIndex
					break
				}
			}
		}
		if minAgendaIndex < len(state.Agenda.Focuses) {
			index = minAgendaIndex / r.maxFocusesPerBatch()
		}
	}
	return fmt.Sprintf("batch-%03d", index+1)
}

func buildAnalyzedFocusResult(focus domain.EvidenceFocus, index int, learnedPatterns []domain.Pattern, refreshRecommend agent.ProfileRefreshRecommendation) learnCurrentFocusResult {
	return learnCurrentFocusResult{
		index:            index,
		focus:            focus,
		patterns:         learnedPatterns,
		refreshRecommend: refreshRecommend,
		completed:        true,
	}
}

func (r *learnCurrentProjectRun) mergeFocusResults(results []learnCurrentFocusResult) int {
	completed := 0
	for _, result := range results {
		if result.completed {
			completed++
		}
		r.mergeFocusResult(result)
	}
	return completed
}

func (r *learnCurrentProjectRun) commitCurrentAnalysis(ctx context.Context) error {
	if r.codebaseRunContext != nil && r.codebaseRunContext.SnapshotFlow != nil {
		if err := r.codebaseRunContext.SnapshotFlow.CommitScoped(analysisCandidatePaths(r.incrementalChanges)); err != nil {
			return err
		}
	}
	return fileanalysis.CommitCurrentChanges(ctx, r.cont.FileTracker, r.incrementalChanges)
}

func (r *learnCurrentProjectRun) mergeFocusResult(result learnCurrentFocusResult) {
	if len(result.patterns) > 0 {
		r.patterns = append(r.patterns, result.patterns...)
	}
	if result.completed {
		r.completedEvidenceFocuses = append(r.completedEvidenceFocuses, result.focus)
	}
	if result.refreshRecommend.Needed {
		r.profileRefreshRecommended = result.refreshRecommend
	}
}

func (r *learnCurrentProjectRun) curateAndSavePatternsStep() error {
	startedAt := time.Now()
	stepLabel := i18n.Get("ProgressLearnCurrentCurateAndSavePatterns")
	if err := r.steps.Run(stepLabel, func() error {
		if !r.analysisArtifactsCommitted() && len(r.patterns) > 0 {
			hooks := curator.ProgressHooks{
				OnStepStart: func(label string) {
					r.patternStageDetail(stepLabel, label)
				},
				OnStepUpdate: func(label string) {
					r.patternStageDetail(stepLabel, label)
				},
				OnValidationStart: func(label string) {
					r.patternStageDetail(stepLabel, label)
				},
				OnStoreStart: func(label string) {
					r.patternStageDetail(stepLabel, label)
				},
			}
			checkpoint := newCurrentCurationCheckpoint(r.stateRepo, r.analysisState, r.importedCuration)
			saved := 0
			err := r.withLearningSession(learningStageGlobalCuration, func(session agent.LearningSession) error {
				var err error
				saved, err = r.cont.LearnerSvc.CurateAndSavePatternsWithOptions(r.ctx, r.patterns, curator.OperationLearnCurrent, learner.CurateOptions{
					Hooks:              hooks,
					DecisionCheckpoint: checkpoint,
					LearningSession:    session,
				})
				return err
			})
			if err != nil {
				return err
			}
			r.savedCount = saved
		}
		if !r.analysisArtifactsCommitted() && r.analysisState != nil {
			r.analysisState.ArtifactsCommitted = true
			if err := r.stateRepo.Save(r.ctx, r.analysisState); err != nil {
				return err
			}
		}
		r.detail(stepLabel, "ProgressLearnCurrentCommitFiles", map[string]interface{}{
			"Count": len(r.incrementalChanges.Records) + len(r.incrementalChanges.Deleted),
		})
		if err := r.commitCurrentAnalysis(r.ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.curate_and_save_patterns",
		"duration", time.Since(startedAt),
		"patterns_count", len(r.patterns),
		"saved_count", r.savedCount,
	)
	if r.opts.showDetailedLogs && len(r.patterns) > 0 {
		logger.Info(i18n.GetWithParams("LearnCurrentPatternsSaved", map[string]interface{}{"Count": r.savedCount}))
	}
	return nil
}

func (r *learnCurrentProjectRun) completeAnalysis() error {
	if err := r.validateCompletedAnalysis(); err != nil {
		return err
	}
	return r.saveAnalysisCheckpoint()
}

func (r *learnCurrentProjectRun) validateCompletedAnalysis() error {
	missing := make([]string, 0)
	if r.analysisState != nil {
		for _, focus := range r.analysisState.Agenda.Focuses {
			if len(evidenceFocusPaths(focus, r.incrementalChanges)) == 0 || evidenceFocusIncluded(r.completedEvidenceFocuses, focus) {
				continue
			}
			missing = append(missing, learnCurrentProgressSubject(focus))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentAgendaIncomplete", map[string]interface{}{"Focuses": strings.Join(missing, ", ")}))
	}
	uncovered := uncoveredAnalysisPaths(r.completedEvidenceFocuses, analysisCandidatePaths(r.incrementalChanges))
	if len(uncovered) == 0 {
		return nil
	}
	return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentAgendaCoverageMissing", map[string]interface{}{"Paths": strings.Join(uncovered, ", ")}))
}

func (r *learnCurrentProjectRun) patternStageDetail(baseLabel, detail string) {
	r.detail(baseLabel, "ProgressLearnCurrentPatternStageDetail", map[string]interface{}{
		"Detail": detail,
	})
}

func (r *learnCurrentProjectRun) analysisArtifactsCommitted() bool {
	return r.analysisState != nil && r.analysisState.ArtifactsCommitted
}

func (r *learnCurrentProjectRun) profileCommitted() bool {
	return r.analysisState != nil && r.analysisState.ProfileCommitted
}

func (r *learnCurrentProjectRun) saveAnalysisCheckpoint() error {
	if r.analysisState == nil {
		return nil
	}
	r.analysisState.Analysis = &commandstate.AnalysisCheckpoint{
		Patterns:             append([]domain.Pattern(nil), r.patterns...),
		CompletedFocuses:     append([]domain.EvidenceFocus(nil), r.completedEvidenceFocuses...),
		ProfileRefreshNeeded: r.profileRefreshRecommended.Needed,
		ProfileRefreshReason: r.profileRefreshRecommended.Reason,
	}
	return r.stateRepo.Save(r.ctx, r.analysisState)
}

func (r *learnCurrentProjectRun) restoreAnalysisCheckpoint() {
	if r.analysisState == nil || r.analysisState.Analysis == nil {
		return
	}
	checkpoint := r.analysisState.Analysis
	r.patterns = append(r.patterns, checkpoint.Patterns...)
	r.completedEvidenceFocuses = append(r.completedEvidenceFocuses, checkpoint.CompletedFocuses...)
	r.profileRefreshRecommended = agent.ProfileRefreshRecommendation{
		Needed: checkpoint.ProfileRefreshNeeded,
		Reason: checkpoint.ProfileRefreshReason,
	}
}

func (r *learnCurrentProjectRun) saveProfileIfNeeded() error {
	profileStartedAt := time.Now()
	if r.refreshProfile && !r.profileCommitted() && !r.analysisArtifactsCommitted() {
		label := r.afterAnalysisLabel(i18n.Get("ProgressLearnCurrentSaveProfile"))
		if err := r.steps.Run(label, func() error {
			profile, err := r.refreshProjectProfileWithSession()
			if err != nil {
				return err
			}
			if err := r.cont.ProfileRepo.Save(r.ctx, profile); err != nil {
				return err
			}
			if r.analysisState == nil {
				return nil
			}
			r.analysisState.ProfileCommitted = true
			return r.stateRepo.Save(r.ctx, r.analysisState)
		}); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "command.learn_current.save_project_profile",
				"duration", time.Since(profileStartedAt),
				"error", err,
			)
			return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentProfileFailed", map[string]interface{}{"Error": err.Error()}))
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.save_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", r.opts.profileMode,
			"incremental_profile", r.existingProfile != nil && len(r.resolvedFocusPaths) > 0,
		)
		if r.opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSaved"))
		}
	} else {
		label := r.afterAnalysisLabel(i18n.Get("ProgressLearnCurrentSkipProfile"))
		if err := r.steps.Run(label, func() error { return nil }); err != nil {
			return err
		}
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.skip_project_profile",
			"duration", time.Since(profileStartedAt),
			"profile_mode", r.opts.profileMode,
		)
		if r.opts.showDetailedLogs {
			logger.Info(i18n.Get("LearnCurrentProfileSkipped"))
		}
	}
	return nil
}

func (r *learnCurrentProjectRun) refreshProjectProfile() (*domain.ProjectProfile, error) {
	options := analyzer.AnalyzeProjectOptions{
		LearningSession: r.activeLearningSession,
	}
	if r.existingProfile != nil && len(r.effectiveFocusPaths) > 0 {
		options.ExistingProfile = r.existingProfile
		options.FocusPaths = r.effectiveFocusPaths
	}
	return r.cont.AnalyzerSvc.RefreshProjectProfile(r.ctx, r.projectRoot, r.projectName, r.currentLanguage, options)
}

func (r *learnCurrentProjectRun) refreshProjectProfileWithSession() (*domain.ProjectProfile, error) {
	var profile *domain.ProjectProfile
	err := r.withLearningSession(learningStageProfileRefresh, func(agent.LearningSession) error {
		var err error
		profile, err = r.refreshProjectProfile()
		return err
	})
	return profile, err
}

func (r *learnCurrentProjectRun) afterAnalysisLabel(label string) string {
	completed, total := r.evidenceFocusProgress()
	if total == 0 {
		return label
	}
	return i18n.GetWithParams("ProgressLearnCurrentAfterAnalysis", map[string]interface{}{
		"Label":     label,
		"Completed": completed,
		"Total":     total,
	})
}

func (r *learnCurrentProjectRun) evidenceFocusProgress() (int, int) {
	if r.analysisState == nil {
		return 0, 0
	}
	completed := 0
	total := 0
	for _, focus := range r.analysisState.Agenda.Focuses {
		if len(evidenceFocusPaths(focus, r.incrementalChanges)) == 0 {
			continue
		}
		total++
		if evidenceFocusIncluded(r.completedEvidenceFocuses, focus) {
			completed++
		}
	}
	return completed, total
}
