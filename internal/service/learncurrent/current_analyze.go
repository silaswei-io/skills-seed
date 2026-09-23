package learncurrent

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
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

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
			// 仅补审查时也要准备待提交快照，避免文件指纹前进而 diff 基线滞后。
			if !r.sourceBaselineCommitted() && r.incrementalChanges != nil {
				var err error
				r.codebaseRunContext, err = r.buildCodebaseRunContext()
				if err != nil {
					return err
				}
			}
			if err := r.reviewRemainingKnowledge(analyzeLabel); err != nil {
				return err
			}
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
	totalFocuses := len(plannedFocuses) + len(r.pendingKnowledgeReviews())
	parallelism := r.analysisParallelism(totalFocuses)
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
		"Total":       totalFocuses,
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

	batchLabel := r.analysisBatchRuntimeLabel(state, batch)
	// 焦点分析只保留一种编排语义：按 Mode 选择材料路径，避免调用点散落布尔分支。
	mode := analyzer.SelectFocusAnalysisMode(r.useDeltaAnalysis())
	var analyzeResult *analyzer.AnalyzeCurrentCodebaseBatchResult
	err := func() error {
		if mode == analyzer.FocusAnalysisModeDelta {
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
	if mode == analyzer.FocusAnalysisModeDelta {
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
		result := buildAnalyzedFocusResult(indexed.focus, indexed.index, focusResult.Patterns, focusResult.ProfileRefreshRecommended, analyzeResult.Conversation)
		result.evidence = focusResult.Evidence
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
