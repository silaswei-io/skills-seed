package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRepositorySaveReplacesPairAndPreservesScripts(t *testing.T) {
	repo := NewRepository(t.TempDir())
	require.NoError(t, repo.Save(domain.Workflow{ID: "deploy", Name: "Deploy", Content: "old"}))
	scriptPath := filepath.Join(repo.ScriptsDir("deploy"), "run.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\n"), 0o755))

	require.NoError(t, repo.Save(domain.Workflow{ID: "deploy", Name: "Deploy v2", Content: "new"}))

	workflow, err := repo.Get("deploy")
	require.NoError(t, err)
	require.Equal(t, "Deploy v2", workflow.Name)
	require.Equal(t, "new", workflow.Content)
	require.FileExists(t, scriptPath)
}
