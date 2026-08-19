package learn

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/service/repositoryscopeconfig"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

type learnCurrentFocusResult struct {
	index             int
	focus             domain.EvidenceFocus
	patterns          []domain.Pattern
	retiredPatternIDs []string
	refreshRecommend  agent.ProfileRefreshRecommendation
	completed         bool
	reviewed          bool
	conversation      agent.Conversation
}

type learnCurrentBatch struct {
	index   int
	focuses []indexedEvidenceFocus
}

type learnCurrentBatchAnalysisResult struct {
	batchIndex int
	results    []learnCurrentFocusResult
	err        error
}

type indexedEvidenceFocus struct {
	index int
	focus domain.EvidenceFocus
}

type knowledgeReviewTask struct {
	index int
	unit  commandstate.FocusKnowledgeCheckpoint
}

func (r *learnCurrentProjectRun) planLearningAgenda() error {
	planStartedAt := time.Now()
	planLabel := i18n.Get("ProgressLearnCurrentPlanFocuses")
	if r.stateSession != nil {
		developmentFocuses, coverageFocuses := focusKindCounts(r.stateSession.State.Agenda.Focuses)
		planLabel = i18n.GetWithParams("ProgressLearnCurrentPlanFocusesRestored", map[string]interface{}{
			"Focuses":            len(r.stateSession.State.Agenda.Focuses),
			"DevelopmentFocuses": developmentFocuses,
			"CoverageFocuses":    coverageFocuses,
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
			state, err = loadOrCreateCurrentState(r.ctx, r.stateRepo, r.cont.AnalyzerSvc, r.projectName, r.projectRoot, r.currentLanguage, r.learningMode, focusRelPaths, r.incrementalChanges, currentStateInputSummary(r.incrementalChanges, r.selectionPlan, r.selectionSummary), r.changeProfile, r.opts.userContext, r.currentStateInvocationHash())
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
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		if r.patternsCommitted() {
			return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentArtifactsCommittedWithPendingFocuses", map[string]interface{}{"Count": len(r.plannedFocuses)}))
		}
		r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzePreparing", nil)
		runContext, err := r.buildCodebaseRunContext()
		if err != nil {
			return err
		}
		r.codebaseRunContext = runContext
		if err := r.ensureSharedLearningContext(); err != nil {
			return err
		}
		_, err = r.analyzePlannedFocuses(analyzeLabel, r.analysisState, r.plannedFocuses)
		if err != nil {
			return err
		}
		if err := r.reviewRemainingKnowledge(analyzeLabel); err != nil {
			return err
		}
		return r.completeAnalysis()
	}); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
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
	parallelism := r.analysisParallelism(len(batches))
	if parallelism <= 1 || len(batches) <= 1 {
		r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeBatches", map[string]interface{}{
			"Focuses":     len(plannedFocuses),
			"Batches":     len(batches),
			"Parallelism": parallelism,
		})
	}
	completedFocuses, err := r.analyzePlannedBatches(analyzeLabel, state, batches, parallelism)
	if err != nil {
		return completedFocuses, err
	}
	logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentAnalyzeFocusesSummary", map[string]interface{}{
		"Completed":   completedFocuses,
		"Total":       len(plannedFocuses),
		"Batches":     len(batches),
		"Parallelism": parallelism,
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
	batches := make([]learnCurrentBatch, 0, len(plannedFocuses))
	for index, focus := range plannedFocuses {
		batches = append(batches, learnCurrentBatch{
			index:   index,
			focuses: []indexedEvidenceFocus{{index: index, focus: focus}},
		})
	}
	return batches
}

func (r *learnCurrentProjectRun) analysisParallelism(batchCount int) int {
	if batchCount <= 0 {
		return 1
	}
	parallelism := workspacediscovery.EffectiveParallelism(domain.ModeProject, r.cont.ConfigRepo.GetAgentConfig().Parallelism, batchCount)
	if parallelism < 1 {
		return 1
	}
	if parallelism > batchCount {
		return batchCount
	}
	return parallelism
}

