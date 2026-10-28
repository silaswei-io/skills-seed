package agent

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEffectiveParallelism(t *testing.T) {
	require.Equal(t, 2, EffectiveParallelism(2))
	require.Equal(t, min(autoParallelismCap, max(1, runtime.NumCPU())), EffectiveParallelism(0))
	require.Positive(t, EffectiveParallelism(0))
}

func TestCallBudgetSharesSlotsAcrossNestedProjects(t *testing.T) {
	ctx := WithCallBudget(context.Background(), 1)
	child := WithCallBudget(ctx, 8)
	release, err := acquireCall(ctx)
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

func TestReviewCallsShareConfiguredBudgetWithAnalysis(t *testing.T) {
	ctx := WithCallBudget(context.Background(), 2)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := RunRetryingCall(ctx, RetryingCallOptions[string]{
				Operation: "ReviewKnowledge/batch",
				Call: func(int) (string, string, time.Duration, bool, error) {
					started <- struct{}{}
					<-release
					return "reviewed", "", 0, false, nil
				},
			})
			done <- err
		}()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("two review calls did not start concurrently")
		}
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	called := false
	_, err := RunRetryingCall(waitCtx, RetryingCallOptions[string]{
		Operation: "AnalyzeCurrentCodebase/batch",
		Call: func(int) (string, string, time.Duration, bool, error) {
			called = true
			return "", "", 0, false, nil
		},
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, called)
	close(release)
	require.NoError(t, <-done)
	require.NoError(t, <-done)
}
