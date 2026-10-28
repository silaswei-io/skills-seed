package learncurrent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
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
	parallelism := agent.EffectiveParallelism(r.cont.ConfigRepo.GetAgentConfig().Parallelism)
	if agent.CurrentLearningSupplement(r.ctx) == nil && (parallelism <= 1 || len(batches) <= 1) {
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
	if r.opts.showDetailedLogs {
		logger.InfoAfterProgress(i18n.GetWithParams("LearnCurrentAnalyzeFocusesSummary", map[string]interface{}{
			"Completed":   completedFocuses,
			"Total":       totalFocuses,
			"Batches":     len(batches),
			"Parallelism": parallelism,
		}))
	}
	return completedFocuses, nil
}

func (r *learnCurrentProjectRun) buildCodebaseRunContext() (*analyzer.CodebaseRunContext, error) {
	return r.cont.AnalyzerSvc.BuildCodebaseRunContext(r.ctx, r.projectRoot, r.currentLanguage, analyzer.AnalyzeCodebaseOptions{
		FocusPaths:       r.effectiveFocusPaths,
		SelectedFiles:    r.selectedFiles,
		SelectedFilesSet: true,
	})
}

func (r *learnCurrentProjectRun) planAnalysisBatches(plannedFocuses []domain.EvidenceFocus) []learnCurrentBatch {
	if len(plannedFocuses) == 0 {
		return nil
	}
	batches := make([]learnCurrentBatch, 0, len(plannedFocuses))
	var current []indexedEvidenceFocus
	currentWeight := 0
	flush := func() {
		if len(current) == 0 {
			return
		}
		batches = append(batches, learnCurrentBatch{index: current[0].index, focuses: current})
		current = nil
		currentWeight = 0
	}
	for index, focus := range plannedFocuses {
		indexed := indexedEvidenceFocus{index: index, focus: focus}
		weight := r.focusEvidenceWeight(focus)
		if len(current) > 0 && (!evidenceFocusRelated(current[len(current)-1].focus, focus) || currentWeight+weight > supplementSourceLimit) {
			flush()
		}
		current = append(current, indexed)
		currentWeight += weight
	}
	flush()
	return batches
}

// focusEvidenceWeight 用源码大小估算批次容量；单个超限焦点仍独立提交，由下层证据校验负责挂起。
func (r *learnCurrentProjectRun) focusEvidenceWeight(focus domain.EvidenceFocus) int {
	weight := 0
	for _, path := range focus.EntryPaths {
		info, err := os.Stat(filepath.Join(r.projectRoot, filepath.FromSlash(path)))
		if err == nil {
			weight += int(info.Size())
		}
	}
	if weight == 0 {
		weight = 1
	}
	return weight
}

func evidenceFocusRelated(left, right domain.EvidenceFocus) bool {
	leftPaths := make(map[string]bool, len(left.EntryPaths))
	leftDirs := make(map[string]bool, len(left.EntryPaths))
	for _, path := range left.EntryPaths {
		clean := filepath.Clean(filepath.FromSlash(path))
		leftPaths[clean] = true
		leftDirs[filepath.Dir(clean)] = true
	}
	for _, path := range right.EntryPaths {
		clean := filepath.Clean(filepath.FromSlash(path))
		if leftPaths[clean] || leftDirs[filepath.Dir(clean)] {
			return true
		}
	}
	return false
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
		params := map[string]interface{}{"Name": learnCurrentProgressSubject(indexed.focus)}
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
	// 编排层只通过统一 Mode 入口分析；full/delta 结果形状差异在本函数内消化。
	mode := analyzer.SelectFocusAnalysisMode(r.useDeltaAnalysis())
	focusOpts := analyzer.AnalyzeCurrentFocusBatchOptions{
		Mode:          mode,
		RuntimeLabel:  batchLabel,
		LearningMode:  r.cont.ConfigRepo.GetCurrentLearningConfig().Mode,
		ChangeProfile: string(r.changeProfile),
		RunContext:    r.codebaseRunContext,
	}
	var related map[string][]domain.Pattern
	if mode == analyzer.FocusAnalysisModeDelta {
		var err error
		related, err = r.relatedPatternsByFocus(ctx, batchFocuses)
		if err != nil {
			return nil, err
		}
		deltaFocuses := make([]analyzer.AnalyzeCurrentDeltaFocus, 0, len(batchFocuses))
		for _, focus := range batchFocuses {
			deltaFocuses = append(deltaFocuses, analyzer.AnalyzeCurrentDeltaFocus{
				EvidenceFocus:   focus.EvidenceFocus,
				FocusAbsPaths:   focus.FocusAbsPaths,
				RelatedPatterns: related[focus.EvidenceFocus.ID],
			})
		}
		focusOpts.DeltaFocuses = deltaFocuses
	} else {
		focusOpts.FullFocuses = batchFocuses
	}
	batchResult, err := r.cont.AnalyzerSvc.AnalyzeCurrentFocusBatch(ctx, r.projectRoot, r.projectName, r.currentLanguage, focusOpts)
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
		deltaResults, err := r.buildDeltaFocusResults(batch, batchFocuses, related, batchResult.Delta)
		if err != nil {
			return nil, err
		}
		results = append(results, deltaResults...)
		sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
		return results, nil
	}
	analyzeResult := batchResult.Full
	if analyzeResult == nil {
		analyzeResult = &analyzer.AnalyzeCurrentCodebaseBatchResult{}
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