func (r *learnCurrentProjectRun) analyzePlannedBatches(analyzeLabel string, state *commandstate.State, batches []learnCurrentBatch, parallelism int) (int, error) {
	if parallelism <= 1 || len(batches) <= 1 {
		return r.analyzePlannedBatchesSerial(analyzeLabel, state, batches)
	}
	return r.analyzePlannedBatchesParallel(analyzeLabel, state, batches, parallelism)
}

func (r *learnCurrentProjectRun) analyzePlannedBatchesSerial(analyzeLabel string, state *commandstate.State, batches []learnCurrentBatch) (int, error) {
	completedFocuses := 0
	for _, batch := range batches {
		results, err := r.analyzeBatch(r.ctx, analyzeLabel, state, batch, true, nil)
		if err != nil {
			return completedFocuses, err
		}
		completed, err := r.checkpointFocusResults(results)
		if err != nil {
			return completedFocuses, err
		}
		completedFocuses += completed
	}
	return completedFocuses, nil
}

func (r *learnCurrentProjectRun) analyzePlannedBatchesParallel(analyzeLabel string, state *commandstate.State, batches []learnCurrentBatch, parallelism int) (int, error) {
	ctx, cancel := context.WithCancelCause(r.ctx)
	defer cancel(nil)

	progress := newLearnCurrentParallelAnalysisProgress(r, analyzeLabel, state, batches, parallelism)
	defer r.steps.ClearDetails()
	defer progress.stopElapsedUpdates()
	ctx = agent.WithAdditionalRetryReporter(ctx, progress.reportRetry)
	progress.update()
	progress.startElapsedUpdates()

	jobs := make(chan learnCurrentBatch)
	results := make(chan learnCurrentBatchAnalysisResult, len(batches))
	var wg sync.WaitGroup
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				progress.start(batch)
				batchResults, err := r.analyzeBatch(ctx, analyzeLabel, state, batch, false, func(candidates int) {
					progress.review(batch, candidates)
				})
				if err != nil {
					progress.stop(batch)
					results <- learnCurrentBatchAnalysisResult{batchIndex: batch.index, err: err}
					cancel(err)
					return
				}
				progress.finish(batch)
				results <- learnCurrentBatchAnalysisResult{batchIndex: batch.index, results: batchResults}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, batch := range batches {
			select {
			case <-ctx.Done():
				return
			case jobs <- batch:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	completedFocuses := 0
	for result := range results {
		if result.err != nil {
			firstErr = preferredBatchError(firstErr, result.err)
			continue
		}
		completed, err := r.checkpointFocusResults(result.results)
		if err != nil {
			firstErr = preferredBatchError(firstErr, err)
			cancel(err)
			continue
		}
		completedFocuses += completed
	}
	if firstErr != nil {
		return completedFocuses, firstErr
	}
	return completedFocuses, nil
}

func preferredBatchError(current, candidate error) error {
	if candidate == nil {
		return current
	}
	if current == nil {
		return candidate
	}
	if errors.Is(current, context.Canceled) && !errors.Is(candidate, context.Canceled) {
		return candidate
	}
	return current
}

type learnCurrentParallelAnalysisProgress struct {
	run         *learnCurrentProjectRun
	baseLabel   string
	state       *commandstate.State
	total       int
	parallelism int
	mu          sync.Mutex
	completed   int
	active      map[int]learnCurrentParallelFocusStatus
	stopElapsed chan struct{}
	elapsedDone chan struct{}
}

type learnCurrentParallelFocusStatus struct {
	base       string
	stage      string
	retryStage string
	startedAt  time.Time
}

func newLearnCurrentParallelAnalysisProgress(run *learnCurrentProjectRun, baseLabel string, state *commandstate.State, batches []learnCurrentBatch, parallelism int) *learnCurrentParallelAnalysisProgress {
	total := 0
	for _, batch := range batches {
		total += len(batch.focuses)
	}
	return &learnCurrentParallelAnalysisProgress{
		run:         run,
		baseLabel:   baseLabel,
		state:       state,
		total:       total,
		parallelism: parallelism,
		active:      make(map[int]learnCurrentParallelFocusStatus),
	}
}

func (p *learnCurrentParallelAnalysisProgress) start(batch learnCurrentBatch) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active[batch.index] = learnCurrentParallelFocusStatus{
		base:      p.run.analysisBatchProgressLabel(p.state, batch, p.total),
		stage:     i18n.Get("LearnCurrentFocusStageSourceEvidence"),
		startedAt: time.Now(),
	}
	p.updateLocked()
}

func (p *learnCurrentParallelAnalysisProgress) finish(batch learnCurrentBatch) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, batch.index)
	p.completed += len(batch.focuses)
	p.updateLocked()
}

