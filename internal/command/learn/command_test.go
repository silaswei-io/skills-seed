package learn

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/stretchr/testify/require"
)

func TestCmd_CurrentIncludesFocusAndProfileFlags(t *testing.T) {
	cmd := Cmd(&container.Container{})
	currentCmd, _, err := cmd.Find([]string{"current"})
	require.NoError(t, err)

	focusFlag := currentCmd.Flags().Lookup("focus")
	require.NotNil(t, focusFlag)
	require.Equal(t, "f", focusFlag.Shorthand)

	profileFlag := currentCmd.Flags().Lookup("profile")
	require.NotNil(t, profileFlag)
	require.Equal(t, "auto", profileFlag.DefValue)

	contextFlag := currentCmd.Flags().Lookup("context")
	require.NotNil(t, contextFlag)

	contextPathFlag := currentCmd.Flags().Lookup("context-path")
	require.NotNil(t, contextPathFlag)

	forceFlag := currentCmd.Flags().Lookup("force")
	require.NotNil(t, forceFlag)
}
