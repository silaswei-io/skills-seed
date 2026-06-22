package hook

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreCommitHookDelegatesToHookRun(t *testing.T) {
	content := preCommitHookContent()

	require.Contains(t, content, "skills-seed hook run")
	require.NotContains(t, content, "skills-seed check")
}

func TestHookActionArgs(t *testing.T) {
	require.Equal(t, []string{"sync"}, hookActionArgs(hookActionSync))
	require.Equal(t, []string{"learn", "current"}, hookActionArgs(hookActionLearn))
	require.Nil(t, hookActionArgs(hookActionSkip))
}
