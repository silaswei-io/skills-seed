package log

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceJournalEntriesIncludesChildProjectRuns(t *testing.T) {
	workspaceRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspaceRoot, ".git"), 0755))
	rootSeed := filepath.Join(workspaceRoot, ".skills-seed")
	rootRepo, err := config.NewRepository(rootSeed, "zh-CN")
	require.NoError(t, err)
	cfg := rootRepo.Get()
	cfg.Project.Name = "demo-workspace"
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.RootPath = workspaceRoot
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend"}}
	require.NoError(t, rootRepo.Update(cfg))

	childSeed := filepath.Join(workspaceRoot, "backend", ".skills-seed")
	require.NoError(t, os.MkdirAll(filepath.Dir(childSeed), 0755))
	require.NoError(t, runjournal.Append(childSeed, runjournal.Entry{
		Command:    "learn current",
		Summary:    "学习当前代码",
		StartedAt:  time.Now().Add(-time.Minute),
		FinishedAt: time.Now(),
		Scope: runjournal.Scope{
			Kind:        runjournal.ScopeChild,
			Name:        "backend",
			ProjectPath: filepath.Join(workspaceRoot, "backend"),
		},
	}))

	entries, err := workspaceJournalEntries(rootSeed)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "learn current", entries[0].Command)
	require.Equal(t, runjournal.ScopeChild, entries[0].Scope.Kind)
	require.Contains(t, entries[0].Scope.ProjectPath, "backend")
}
