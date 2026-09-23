package learncurrent

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

func (r *learnCurrentProjectRun) normalizeAndSavePatternsStep() error {
	startedAt := time.Now()
	stepLabel := i18n.Get("ProgressLearnCurrentNormalizeAndSavePatterns")
	if err := r.steps.Run(stepLabel, func() error {
		if len(r.pendingKnowledgeReviews()) > 0 {
			return fmt.Errorf("cannot store knowledge before all focuses are reviewed")
		}
		if !r.patternsCommitted() {
			if knowledgeNeedsNormalizeStore(r.patterns, r.retiredPatternIDs) {
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
				r.dropped = append([]patternnorm.Drop(nil), result.Dropped...)
				if result.AISkipped {
					r.observer.noteSkip(skipNormalizeNoRelation)
				}
			} else {
				// 早停：无候选且无退役，跳过规范化 AI/入库，仍继续提交源码基线。
				r.observer.noteSkip(skipNormalizeEmpty)
			}
		}
		if !r.patternsCommitted() && r.analysisState != nil {
			r.analysisState.MarkPatternsCommitted(commandstate.PatternCommitSummary{
				Found:   len(r.patterns),
				Saved:   r.savedCount,
				Retired: r.retiredCount,
			})
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
	if len(r.pendingKnowledgeReviews()) > 0 {
		return fmt.Errorf("cannot complete analysis with unreviewed focuses")
	}
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

func (r *learnCurrentProjectRun) restoreKnowledgeCommitCheckpoint() {
	if r.analysisState == nil {
		return
	}
	r.restoreAnalysisCheckpoint()
	if !r.analysisState.PatternsCommitComplete() {
		return
	}
	summary := r.analysisState.CommittedPatternSummary()
	r.savedCount = summary.Saved
	r.retiredCount = summary.Retired
}

func (r *learnCurrentProjectRun) resultPatternCount() int {
	count := len(r.patterns)
	if r.analysisState == nil || !r.analysisState.PatternsCommitComplete() {
		return count
	}
	summary := r.analysisState.CommittedPatternSummary()
	if summary.Found > count {
		return summary.Found
	}
	return count
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
		if !unit.Reviewed {
			continue
		}
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
	unit.Evidence = unit.Evidence.Clone()
	unit.Patterns = append([]domain.Pattern(nil), unit.Patterns...)
	unit.RetiredPatternIDs = append([]string(nil), unit.RetiredPatternIDs...)
	return unit
}
