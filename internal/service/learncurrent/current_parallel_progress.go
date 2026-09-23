package learncurrent

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
)

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
	runtimeLabel string
	base         string
	stage        string
	retryStage   string
	startedAt    time.Time
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
		runtimeLabel: p.run.analysisBatchRuntimeLabel(p.state, batch),
		base:         p.run.analysisBatchProgressLabel(p.state, batch, p.total),
		stage:        i18n.Get("LearnCurrentFocusStageSourceEvidence"),
		startedAt:    time.Now(),
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
	if p.parallelism == 1 && len(batch.focuses) == 1 {
		focus := batch.focuses[0]
		current, total := learnCurrentFocusProgress(p.state, focus.index+1, p.total, focus.focus)
		p.run.detail(p.baseLabel, "ProgressLearnCurrentReviewFocus", map[string]interface{}{
			"Completed": p.completed, "Total": p.total, "Current": current, "AgendaTotal": total,
			"Name": learnCurrentProgressSubject(focus.focus), "Candidates": candidates,
		})
		return
	}
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
	if !p.run.opts.showProgress || p.parallelism <= 1 {
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
	// 串行进度由既有 step binder 维护，避免覆盖当前焦点和重试原因。
	if p.parallelism == 1 {
		return
	}
	label := agent.OperationLabel(info.Operation)
	if label == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for index, status := range p.active {
		if status.runtimeLabel != label {
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