func (p *learnCurrentParallelAnalysisProgress) review(batch learnCurrentBatch, candidates int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	status := p.active[batch.index]
	status.stage = i18n.GetWithParams("LearnCurrentFocusStageKnowledgeReview", map[string]interface{}{
		"Candidates": candidates,
	})
	if status.startedAt.IsZero() {
		status.startedAt = time.Now()
	}
	p.active[batch.index] = status
	p.updateLocked()
}

func (p *learnCurrentParallelAnalysisProgress) stop(batch learnCurrentBatch) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, batch.index)
	p.updateLocked()
}

// parallelProgressElapsedRefreshInterval 控制并行焦点耗时的终端刷新频率。
const parallelProgressElapsedRefreshInterval = time.Second

func (p *learnCurrentParallelAnalysisProgress) startElapsedUpdates() {
	if !p.run.opts.showProgress {
		return
	}
	p.mu.Lock()
	if p.stopElapsed != nil {
		p.mu.Unlock()
		return
	}
	p.stopElapsed = make(chan struct{})
	p.elapsedDone = make(chan struct{})
	stop := p.stopElapsed
	done := p.elapsedDone
	p.mu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(parallelProgressElapsedRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				p.mu.Lock()
				if len(p.active) > 0 {
					p.updateLocked()
				}
				p.mu.Unlock()
			}
		}
	}()
}

func (p *learnCurrentParallelAnalysisProgress) stopElapsedUpdates() {
	p.mu.Lock()
	stop := p.stopElapsed
	done := p.elapsedDone
	p.stopElapsed = nil
	p.elapsedDone = nil
	p.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

func (p *learnCurrentParallelAnalysisProgress) update() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.updateLocked()
}

func (p *learnCurrentParallelAnalysisProgress) reportRetry(info agent.RetryInfo) {
	label := agent.OperationLabel(info.Operation)
	if label == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for index, status := range p.active {
		batch := learnCurrentBatch{index: index}
		if p.run.analysisBatchRuntimeLabel(p.state, batch) != label {
			continue
		}
		switch info.Status {
		case agent.RetryProgressStatusWaiting:
			status.retryStage = agent.RetryProgressLabel(status.stage, info)
		case agent.RetryProgressStatusAttempt:
			status.retryStage = agent.RetryAttemptProgressLabel(status.stage, info)
		case agent.RetryProgressStatusRecovered:
			status.retryStage = ""
		}
		p.active[index] = status
		p.updateLocked()
		return
	}
}

func (p *learnCurrentParallelAnalysisProgress) updateLocked() {
	p.run.detailWithLines(p.baseLabel, "ProgressLearnCurrentAnalyzeParallel", map[string]interface{}{
		"Completed":   p.completed,
		"Total":       p.total,
		"Parallelism": p.parallelism,
	}, p.activeLines())
}

