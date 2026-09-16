package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCallBudgetSharesSlotsAcrossNestedProjects(t *testing.T) {
	ctx := WithCallBudget(context.Background(), 1)
	child := WithCallBudget(ctx, 8)
	release, err := acquireCall(ctx, "AnalyzeCurrentCodebase/batch-001")
	require.NoError(t, err)
	defer release()
	waitCtx, cancel := context.WithTimeout(child, 20*time.Millisecond)
	defer cancel()
	called := false
	_, err = RunRetryingCall(waitCtx, RetryingCallOptions[string]{Call: func(int) (string, string, time.Duration, bool, error) {
		called = true
		return "", "", 0, false, nil
	}})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, called)
}

func TestReviewBudgetDoesNotConsumeSourceSlotWhileWaiting(t *testing.T) {
	ctx := WithCallBudget(context.Background(), 2)
	releaseReview, err := acquireCall(ctx, "ReviewKnowledge/batch-001")
	require.NoError(t, err)
	releaseSource, err := acquireCall(ctx, "AnalyzeCurrentCodebase/batch-002")
	require.NoError(t, err)
	releaseSource()
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	_, err = acquireCall(waitCtx, "ReviewKnowledge/batch-003")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	releaseSource, err = acquireCall(ctx, "AnalyzeCurrentCodebase/batch-004")
	require.NoError(t, err)
	releaseSource()
	releaseReview()
	releaseReview, err = acquireCall(ctx, "ReviewKnowledge/batch-005")
	require.NoError(t, err)
	releaseReview()
}
