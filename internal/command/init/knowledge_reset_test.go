package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResetCmdResetsSelectedKnowledge(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	patternPath := filepath.Join(seedPath, "store", "project.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(patternPath), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "config.yaml"), []byte("project: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(patternPath, []byte("patterns"), 0o644))

	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectRoot))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previousDir)) })

	command := ResetCmd()
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetArgs([]string{"patterns"})
	require.NoError(t, command.Execute())
	require.NoFileExists(t, patternPath)
	require.NotEmpty(t, output.String())
}

func TestResetCmdRejectsInvalidKnowledgeScope(t *testing.T) {
	command := ResetCmd()
	command.SetArgs([]string{"unknown"})
	require.Error(t, command.Execute())
}

func TestResetCmdRejectsMixedKnowledgeScopes(t *testing.T) {
	command := ResetCmd()
	command.SetArgs([]string{"all", "patterns"})
	require.Error(t, command.Execute())
}
