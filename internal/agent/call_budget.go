package agent

import (
	"context"
	"runtime"
)

// autoParallelismCap 限制自动模式下的 Agent 进程数，避免与本机资源规模线性增长。
const autoParallelismCap = 6

// EffectiveParallelism 统一解析配置值；正数为显式额度，零为按本机资源自动分配。
func EffectiveParallelism(configured int) int {
	if configured > 0 {
		return configured
	}
	return min(autoParallelismCap, max(1, runtime.NumCPU()))
}

type callBudgetKey struct{}

type callBudget struct {
	calls chan struct{}
}

// WithCallBudget 为同一次运行及其子项目共享 Agent 调用上限；嵌套调用不扩大额度。
func WithCallBudget(ctx context.Context, parallelism int) context.Context {
	if _, ok := ctx.Value(callBudgetKey{}).(*callBudget); ok {
		return ctx
	}
	return context.WithValue(ctx, callBudgetKey{}, &callBudget{
		calls: make(chan struct{}, EffectiveParallelism(parallelism)),
	})
}

func acquireCall(ctx context.Context) (func(), error) {
	budget, ok := ctx.Value(callBudgetKey{}).(*callBudget)
	if !ok {
		return func() {}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case budget.calls <- struct{}{}:
		return func() { <-budget.calls }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
