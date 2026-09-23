package learncurrent

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
)

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
		r.setFocusKnowledge(result.checkpoint())
		if result.conversation.Valid() {
			if r.conversations == nil {
				r.conversations = make(map[string]agent.Conversation)
			}
			r.conversations[result.focus.ID] = result.conversation
		}
	}
	if result.refreshRecommend.Needed {
		r.profileRefreshRecommended = result.refreshRecommend
	}
	r.syncDerivedKnowledge()
}

func (r *learnCurrentProjectRun) focusConversation(focus domain.EvidenceFocus) agent.Conversation {
	if r.conversations == nil {
		return agent.Conversation{}
	}
	return r.conversations[focus.ID]
}

func (r *learnCurrentProjectRun) reviewRemainingKnowledge(label string) error {
	pending := r.pendingKnowledgeReviews()
	if len(pending) == 0 || r.patternsCommitted() {
		return nil
	}
	_, err := r.analyzePlannedBatches(label, r.analysisState, nil, 1)
	return err
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
		Evidence:     task.unit.Evidence.Clone(),
		UserContext:  r.opts.userContext,
		Conversation: r.focusConversation(task.unit.Focus),
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

// analyzedFocusRouting 是一批分析结果的审查路由结果。
type analyzedFocusRouting struct {
	ai             []knowledgeReviewTask
	localCompleted int
}

// routeAnalyzedFocuses 按策略分流：空候选 / standard 本地硬闸 / AI 审查。
// progress 用于把本地收口计入并行进度完成数，避免 UI 卡在 0/N。
func (r *learnCurrentProjectRun) routeAnalyzedFocuses(results []learnCurrentFocusResult, progress *learnCurrentParallelAnalysisProgress) analyzedFocusRouting {
	out := analyzedFocusRouting{}
	for _, analyzed := range results {
		if !analyzed.completed {
			continue
		}
		switch routeFocusReview(analyzed.focus, analyzed.patterns) {
		case reviewRouteNone:
			r.applyLocalEmptyReview(analyzed)
			r.observer.noteSkip(skipReviewEmpty)
			out.localCompleted++
			if progress != nil {
				progress.finish(localReviewProgressBatch(analyzed))
			}
		case reviewRouteLocalStandard:
			r.applyLocalStandardReview(analyzed)
			r.observer.noteSkip(skipReviewLocalStandard)
			out.localCompleted++
			if progress != nil {
				progress.finish(localReviewProgressBatch(analyzed))
			}
		default:
			out.ai = append(out.ai, knowledgeReviewTask{index: analyzed.index, unit: analyzed.checkpoint()})
		}
	}
	return out
}

// localReviewProgressBatch 构造仅用于进度完成计数的虚拟批次，不进入 active 集合。
func localReviewProgressBatch(result learnCurrentFocusResult) learnCurrentBatch {
	return learnCurrentBatch{
		index:   -(result.index + 1),
		focuses: []indexedEvidenceFocus{{index: result.index, focus: result.focus}},
	}
}

// applyLocalEmptyReview 对无候选焦点完成本地审查收口。
func (r *learnCurrentProjectRun) applyLocalEmptyReview(result learnCurrentFocusResult) {
	unit := result.checkpoint()
	unit.Patterns = nil
	unit.Reviewed = true
	r.setFocusKnowledge(unit)
	r.syncDerivedKnowledge()
}

// applyLocalStandardReview 对 standard 且通过硬闸的候选本地接受（S3）。
// 保留分析产出的表述与证据，不调用 Agent；careful/critical 与 operational_risk 不会进入此路径。
func (r *learnCurrentProjectRun) applyLocalStandardReview(result learnCurrentFocusResult) {
	unit := result.checkpoint()
	focus := domain.DevelopmentFocusFromEvidenceFocus(unit.Focus)
	patterns := append([]domain.Pattern(nil), unit.Patterns...)
	for index := range patterns {
		patterns[index].DevelopmentFocus = focus.Clone()
		patterns[index].KnowledgeFlags = domain.CanonicalKnowledgeFlags(patterns[index].KnowledgeFlags)
	}
	unit.Patterns = patterns
	unit.Reviewed = true
	r.setFocusKnowledge(unit)
	r.syncDerivedKnowledge()
}
