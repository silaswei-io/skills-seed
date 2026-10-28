package learncurrent

import (
	"errors"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
)

type learningTask struct {
	batch        learnCurrentBatch
	review       knowledgeReviewTask
	conversation agent.Conversation
	isReview     bool
}

type learningOutcome struct {
	task     learningTask
	analyzed []learnCurrentFocusResult
	reviewed []domain.Pattern
	err      error
}

// analyzePlannedBatches 将已保存候选的审查与其他批次分析流水执行；检查点由当前协程串行提交。
func (r *learnCurrentProjectRun) analyzePlannedBatches(label string, state *commandstate.State, batches []learnCurrentBatch, parallelism int) (int, error) {
	limit := max(1, parallelism)
	reviews := r.pendingKnowledgeReviews()
	results := make(chan learningOutcome, limit)
	next, active, completed := 0, 0, 0
	var firstErr error
	for active > 0 || next < len(batches) || len(reviews) > 0 {
		for firstErr == nil && r.ctx.Err() == nil && active < limit {
			if len(reviews) == 0 && next == len(batches) {
				break
			}
			var task learningTask
			if len(reviews) > 0 {
				task.isReview = true
				task.review = reviews[0]
				reviews = reviews[1:]
				task.conversation = r.focusConversation(task.review.unit.Focus)
				r.detail(label, "ProgressLearnCurrentReviewFocus", map[string]interface{}{
					"Name": learnCurrentProgressSubject(task.review.unit.Focus), "Candidates": len(task.review.unit.Patterns),
				})
			} else {
				task.batch = batches[next]
				next++
			}
			active++
			go func() {
				outcome := learningOutcome{task: task}
				if task.isReview {
					outcome.reviewed, outcome.err = r.reviewKnowledgeFocus(r.ctx, task.review, task.conversation)
					if outcome.err != nil {
						outcome.err = fmt.Errorf("review learned knowledge for %s: %w", learnCurrentProgressSubject(task.review.unit.Focus), outcome.err)
					}
				} else {
					outcome.analyzed, outcome.err = r.analyzeBatch(r.ctx, label, state, task.batch, limit == 1)
				}
				results <- outcome
			}()
		}
		if active == 0 {
			break
		}
		result := <-results
		active--
		if result.err != nil {
			firstErr = errors.Join(firstErr, result.err)
			continue
		}
		if result.task.isReview {
			r.applyKnowledgeReviewResult(result.task.review, result.reviewed)
			if err := r.saveAnalysisCheckpoint(); err != nil {
				firstErr = errors.Join(firstErr, err)
			} else {
				completed++
			}
			continue
		}
		if _, err := r.checkpointFocusResults(result.analyzed); err != nil {
			firstErr = errors.Join(firstErr, err)
			continue
		}
		for _, analyzed := range result.analyzed {
			if analyzed.completed {
				reviews = append(reviews, knowledgeReviewTask{index: analyzed.index, unit: analyzed.checkpoint()})
			}
		}
	}
	return completed, errors.Join(firstErr, r.ctx.Err())
}
