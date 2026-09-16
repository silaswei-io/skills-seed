package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResultRepairUsesArchivedOutputAndStopsAfterOneRepair(t *testing.T) {
	repair := NewResultRepair("Analyze the complete focus; candidate ID = exact-id")
	err := NewResultContractError("test", "ReviewKnowledge", 1, errors.New("missing verdict"), "invalid JSON", AgentOutputArchive{ContentPath: "/tmp/previous.json"})
	require.True(t, repair.Prepare(err, true))
	require.Contains(t, repair.Prompt(), "exact-id")
	require.Contains(t, repair.Prompt(), "/tmp/previous.json")
	require.Contains(t, repair.Prompt(), "missing verdict")
	require.Contains(t, repair.Prompt(), "Do not repeat repository discovery")
	require.False(t, repair.Prepare(err, true))
}

func TestResultRepairDoesNotConsumeBudgetForTransientFailure(t *testing.T) {
	repair := NewResultRepair("original task")
	require.True(t, repair.Prepare(errors.New("HTTP 529"), true))
	require.Equal(t, "original task", repair.Prompt())
	err := NewResultContractError("test", "ReviewKnowledge", 2, errors.New("missing verdict"), "", AgentOutputArchive{RawPath: "/tmp/raw.json"})
	require.True(t, repair.Prepare(err, true))
}

func TestResultRepairFailsClosedWithoutRecoverableOutput(t *testing.T) {
	repair := NewResultRepair("original task")
	err := NewResultContractError("test", "ReviewKnowledge", 1, errors.New("missing result"), "", AgentOutputArchive{})
	require.False(t, repair.Prepare(err, true))
}

func TestResultRepairSkipsTransientBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	_, err := RunRetryingCall(ctx, RetryingCallOptions[string]{
		Policy: retryPolicyStub{maxRetries: 1, wait: time.Hour},
		Call: func(int) (string, string, time.Duration, bool, error) {
			calls++
			if calls == 1 {
				return "", "", 0, true, NewResultContractError("test", "review", 1, errors.New("invalid"), "", AgentOutputArchive{})
			}
			return "ok", "", 0, false, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
