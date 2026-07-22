package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeInvocationErrorReportsTimeoutInsteadOfProcessSignal(t *testing.T) {
	err := NormalizeInvocationError(errors.New("signal: killed"), context.DeadlineExceeded, 30*time.Minute)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.EqualError(t, err, "agent invocation timed out after 30m0s: context deadline exceeded")
}

func TestNormalizeInvocationErrorReportsParentCancellation(t *testing.T) {
	err := NormalizeInvocationError(errors.New("signal: killed"), context.Canceled, 30*time.Minute)

	require.ErrorIs(t, err, context.Canceled)
	require.EqualError(t, err, "agent invocation canceled: context canceled")
}

func TestNormalizeInvocationErrorKeepsCommandFailure(t *testing.T) {
	runErr := errors.New("exit status 1")

	require.Same(t, runErr, NormalizeInvocationError(runErr, nil, 30*time.Minute))
}
