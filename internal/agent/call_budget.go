package agent

import (
	"context"
	"strings"
)

type callBudgetKey struct{}

type callBudget struct {
	calls   chan struct{}
	reviews chan struct{}
}

// WithCallBudget 为同一次运行及其子项目共享 Agent 调用上限；嵌套调用不扩大额度。
// 知识审查额外串行化，等待审查资格时不占用通用调用额度。
func WithCallBudget(ctx context.Context, parallelism int) context.Context {
	if _, ok := ctx.Value(callBudgetKey{}).(*callBudget); ok {
		return ctx
	}
	return context.WithValue(ctx, callBudgetKey{}, &callBudget{
		calls: make(chan struct{}, max(1, parallelism)), reviews: make(chan struct{}, 1),
	})
}

func acquireCall(ctx context.Context, operation string) (func(), error) {
	budget, ok := ctx.Value(callBudgetKey{}).(*callBudget)
	if !ok {
		return func() {}, ctx.Err()
	}
	acquire := func(slots chan struct{}) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case slots <- struct{}{}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	review := operation == "ReviewKnowledge" || strings.HasPrefix(operation, "ReviewKnowledge/")
	if review {
		if err := acquire(budget.reviews); err != nil {
			return nil, err
		}
	}
	if err := acquire(budget.calls); err != nil {
		if review {
			<-budget.reviews
		}
		return nil, err
	}
	return func() {
		<-budget.calls
		if review {
			<-budget.reviews
		}
	}, nil
}
