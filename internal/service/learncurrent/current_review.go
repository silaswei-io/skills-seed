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
