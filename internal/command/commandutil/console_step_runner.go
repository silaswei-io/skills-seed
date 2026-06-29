package commandutil

import (
	"context"
	"sync"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
)

// ConsoleStepRunnerOptions 定义命令行步骤进度渲染参数。
type ConsoleStepRunnerOptions struct {
	// TotalSteps 是命令顶层阶段总数，用于进度条分母。
	TotalSteps int
	// ShowProgress 控制是否渲染动态进度条；关闭时仍会触发回调并顺序打印细节。
	ShowProgress bool
	// OnStepStart 在顶层阶段开始时触发。
	OnStepStart func(label string)
	// OnStepComplete 在顶层阶段成功完成时触发。
	OnStepComplete func(label string)
	// OnStepUpdate 在阶段细节或重试状态刷新时触发。
	OnStepUpdate func(label string)
}

// ConsoleStepRunner 统一封装命令行步骤、细节标签、失败展示和 Agent 重试进度。
type ConsoleStepRunner struct {
	tracker       *progress.Tracker
	retryProgress *agent.RetryProgressBinder
	showProgress  bool

	mu             sync.Mutex
	currentLabels  map[string]string
	onStepStart    func(label string)
	onStepComplete func(label string)
	onStepUpdate   func(label string)
}

// NewConsoleStepRunner 创建可复用的命令步骤进度 runner。
func NewConsoleStepRunner(opts ConsoleStepRunnerOptions) *ConsoleStepRunner {
	runner := &ConsoleStepRunner{
		tracker:        progress.New(opts.TotalSteps),
		showProgress:   opts.ShowProgress,
		currentLabels:  make(map[string]string),
		onStepStart:    opts.OnStepStart,
		onStepComplete: opts.OnStepComplete,
		onStepUpdate:   opts.OnStepUpdate,
	}
	runner.retryProgress = agent.NewRetryProgressBinder(runner.update)
	return runner
}

// WithContext 把 Agent 重试进度回调绑定到 context。
func (r *ConsoleStepRunner) WithContext(ctx context.Context) context.Context {
	if r == nil || r.retryProgress == nil {
		return ctx
	}
	return r.retryProgress.WithContext(ctx)
}

// Run 执行一个顶层步骤。失败时会保留最近一次 Detail 设置的标签。
func (r *ConsoleStepRunner) Run(label string, fn func() error) error {
	r.remember(label, label)
	r.retryProgress.StartStep(label)
	if r.onStepStart != nil {
		r.onStepStart(label)
	}

	var err error
	if r.showProgress {
		r.tracker.StartStep(label)
		err = fn()
	} else {
		err = fn()
	}
	if err != nil {
		failureLabel := r.DisplayLabel(label)
		if r.showProgress {
			r.tracker.FailStep(failureLabel)
		}
		r.retryProgress.FinishStep(failureLabel, false)
		return err
	}
	if r.showProgress {
		r.tracker.CompleteStep(label)
	}
	r.retryProgress.FinishStep(r.DisplayLabel(label), true)
	if r.onStepComplete != nil {
		r.onStepComplete(label)
	}
	return nil
}

// Detail 刷新顶层步骤内的详细动作标签，并返回该标签用于错误上下文。
func (r *ConsoleStepRunner) Detail(baseLabel, detailLabel string) string {
	r.remember(baseLabel, detailLabel)
	r.retryProgress.StartStep(detailLabel)
	r.update(detailLabel)
	return detailLabel
}

// DisplayLabel 返回指定顶层步骤当前应展示的标签。
func (r *ConsoleStepRunner) DisplayLabel(baseLabel string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if label := r.currentLabels[baseLabel]; label != "" {
		return label
	}
	return baseLabel
}

func (r *ConsoleStepRunner) update(label string) {
	if r.onStepUpdate != nil {
		r.onStepUpdate(label)
	}
	if r.showProgress {
		r.tracker.UpdateStep(label)
	}
}

func (r *ConsoleStepRunner) remember(baseLabel, label string) {
	r.mu.Lock()
	r.currentLabels[baseLabel] = label
	r.mu.Unlock()
}
