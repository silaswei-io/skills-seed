package learn

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
)

type learnCurrentUnitResult struct {
	index            int
	unit             domain.AnalysisUnit
	patterns         []domain.Pattern
	refreshRecommend agent.ProfileRefreshRecommendation
	completed        bool
}

type learnCurrentBatch struct {
	index int
	units []indexedAnalysisUnit
}

type indexedAnalysisUnit struct {
	index int
	unit  domain.AnalysisUnit
}

func (r *learnCurrentProjectRun) planAnalysisUnits() error {
	planStartedAt := time.Now()
	planLabel := i18n.Get("ProgressLearnCurrentPlanUnits")
	if r.stateSession != nil {
		planLabel = i18n.GetWithParams("ProgressLearnCurrentPlanUnitsRestored", map[string]interface{}{
			"Units": len(r.stateSession.State.Units),
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
			state, err = loadOrCreateCurrentState(r.ctx, r.stateRepo, r.cont.AnalyzerSvc, r.projectName, r.projectRoot, r.currentLanguage, r.learningMode, r.learningScope, focusRelPaths, r.incrementalChanges, currentStateInputSummary(r.incrementalChanges, r.selectionPlan, r.selectionSummary), r.opts.userContext, r.currentStateInvocationHash())
			if err != nil {
				return err
			}
		}
		r.analysisState = state
		r.plannedUnits = pendingAnalysisUnits(state, r.incrementalChanges)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
			"operation", "command.learn_current.plan_analysis_units",
			"duration", time.Since(planStartedAt),
			"units_count", len(state.Units),
			"pending_units_count", len(r.plannedUnits),
			"candidate_count", len(focusRelPaths),
		)
		return nil
	}); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "command.learn_current.plan_analysis_units",
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
		if r.restoreAnalysisCheckpoint() {
			var err error
			r.codebaseRunContext, err = r.buildCodebaseRunContext()
			return err
		}
		if len(r.plannedUnits) == 0 {
			return r.saveAnalysisCheckpoint(true)
		}
		if r.analysisArtifactsCommitted() {
			runContext, err := r.buildCodebaseRunContext()
			if err != nil {
				return err
			}
			r.codebaseRunContext = runContext
			r.completedAnalysisUnits = append(r.completedAnalysisUnits, r.plannedUnits...)
			return nil
		}
		runContext, err := r.buildCodebaseRunContext()
		if err != nil {
			return err
		}
		r.codebaseRunContext = runContext
		_, err = r.analyzePlannedUnits(analyzeLabel, r.analysisState, r.plannedUnits)
		if err != nil {
			return err
		}
		return r.saveAnalysisCheckpoint(true)
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

func (r *learnCurrentProjectRun) analyzePlannedUnits(analyzeLabel string, state *commandstate.State, plannedUnits []domain.AnalysisUnit) (int, error) {
	batches := r.planAnalysisBatches(plannedUnits)
	parallelism := r.effectiveUnitParallelism(len(batches))
	var (
		completedUnits int
		err            error
	)
	if parallelism <= 1 {
		completedUnits, err = r.analyzePlannedBatchesSerial(analyzeLabel, state, batches)
	} else {
		completedUnits, err = r.analyzePlannedBatchesParallel(analyzeLabel, state, plannedUnits, batches, parallelism)
	}
	if err != nil {
		return completedUnits, err
	}
	logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentAnalyzeUnitsSummary", map[string]interface{}{
		"Completed":   completedUnits,
		"Total":       len(plannedUnits),
		"Batches":     len(batches),
		"Parallelism": parallelism,
	}))
	return completedUnits, nil
}

func (r *learnCurrentProjectRun) effectiveUnitParallelism(unitCount int) int {
	if unitCount <= 1 {
		return 1
	}
	parallelism := r.cont.ConfigRepo.GetCurrentLearningConfig().Parallelism
	if parallelism <= 1 {
		return 1
	}
	if parallelism > unitCount {
		return unitCount
	}
	return parallelism
}

func (r *learnCurrentProjectRun) buildCodebaseRunContext() (*analyzer.CodebaseRunContext, error) {
	return r.cont.AnalyzerSvc.BuildCodebaseRunContext(r.ctx, r.projectRoot, r.currentLanguage, analyzer.AnalyzeCodebaseOptions{
		FocusPaths:       r.effectiveFocusPaths,
		SelectedFiles:    r.selectedFiles,
		SelectedFilesSet: true,
		UseSnapshotDiffs: true,
	})
}