func (p *learnCurrentParallelAnalysisProgress) activeLines() []string {
	indices := make([]int, 0, len(p.active))
	for index := range p.active {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	lines := make([]string, 0, len(indices))
	for _, index := range indices {
		status := p.active[index]
		stage := status.stage
		if status.retryStage != "" {
			stage = status.retryStage
		}
		lines = append(lines, i18n.GetWithParams("ProgressLearnCurrentAnalyzeParallelItem", map[string]interface{}{
			"Base":    status.base,
			"Stage":   stage,
			"Elapsed": parallelFocusElapsed(status.startedAt),
		}))
	}
	return lines
}

func parallelFocusElapsed(startedAt time.Time) time.Duration {
	if startedAt.IsZero() {
		return 0
	}
	return time.Since(startedAt).Truncate(time.Second)
}

func (r *learnCurrentProjectRun) focusProgressParams(state *commandstate.State, focus domain.EvidenceFocus, current, total int) map[string]interface{} {
	currentFocus, allFocuses := learnCurrentFocusProgress(state, current, total, focus)
	return map[string]interface{}{
		"Current": currentFocus,
		"Total":   allFocuses,
		"Name":    learnCurrentProgressSubject(focus),
	}
}

func (r *learnCurrentProjectRun) analyzeBatch(ctx context.Context, analyzeLabel string, state *commandstate.State, batch learnCurrentBatch, showDetails bool, onReview func(int)) ([]learnCurrentFocusResult, error) {
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

	batchLabel := r.analysisBatchRuntimeLabel(state, batch)
	var analyzeResult *analyzer.AnalyzeCurrentCodebaseBatchResult
	err := func() error {
		if r.useDeltaAnalysis() {
			deltaResults, err := r.analyzeDeltaBatch(ctx, batch, batchFocuses)
			if err != nil {
				return err
			}
			results = append(results, deltaResults...)
			return nil
		}
		var err error
		analyzeResult, err = r.cont.AnalyzerSvc.AnalyzeCurrentCodebaseBatch(ctx, r.projectRoot, r.projectName, r.currentLanguage, analyzer.AnalyzeCurrentCodebaseBatchOptions{
			RuntimeLabel:      batchLabel,
			LearningMode:      r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
			ChangeProfile:     string(r.changeProfile),
			RunContext:        r.codebaseRunContext,
			SharedContextPath: r.sharedLearningContextPath,
			Focuses:           batchFocuses,
		})
		return err
	}()
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
		return r.reviewAnalyzedFocusResults(ctx, analyzeLabel, state, batch, results, showDetails, onReview)
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
		result := buildAnalyzedFocusResult(indexed.focus, indexed.index, focusResult.Patterns, focusResult.ProfileRefreshRecommended, analyzeResult.Conversation)
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
	return r.reviewAnalyzedFocusResults(ctx, analyzeLabel, state, batch, results, showDetails, onReview)
}

func (r *learnCurrentProjectRun) reviewAnalyzedFocusResults(ctx context.Context, analyzeLabel string, state *commandstate.State, batch learnCurrentBatch, results []learnCurrentFocusResult, showDetails bool, onReview func(int)) ([]learnCurrentFocusResult, error) {
	for index := range results {
		result := &results[index]
		if !result.completed {
			continue
		}
		if onReview != nil {
			onReview(len(result.patterns))
		} else if showDetails {
			total := len(results)
			if state != nil && len(state.Agenda.Focuses) > 0 {
				total = len(state.Agenda.Focuses)
			}
			r.detail(analyzeLabel, "ProgressLearnCurrentAnalyzeReviewFocus", map[string]interface{}{
				"Base":       r.analysisBatchProgressLabel(state, batch, total),
				"Candidates": len(result.patterns),
			})
		}
		patterns, err := r.reviewCurrentFocusResult(ctx, *result, r.analysisBatchRuntimeLabel(state, batch))
		if err != nil {
			return nil, fmt.Errorf("review learned knowledge for %s: %w", learnCurrentProgressSubject(result.focus), err)
		}
		result.patterns = patterns
		result.reviewed = true
		result.conversation = agent.Conversation{}
	}
	return results, nil
}

func (r *learnCurrentProjectRun) reviewCurrentFocusResult(ctx context.Context, result learnCurrentFocusResult, runtimeLabel string) ([]domain.Pattern, error) {
	patterns, err := r.cont.PatternNormSvc.ReviewCurrentKnowledge(ctx, patternnorm.ReviewRequest{
		ProjectName:  r.projectName,
		RootPath:     r.projectRoot,
		Language:     r.currentLanguage,
		RuntimeLabel: runtimeLabel,
		Focus:        result.focus,
		Candidates:   result.patterns,
		UserContext:  r.opts.userContext,
		Conversation: result.conversation,
	})
	if err != nil {
		return nil, err
	}
	focus := domain.DevelopmentFocusFromEvidenceFocus(result.focus)
	for index := range patterns {
		patterns[index].DevelopmentFocus = focus.Clone()
	}
	return patterns, nil
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
			index = minAgendaIndex
		}
	}
	return fmt.Sprintf("batch-%03d", index+1)
}

