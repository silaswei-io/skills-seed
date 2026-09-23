package learncurrent

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
)

// focusTask 区分源码分析和独立审查；两种任务共享调用额度。
type focusTask struct {
	batch  learnCurrentBatch
	review *knowledgeReviewTask
}

type focusTaskResult struct {
	task     focusTask
	analyzed []learnCurrentFocusResult
	reviewed []domain.Pattern
	err      error
}

// analyzePlannedBatches 由协调器独占检查点写入，工作协程只执行 Agent 调用。
// 分析成功必须先落盘才能排入审查。审查串行，与其他焦点分析流水执行。
func (r *learnCurrentProjectRun) analyzePlannedBatches(label string, state *commandstate.State, batches []learnCurrentBatch, parallelism int) (int, error) {
	parallelism = max(1, parallelism)
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	reviews := r.pendingKnowledgeReviews()
	allBatches := append([]learnCurrentBatch(nil), batches...)
	for _, review := range reviews {
		allBatches = append(allBatches, reviewBatch(review, len(allBatches)))
	}
	progress := newLearnCurrentParallelAnalysisProgress(r, label, state, allBatches, parallelism)
	defer r.steps.ClearDetails()
	defer progress.stopElapsedUpdates()
	ctx = agent.WithAdditionalRetryReporter(ctx, progress.reportRetry)
	progress.update()
	progress.startElapsedUpdates()

	results := make(chan focusTaskResult, parallelism)
	active, next, completed := 0, 0, 0
	reviewing := false
	var firstErr error
	for {
		if err := ctx.Err(); err != nil {
			firstErr = preferredBatchError(firstErr, err)
		}
		for firstErr == nil && active < parallelism {
			var task focusTask
			if !reviewing && len(reviews) > 0 {
				review := reviews[0]
				reviews = reviews[1:]
				task = focusTask{batch: reviewBatch(review, len(batches)+review.index), review: &review}
				reviewing = true
			} else if next < len(batches) {
				task.batch = batches[next]
				next++
			} else {
				break
			}
			active++
			progress.start(task.batch)
			if task.review != nil {
				progress.review(task.batch, len(task.review.unit.Patterns))
			}
			go func() {
				result := focusTaskResult{task: task}
				if task.review != nil {
					result.reviewed, result.err = r.reviewKnowledgeFocus(ctx, *task.review)
					if result.err != nil {
						result.err = fmt.Errorf("review learned knowledge for %s: %w", learnCurrentProgressSubject(task.review.unit.Focus), result.err)
					}
				} else {
					result.analyzed, result.err = r.analyzeBatch(ctx, label, state, task.batch, parallelism == 1)
				}
				results <- result
			}()
		}
		if active == 0 {
			return completed, firstErr
		}
		result := <-results
		active--
		progress.stop(result.task.batch)
		if result.task.review != nil {
			reviewing = false
		}
		if result.err != nil {
			// 停止派发新任务，收齐在途结果，保留已经支付的分析成本。
			firstErr = preferredBatchError(firstErr, result.err)
			continue
		}
		var err error
		if result.task.review != nil {
			r.applyKnowledgeReviewResult(*result.task.review, result.reviewed)
			err = r.saveAnalysisCheckpoint()
			if err == nil {
				completed++
				progress.finish(result.task.batch)
			}
		} else {
			_, err = r.checkpointFocusResults(result.analyzed)
			if err == nil {
				for _, analyzed := range result.analyzed {
					if analyzed.completed {
						reviews = append(reviews, knowledgeReviewTask{index: analyzed.index, unit: analyzed.checkpoint()})
					}
				}
			}
		}
		if err != nil {
			firstErr = preferredBatchError(firstErr, err)
			cancel()
		}
	}
}

func reviewBatch(task knowledgeReviewTask, index int) learnCurrentBatch {
	return learnCurrentBatch{index: index, focuses: []indexedEvidenceFocus{{index: task.index, focus: task.unit.Focus}}}
}