func (r *learnCurrentProjectRun) planAnalysisBatches(plannedUnits []domain.AnalysisUnit) []learnCurrentBatch {
	maxUnits := r.maxUnitsPerBatch()
	if maxUnits < 1 {
		maxUnits = 1
	}
	batches := make([]learnCurrentBatch, 0, (len(plannedUnits)+maxUnits-1)/maxUnits)
	for start := 0; start < len(plannedUnits); start += maxUnits {
		end := start + maxUnits
		if end > len(plannedUnits) {
			end = len(plannedUnits)
		}
		batch := learnCurrentBatch{index: len(batches)}
		for i := start; i < end; i++ {
			batch.units = append(batch.units, indexedAnalysisUnit{index: i, unit: plannedUnits[i]})
		}
		batches = append(batches, batch)
	}
	return batches
}

func (r *learnCurrentProjectRun) maxUnitsPerBatch() int {
	maxUnits := r.cont.ConfigRepo.GetCurrentLearningConfig().MaxUnitsPerCall
	if maxUnits < 1 {
		return 1
	}
	return maxUnits
}

func (r *learnCurrentProjectRun) analyzePlannedBatchesSerial(analyzeLabel string, state *commandstate.State, batches []learnCurrentBatch) (int, error) {
	completedUnits := 0
	for _, batch := range batches {
		results, err := r.analyzeBatch(r.ctx, analyzeLabel, state, batch, true)
		if err != nil {
			return completedUnits, err
		}
		completedUnits += r.mergeUnitResults(results)
		if err := r.saveAnalysisCheckpoint(false); err != nil {
			return completedUnits, err
		}
	}
	return completedUnits, nil
}

func (r *learnCurrentProjectRun) analyzePlannedBatchesParallel(analyzeLabel string, state *commandstate.State, plannedUnits []domain.AnalysisUnit, batches []learnCurrentBatch, parallelism int) (int, error) {
	running := newLearnCurrentRunningUnits(state, plannedUnits)
	if r.opts.showDetailedLogs {
		logger.Info(i18n.GetWithParams("LearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism)))
	}
	r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism))

	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	jobs := make(chan learnCurrentBatch)
	results := make(chan []learnCurrentUnitResult, len(batches))
	errs := make(chan error, len(batches))
	var wg sync.WaitGroup

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				for _, job := range batch.units {
					running.start(job.index, learnCurrentProgressSubject(job.unit))
				}
				r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism))
				batchResults, err := r.analyzeBatch(ctx, analyzeLabel, state, batch, false)
				if err != nil {
					errs <- err
					cancel()
					return
				}
				for _, result := range batchResults {
					running.finish(result.index, result.completed)
				}
				r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeParallelUnits", running.progressParams(parallelism))
				results <- batchResults
			}
		}()
	}

	for _, batch := range batches {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			close(results)
			close(errs)
			return r.collectParallelUnitResults(results, errs)
		case jobs <- batch:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	close(errs)
	return r.collectParallelUnitResults(results, errs)
}

func (r *learnCurrentProjectRun) collectParallelUnitResults(results <-chan []learnCurrentUnitResult, errs <-chan error) (int, error) {
	var firstErr error
	for err := range errs {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	completedUnits := 0
	collected := make([]learnCurrentUnitResult, 0)
	for batchResults := range results {
		for _, result := range batchResults {
			collected = append(collected, result)
			if result.completed {
				completedUnits++
			}
		}
	}
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].index < collected[j].index
	})
	for _, result := range collected {
		r.mergeUnitResult(result)
	}
	if len(collected) > 0 {
		if err := r.saveAnalysisCheckpoint(false); err != nil {
			return completedUnits, err
		}
	}
	return completedUnits, firstErr
}

func (r *learnCurrentProjectRun) unitProgressParams(state *commandstate.State, unit domain.AnalysisUnit, current, total int) map[string]interface{} {
	currentUnit, allUnits := learnCurrentUnitProgress(state, current, total, unit)
	return map[string]interface{}{
		"Current": currentUnit,
		"Total":   allUnits,
		"Name":    learnCurrentProgressSubject(unit),
	}
}

