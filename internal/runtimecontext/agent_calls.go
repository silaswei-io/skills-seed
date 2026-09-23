package runtimecontext

import (
	"context"
	"sort"
	"strings"
	"sync"
)

type agentCallsKey struct{}

// AgentCallCounter 统计一次 learn/sync 运行中的结构化 Agent 调用次数。
type AgentCallCounter struct {
	mu   sync.Mutex
	byOp map[string]int
}

// NewAgentCallCounter 创建可附加到 context 的调用计数器。
func NewAgentCallCounter() *AgentCallCounter {
	return &AgentCallCounter{byOp: make(map[string]int)}
}

// WithAgentCallCounter 将调用计数器附加到 ctx。
func WithAgentCallCounter(ctx context.Context, counter *AgentCallCounter) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if counter == nil {
		return ctx
	}
	return context.WithValue(ctx, agentCallsKey{}, counter)
}

// AgentCalls 返回 ctx 上的调用计数器；未附加时返回 nil。
func AgentCalls(ctx context.Context) *AgentCallCounter {
	if ctx == nil {
		return nil
	}
	counter, _ := ctx.Value(agentCallsKey{}).(*AgentCallCounter)
	return counter
}

// Add 记录一次 Agent 操作调用。
func (c *AgentCallCounter) Add(operation string) {
	if c == nil {
		return
	}
	operation = strings.TrimSpace(operation)
	if operation == "" {
		operation = "unknown"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byOp == nil {
		c.byOp = make(map[string]int)
	}
	c.byOp[operation]++
}

// Snapshot 返回按操作名排序的调用次数副本。
func (c *AgentCallCounter) Snapshot() map[string]int {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.byOp) == 0 {
		return nil
	}
	out := make(map[string]int, len(c.byOp))
	for k, v := range c.byOp {
		out[k] = v
	}
	return out
}

// Total 返回全部操作的调用总数。
func (c *AgentCallCounter) Total() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, n := range c.byOp {
		total += n
	}
	return total
}

// SortedOperations 返回有调用记录的操作名（字典序）。
func (c *AgentCallCounter) SortedOperations() []string {
	snap := c.Snapshot()
	if len(snap) == 0 {
		return nil
	}
	names := make([]string, 0, len(snap))
	for name := range snap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
