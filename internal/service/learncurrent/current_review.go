package learncurrent

import (
	"context"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
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
	parallelism := 1
	if r.cont != nil && r.cont.ConfigRepo != nil {
		parallelism = agent.EffectiveParallelism(r.cont.ConfigRepo.GetAgentConfig().Parallelism)
	}
	_, err := r.analyzePlannedBatches(label, r.analysisState, nil, parallelism)
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

func (r *learnCurrentProjectRun) reviewKnowledgeFocus(ctx context.Context, task knowledgeReviewTask, conversation agent.Conversation) ([]domain.Pattern, error) {
	runtimeLabel := r.analysisBatchRuntimeLabel(r.analysisState, learnCurrentBatch{
		index:   task.index,
		focuses: []indexedEvidenceFocus{{index: task.index, focus: task.unit.Focus}},
	})
	candidates := task.unit.Patterns
	if agent.CurrentLearningSupplement(ctx) != nil {
		candidates = nil
		for _, pattern := range task.unit.Patterns {
			if pattern.IsSemanticKnowledge() {
				candidates = append(candidates, pattern)
			}
		}
	}
	patterns, err := r.cont.PatternNormSvc.ReviewCurrentKnowledge(ctx, patternnorm.ReviewRequest{
		ProjectName:  r.projectName,
		RootPath:     r.projectRoot,
		Language:     r.currentLanguage,
		RuntimeLabel: runtimeLabel,
		Focus:        task.unit.Focus,
		Candidates:   candidates,
		Evidence:     task.unit.Evidence.Clone(),
		UserContext:  r.opts.userContext,
		Conversation: conversation,
	})
	return patterns, err
}

func (r *learnCurrentProjectRun) applyKnowledgeReviewResult(task knowledgeReviewTask, patterns []domain.Pattern) {
	unit := task.unit
	if input := agent.CurrentLearningSupplement(r.ctx); input != nil {
		verified := make([]domain.Pattern, 0, len(patterns))
		for _, pattern := range patterns {
			pattern.BusinessMethod = verifiedSupplementMethod(pattern.BusinessMethod, input)
			valid := pattern.IsSemanticKnowledge() && len(pattern.EvidenceLocations) > 0
			for i, evidence := range pattern.EvidenceLocations {
				hash := input.SourceHashes[evidence.Path]
				if hash == "" {
					valid = false
					break
				}
				if evidence.Line > 0 {
					covered := false
					for _, sourceRange := range input.SourceRanges[evidence.Path] {
						covered = covered || sourceRange.Start <= evidence.Line && evidence.Line <= sourceRange.End
					}
					if !covered {
						valid = false
						break
					}
				}
				pattern.EvidenceLocations[i].ContentHash = hash
			}
			if valid {
				verified = append(verified, pattern)
			}
		}
		patterns = verified
	}
	unit.Patterns = append([]domain.Pattern(nil), patterns...)
	unit.Reviewed = true
	r.setFocusKnowledge(unit)
	r.syncDerivedKnowledge()
}

// verifiedSupplementMethod 用本轮 CodeGraph 符号事实覆盖 AI 声明的入口签名。
func verifiedSupplementMethod(method *domain.BusinessMethod, input *agent.LearningSupplement) *domain.BusinessMethod {
	if method == nil || input == nil {
		return nil
	}
	location := method.DisplayLocation()
	path := pathx.CleanEvidenceLocationPath(location)
	separator := strings.LastIndex(location, ":")
	if path == "" || separator < 0 {
		return nil
	}
	line, err := strconv.Atoi(location[separator+1:])
	if err != nil || line < 1 {
		return nil
	}
	for _, symbol := range input.Symbols[path] {
		if symbol.Line == line && symbol.Name == method.Name {
			verified := *method
			verified.Function = symbol.Signature
			return &verified
		}
	}
	return nil
}