func (r *learnCurrentProjectRun) analysisBatchProgressLabel(state *commandstate.State, batch learnCurrentBatch, totalFocuses int) string {
	runtimeLabel := r.analysisBatchRuntimeLabel(state, batch)
	if len(batch.focuses) == 0 {
		return runtimeLabel
	}
	first := batch.focuses[0]
	last := batch.focuses[len(batch.focuses)-1]
	currentStart, allFocuses := learnCurrentFocusProgress(state, first.index+1, totalFocuses, first.focus)
	currentEnd, _ := learnCurrentFocusProgress(state, last.index+1, totalFocuses, last.focus)
	subjects := make([]string, 0, len(batch.focuses))
	for _, item := range batch.focuses {
		subjects = append(subjects, shortenRunes(learnCurrentProgressSubject(item.focus), 24))
	}
	if currentStart == currentEnd {
		return i18n.GetWithParams("LearnCurrentParallelActiveSingle", map[string]interface{}{
			"Batch":   runtimeLabel,
			"Current": currentStart,
			"Total":   allFocuses,
			"Name":    strings.Join(subjects, " / "),
		})
	}
	return i18n.GetWithParams("LearnCurrentParallelActiveRange", map[string]interface{}{
		"Batch": runtimeLabel,
		"Start": currentStart,
		"End":   currentEnd,
		"Total": allFocuses,
		"Name":  strings.Join(subjects, " / "),
	})
}

func buildAnalyzedFocusResult(focus domain.EvidenceFocus, index int, learnedPatterns []domain.Pattern, refreshRecommend agent.ProfileRefreshRecommendation, conversation agent.Conversation) learnCurrentFocusResult {
	return learnCurrentFocusResult{
		index:            index,
		focus:            focus,
		patterns:         learnedPatterns,
		refreshRecommend: refreshRecommend,
		completed:        true,
		conversation:     conversation,
	}
}

