package commandutil

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestCommandStateScopeUsesCommandPath(t *testing.T) {
	require.Equal(t, "sync", CommandStateScope("sync"))
	require.Equal(t, "learn-current", CommandStateScope("learn current"))
	require.Equal(t, "generate-skills", CommandStateScope("generate skills"))
}

func TestCommandStateScopeForCobra(t *testing.T) {
	root := &cobra.Command{Use: "skills-seed"}
	syncCmd := &cobra.Command{Use: "sync"}
	root.AddCommand(syncCmd)

	require.Equal(t, "sync", CommandStateScopeForCobra(syncCmd))
}
