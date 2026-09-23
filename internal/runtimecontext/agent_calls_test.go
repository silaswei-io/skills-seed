package runtimecontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentCallCounterSnapshot(t *testing.T) {
	counter := NewAgentCallCounter()
	ctx := WithAgentCallCounter(context.Background(), counter)
	AgentCalls(ctx).Add("Plan")
	AgentCalls(ctx).Add("Plan")
	AgentCalls(ctx).Add("Review")

	require.Equal(t, 3, counter.Total())
	require.Equal(t, map[string]int{"Plan": 2, "Review": 1}, counter.Snapshot())
	require.Equal(t, []string{"Plan", "Review"}, counter.SortedOperations())
	require.Nil(t, AgentCalls(context.Background()))
}