func appendUniquePatternIDs(current []string, additions ...string) []string {
	seen := make(map[string]bool, len(current)+len(additions))
	out := make([]string, 0, len(current)+len(additions))
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, id := range current {
		add(id)
	}
	for _, id := range additions {
		add(id)
	}
	return out
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

// checkpointFocusResults 将完成的焦点逐项写入可恢复运行状态。
// 调用方必须是结果汇聚者，避免并发写入命令状态。
func (r *learnCurrentProjectRun) checkpointFocusResults(results []learnCurrentFocusResult) (int, error) {
	completed := 0
	for _, result := range results {
		current, err := r.checkpointFocusResult(result)
		if err != nil {
			return completed, err
		}
		completed += current
	}
	return completed, nil
}

func (r *learnCurrentProjectRun) checkpointFocusResult(result learnCurrentFocusResult) (int, error) {
	completed := r.mergeFocusResults([]learnCurrentFocusResult{result})
	if err := r.saveAnalysisCheckpoint(); err != nil {
		return 0, err
	}
	return completed, nil
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
	if result.completed {
		r.setFocusKnowledge(commandstate.FocusKnowledgeCheckpoint{
			Focus:             result.focus,
			Patterns:          append([]domain.Pattern(nil), result.patterns...),
			RetiredPatternIDs: appendUniquePatternIDs(nil, result.retiredPatternIDs...),
			Reviewed:          result.reviewed,
		})
	}
	if result.refreshRecommend.Needed {
		r.profileRefreshRecommended = result.refreshRecommend
	}
	r.syncDerivedKnowledge()
}

func (r *learnCurrentProjectRun) reviewRemainingKnowledge(label string) error {
	pending := r.pendingKnowledgeReviews()
	if len(pending) == 0 || r.patternsCommitted() {
		return nil
	}
	return r.reviewKnowledgeFocuses(label, pending)
}

// reviewLearnedKnowledge 保留给旧检查点恢复和直接服务测试使用。
// 正常学习已在每个焦点完成源码分析后立即审查，不再占用独立顶层步骤。
func (r *learnCurrentProjectRun) reviewLearnedKnowledge() error {
	return r.reviewRemainingKnowledge(i18n.Get("ProgressLearnCurrentReviewKnowledge"))
}

func (r *learnCurrentProjectRun) pendingKnowledgeReviews() []knowledgeReviewTask {
	tasks := make([]knowledgeReviewTask, 0, len(r.focusKnowledge))
	for index, unit := range r.focusKnowledge {
		if unit.Reviewed {
			continue
		}
		tasks = append(tasks, knowledgeReviewTask{index: index, unit: unit})
	}
	return tasks
}

func (r *learnCurrentProjectRun) reviewKnowledgeFocuses(label string, tasks []knowledgeReviewTask) error {
	for completed, task := range tasks {
		r.detail(label, "ProgressLearnCurrentReviewFocus", map[string]interface{}{
			"Completed":   completed,
			"Total":       len(tasks),
			"Current":     task.index + 1,
			"AgendaTotal": len(r.focusKnowledge),
			"Name":        learnCurrentProgressSubject(task.unit.Focus),
			"Candidates":  len(task.unit.Patterns),
		})
		patterns, err := r.reviewKnowledgeFocus(r.ctx, task)
		if err != nil {
			return fmt.Errorf("review learned knowledge for %s: %w", learnCurrentProgressSubject(task.unit.Focus), err)
		}
		r.applyKnowledgeReviewResult(task, patterns)
		if err := r.saveAnalysisCheckpoint(); err != nil {
			return err
		}
	}
	r.detail(label, "ProgressLearnCurrentReviewComplete", map[string]interface{}{
		"Completed": len(tasks),
		"Total":     len(tasks),
	})
	return nil
}

func (r *learnCurrentProjectRun) reviewKnowledgeFocus(ctx context.Context, task knowledgeReviewTask) ([]domain.Pattern, error) {
	runtimeLabel := r.analysisBatchRuntimeLabel(r.analysisState, learnCurrentBatch{
		index:   task.index,
		focuses: []indexedEvidenceFocus{{index: task.index, focus: task.unit.Focus}},
	})
	patterns, err := r.cont.PatternNormSvc.ReviewCurrentKnowledge(ctx, patternnorm.ReviewRequest{
		ProjectName:  r.projectName,
		RootPath:     r.projectRoot,
		Language:     r.currentLanguage,
		RuntimeLabel: runtimeLabel,
		Focus:        task.unit.Focus,
		Candidates:   task.unit.Patterns,
		UserContext:  r.opts.userContext,
	})
	return patterns, err
}

func (r *learnCurrentProjectRun) applyKnowledgeReviewResult(task knowledgeReviewTask, patterns []domain.Pattern) {
	unit := task.unit
	focus := domain.DevelopmentFocusFromEvidenceFocus(unit.Focus)
	for index := range patterns {
		patterns[index].DevelopmentFocus = focus.Clone()
	}
	unit.Patterns = append([]domain.Pattern(nil), patterns...)
	unit.Reviewed = true
	r.setFocusKnowledge(unit)
	r.syncDerivedKnowledge()
}

func (r *learnCurrentProjectRun) normalizeAndSavePatternsStep() error {
	startedAt := time.Now()
	stepLabel := i18n.Get("ProgressLearnCurrentNormalizeAndSavePatterns")
	if err := r.steps.Run(stepLabel, func() error {
		if !r.patternsCommitted() && (len(r.patterns) > 0 || len(r.retiredPatternIDs) > 0) {
			hooks := patternnorm.ProgressHooks{
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
			result, err := r.cont.PatternNormSvc.NormalizeAndStoreWithHooks(r.ctx, patternnorm.NormalizeRequest{
				Operation:          patternnorm.OperationLearnCurrent,
				ProjectName:        r.projectName,
				RootPath:           r.projectRoot,
				Language:           r.currentLanguage,
				Candidates:         r.patterns,
				RetiredPatternIDs:  r.retiredPatternIDs,
				DecisionCheckpoint: newCurrentDecisionCheckpoint(r.stateRepo, r.analysisState),
				UserContext:        r.opts.userContext,
			}, hooks)
			if err != nil {
				return err
			}
			r.savedCount = len(result.Written)
			r.retiredCount = len(result.RetiredPatternIDs)
		}
		if !r.patternsCommitted() && r.analysisState != nil {
			r.analysisState.MarkPatternsCommitted()
			if err := r.stateRepo.Save(r.ctx, r.analysisState); err != nil {
				return err
			}
		}
		if !r.sourceBaselineCommitted() {
			r.detail(stepLabel, "ProgressLearnCurrentCommitFiles", map[string]interface{}{
				"Count": len(r.incrementalChanges.Records) + len(r.incrementalChanges.Deleted),
			})
			if err := r.commitCurrentAnalysis(r.ctx); err != nil {
				return err
			}
			if r.analysisState != nil {
				r.analysisState.MarkSourceBaselineCommitted()
				if err := r.stateRepo.Save(r.ctx, r.analysisState); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "command.learn_current.normalize_and_save_patterns",
		"duration", time.Since(startedAt),
		"patterns_count", len(r.patterns),
		"saved_count", r.savedCount,
		"retired_count", r.retiredCount,
	)
	if r.opts.showDetailedLogs && (len(r.patterns) > 0 || len(r.retiredPatternIDs) > 0) {
		logger.Info(i18n.GetWithParams("LearnCurrentPatternsSaved", map[string]interface{}{
			"Saved":   r.savedCount,
			"Retired": r.retiredCount,
		}))
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
	completed := r.completedEvidenceFocuses()
	if r.analysisState != nil {
		for _, focus := range r.analysisState.Agenda.Focuses {
			if len(evidenceFocusPaths(focus, r.incrementalChanges)) == 0 || evidenceFocusIncluded(completed, focus) {
				continue
			}
			missing = append(missing, learnCurrentProgressSubject(focus))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s", i18n.GetWithParams("LearnCurrentAgendaIncomplete", map[string]interface{}{"Focuses": strings.Join(missing, ", ")}))
	}
	uncovered := uncoveredAnalysisPaths(completed, analysisCandidatePaths(r.incrementalChanges))
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

func (r *learnCurrentProjectRun) patternsCommitted() bool {
	return r.analysisState != nil && r.analysisState.PatternsCommitComplete()
}

func (r *learnCurrentProjectRun) sourceBaselineCommitted() bool {
	return r.analysisState != nil && r.analysisState.SourceBaselineCommitComplete()
}

func (r *learnCurrentProjectRun) projectionsCommitted() bool {
	return r.analysisState != nil && r.analysisState.ProjectionsCommitComplete()
}

func (r *learnCurrentProjectRun) saveAnalysisCheckpoint() error {
	if r.analysisState == nil {
		return nil
	}
	r.analysisState.Analysis = &commandstate.AnalysisCheckpoint{
		FocusKnowledge:       cloneFocusKnowledge(r.focusKnowledge),
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
	r.focusKnowledge = cloneFocusKnowledge(checkpoint.FocusKnowledge)
	r.syncDerivedKnowledge()
	r.profileRefreshRecommended = agent.ProfileRefreshRecommendation{
		Needed: checkpoint.ProfileRefreshNeeded,
		Reason: checkpoint.ProfileRefreshReason,
	}
}

func (r *learnCurrentProjectRun) setFocusKnowledge(unit commandstate.FocusKnowledgeCheckpoint) {
	for index, current := range r.focusKnowledge {
		if evidenceFocusSame(current.Focus, unit.Focus) {
			r.focusKnowledge[index] = cloneFocusKnowledgeUnit(unit)
			return
		}
	}
	r.focusKnowledge = append(r.focusKnowledge, cloneFocusKnowledgeUnit(unit))
	r.orderFocusKnowledge()
}

func (r *learnCurrentProjectRun) orderFocusKnowledge() {
	if r.analysisState == nil || len(r.analysisState.Agenda.Focuses) == 0 {
		return
	}
	indexOf := func(target domain.EvidenceFocus) int {
		for index, focus := range r.analysisState.Agenda.Focuses {
			if evidenceFocusSame(focus, target) {
				return index
			}
		}
		return len(r.analysisState.Agenda.Focuses)
	}
	sort.SliceStable(r.focusKnowledge, func(i, j int) bool {
		return indexOf(r.focusKnowledge[i].Focus) < indexOf(r.focusKnowledge[j].Focus)
	})
}

func (r *learnCurrentProjectRun) syncDerivedKnowledge() {
	r.orderFocusKnowledge()
	r.patterns = r.patterns[:0]
	r.retiredPatternIDs = r.retiredPatternIDs[:0]
	for _, unit := range r.focusKnowledge {
		r.patterns = append(r.patterns, unit.Patterns...)
		r.retiredPatternIDs = appendUniquePatternIDs(r.retiredPatternIDs, unit.RetiredPatternIDs...)
	}
}

func (r *learnCurrentProjectRun) completedEvidenceFocuses() []domain.EvidenceFocus {
	focuses := make([]domain.EvidenceFocus, 0, len(r.focusKnowledge))
	for _, unit := range r.focusKnowledge {
		focuses = append(focuses, unit.Focus)
	}
	return focuses
}

func cloneFocusKnowledge(units []commandstate.FocusKnowledgeCheckpoint) []commandstate.FocusKnowledgeCheckpoint {
	out := make([]commandstate.FocusKnowledgeCheckpoint, 0, len(units))
	for _, unit := range units {
		out = append(out, cloneFocusKnowledgeUnit(unit))
	}
	return out
}

func cloneFocusKnowledgeUnit(unit commandstate.FocusKnowledgeCheckpoint) commandstate.FocusKnowledgeCheckpoint {
	unit.Patterns = append([]domain.Pattern(nil), unit.Patterns...)
	unit.RetiredPatternIDs = append([]string(nil), unit.RetiredPatternIDs...)
	return unit
}

func (r *learnCurrentProjectRun) saveProfileIfNeeded() error {
	profileStartedAt := time.Now()
	var profile *domain.ProjectProfile
	if r.projectionsCommitted() {
		return nil
	}
	if r.refreshProfile {
		label := i18n.Get("ProgressLearnCurrentSaveProfile")
		if err := r.steps.Run(label, func() error {
			var err error
			profile, err = r.refreshProjectProfile()
			if err != nil {
				return err
			}
			profile, err = r.verifyProjectProfile(r.ctx, profile)
			if err != nil {
				return err
			}
			if err := r.cont.ProfileRepo.Save(r.ctx, profile); err != nil {
				return err
			}
			return r.markProjectionsCommitted()
		}); err != nil {
			logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		label := i18n.Get("ProgressLearnCurrentSkipProfile")
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
		profile = r.existingProfile
		if profile == nil {
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
		if err := r.markProjectionsCommitted(); err != nil {
			return err
		}
	}
	return nil
}

func (r *learnCurrentProjectRun) markProjectionsCommitted() error {
	if r.analysisState == nil {
		return nil
	}
	r.analysisState.MarkProjectionsCommitted()
	return r.stateRepo.Save(r.ctx, r.analysisState)
}

func (r *learnCurrentProjectRun) verifyProjectProfile(ctx context.Context, profile *domain.ProjectProfile) (*domain.ProjectProfile, error) {
	if profile == nil || r.cont == nil || r.cont.PatternReader == nil {
		return profile, nil
	}
	patterns, err := r.cont.PatternReader.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	structuralConfig := config.StructuralConfig{Provider: config.StructuralProviderAuto}
	if r.cont.ConfigRepo != nil {
		structuralConfig = r.cont.ConfigRepo.GetCurrentLearningConfig().Structural
	}
	scope := repositoryscopeconfig.DefaultKnowledgeScope()
	if r.cont.ConfigRepo != nil {
		scope = repositoryscopeconfig.KnowledgeScope(r.cont.ConfigRepo, r.projectRoot)
	}
	verified, _, err := knowledge.VerifyProjectKnowledge(ctx, profile, domain.ActivePatterns(patterns), r.projectRoot, sourcecode.NewResolver(structuralConfig), scope)
	return verified, err
}

func (r *learnCurrentProjectRun) refreshProjectProfile() (*domain.ProjectProfile, error) {
	options := analyzer.AnalyzeProjectOptions{}
	if r.existingProfile != nil && len(r.effectiveFocusPaths) > 0 {
		options.ExistingProfile = r.existingProfile
		options.FocusPaths = r.effectiveFocusPaths
	}
	options.OnStage = func(label string) {
		r.patternStageDetail(i18n.Get("ProgressLearnCurrentSaveProfile"), label)
	}
	return r.cont.AnalyzerSvc.RefreshProjectProfile(r.ctx, r.projectRoot, r.projectName, r.currentLanguage, options)
}