func (r *learnCurrentProjectRun) analyzeBatch(ctx context.Context, analyzeLabel string, state *commandstate.State, batch learnCurrentBatch, showDetails bool) ([]learnCurrentUnitResult, error) {
	var batchUnits []analyzer.AnalyzeCurrentCodebaseBatchUnit
	results := make([]learnCurrentUnitResult, 0, len(batch.units))
	pendingByID := make(map[string]indexedAnalysisUnit, len(batch.units))
	pendingByName := make(map[string]indexedAnalysisUnit, len(batch.units))
	progressLabelByID := make(map[string]string, len(batch.units))
	for _, indexed := range batch.units {
		unitFocusRelPaths := unitFocusPaths(indexed.unit, r.incrementalChanges)
		if len(unitFocusRelPaths) == 0 {
			results = append(results, learnCurrentUnitResult{index: indexed.index, unit: indexed.unit})
			continue
		}
		params := r.batchUnitProgressParams(state, indexed)
		progressLabel := learnCurrentProgressDetail(analyzeLabel, "ProgressLearnCurrentAnalyzeUnit", params)
		if showDetails {
			progressLabel = r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeUnit", params)
		}
		batchUnits = append(batchUnits, analyzer.AnalyzeCurrentCodebaseBatchUnit{
			AnalysisUnit:  indexed.unit,
			FocusAbsPaths: resolveIncrementalFocusPaths(r.projectRoot, unitFocusRelPaths),
		})
		pendingByID[indexed.unit.ID] = indexed
		pendingByName[indexed.unit.Name] = indexed
		progressLabelByID[indexed.unit.ID] = progressLabel
	}
	if len(batchUnits) == 0 {
		return results, nil
	}

	batchLabel := fmt.Sprintf("batch-%03d", batch.index+1)
	analyzeResult, err := r.cont.AnalyzerSvc.AnalyzeCurrentCodebaseBatch(ctx, r.projectRoot, r.projectName, r.currentLanguage, analyzer.AnalyzeCurrentCodebaseBatchOptions{
		RuntimeLabel:  batchLabel,
		LearningMode:  r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
		ChangeProfile: string(r.changeProfile),
		RunContext:    r.codebaseRunContext,
		Units:         batchUnits,
	})
	if err != nil {
		if len(batchUnits) == 1 {
			unitID := batchUnits[0].AnalysisUnit.ID
			if progressLabel := progressLabelByID[unitID]; progressLabel != "" {
				return nil, fmt.Errorf("%s: %w", progressLabel, err)
			}
		}
		return nil, err
	}

	seen := make(map[string]bool, len(analyzeResult.Units))
	for _, unitResult := range analyzeResult.Units {
		indexed, ok := pendingByID[unitResult.AnalysisUnit.ID]
		if !ok && unitResult.AnalysisUnit.Name != "" {
			indexed, ok = pendingByName[unitResult.AnalysisUnit.Name]
		}
		if !ok {
			return nil, fmt.Errorf("batch returned unknown analysis unit %q", unitResult.AnalysisUnit.ID)
		}
		result := buildAnalyzedUnitResult(indexed.unit, indexed.index, unitResult.Patterns, unitResult.ProfileRefreshRecommended)
		results = append(results, result)
		seen[indexed.unit.ID] = true
	}
	for _, indexed := range batch.units {
		unitFocusRelPaths := unitFocusPaths(indexed.unit, r.incrementalChanges)
		if len(unitFocusRelPaths) == 0 {
			continue
		}
		if !seen[indexed.unit.ID] {
			return nil, fmt.Errorf("batch missed analysis unit %q", indexed.unit.ID)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
	return results, nil
}

func (r *learnCurrentProjectRun) batchUnitProgressParams(state *commandstate.State, indexed indexedAnalysisUnit) map[string]interface{} {
	total := indexed.index + 1
	if state != nil && len(state.Units) > 0 {
		total = len(state.Units)
	}
	return r.unitProgressParams(state, indexed.unit, indexed.index+1, total)
}

func buildAnalyzedUnitResult(unit domain.AnalysisUnit, index int, learnedPatterns []domain.Pattern, refreshRecommend agent.ProfileRefreshRecommendation) learnCurrentUnitResult {
	for i := range learnedPatterns {
		if learnedPatterns[i].AnalysisUnitID == "" {
			learnedPatterns[i].AnalysisUnitID = unit.ID
		}
		if learnedPatterns[i].AnalysisUnitName == "" {
			learnedPatterns[i].AnalysisUnitName = unit.Name
		}
	}
	return learnCurrentUnitResult{
		index:            index,
		unit:             unit,
		patterns:         learnedPatterns,
		refreshRecommend: refreshRecommend,
		completed:        true,
	}
}

func (r *learnCurrentProjectRun) mergeUnitResults(results []learnCurrentUnitResult) int {
	completed := 0
	for _, result := range results {
		if result.completed {
			completed++
		}
		r.mergeUnitResult(result)
	}
	return completed
}

func (r *learnCurrentProjectRun) commitUnitSuccess(ctx context.Context, unit domain.AnalysisUnit) error {
	if r.codebaseRunContext != nil && r.codebaseRunContext.SnapshotFlow != nil {
		r.snapshotCommitMu.Lock()
		err := r.codebaseRunContext.SnapshotFlow.CommitScoped(unitFocusPaths(unit, r.incrementalChanges))
		r.snapshotCommitMu.Unlock()
		if err != nil {
			return err
		}
	}
	return commitUnitFileRecords(ctx, r.cont.FileTracker, unitCommittedRecords(unit, r.incrementalChanges))
}

func (r *learnCurrentProjectRun) mergeUnitResult(result learnCurrentUnitResult) {
	if len(result.patterns) > 0 {
		r.patterns = append(r.patterns, result.patterns...)
	}
	if result.completed {
		r.completedAnalysisUnits = append(r.completedAnalysisUnits, result.unit)
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
			saved, err := r.cont.LearnerSvc.CurateAndSavePatternsWithHooks(r.ctx, r.patterns, curator.OperationLearnCurrent, hooks)
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
		analyzeLabel := i18n.Get("ProgressLearnCurrentAnalyzeCodebase")
		for index, unit := range r.completedAnalysisUnits {
			params := r.unitProgressParams(r.analysisState, unit, index+1, len(r.completedAnalysisUnits))
			r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeCommitUnit", params)
			if err := r.commitUnitSuccess(r.ctx, unit); err != nil {
				return err
			}
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

func (r *learnCurrentProjectRun) saveAnalysisCheckpoint(complete bool) error {
	if r.analysisState == nil {
		return nil
	}
	r.analysisState.Analysis = &commandstate.AnalysisCheckpoint{
		Complete:             complete,
		Patterns:             append([]domain.Pattern(nil), r.patterns...),
		CompletedUnits:       append([]domain.AnalysisUnit(nil), r.completedAnalysisUnits...),
		ProfileRefreshNeeded: r.profileRefreshRecommended.Needed,
		ProfileRefreshReason: r.profileRefreshRecommended.Reason,
	}
	return r.stateRepo.Save(r.ctx, r.analysisState)
}

func (r *learnCurrentProjectRun) restoreAnalysisCheckpoint() bool {
	if r.analysisState == nil || r.analysisState.Analysis == nil {
		return false
	}
	checkpoint := r.analysisState.Analysis
	r.patterns = append(r.patterns, checkpoint.Patterns...)
	r.completedAnalysisUnits = append(r.completedAnalysisUnits, checkpoint.CompletedUnits...)
	r.profileRefreshRecommended = agent.ProfileRefreshRecommendation{
		Needed: checkpoint.ProfileRefreshNeeded,
		Reason: checkpoint.ProfileRefreshReason,
	}
	return checkpoint.Complete
}

func (r *learnCurrentProjectRun) saveProfileIfNeeded() error {
	profileStartedAt := time.Now()
	if r.refreshProfile && !r.profileCommitted() && !r.analysisArtifactsCommitted() {
		if err := r.steps.Run(i18n.Get("ProgressLearnCurrentSaveProfile"), func() error {
			profile, err := analyzeProjectProfile(r.ctx, r.cont, r.projectRoot, r.projectName, r.currentLanguage, r.effectiveFocusPaths, r.existingProfile)
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
		if err := r.steps.Run(i18n.Get("ProgressLearnCurrentSkipProfile"), func() error { return nil }); err != nil {
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
