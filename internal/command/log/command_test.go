package log

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	"github.com/stretchr/testify/require"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

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

func TestCommandRunReportsMissingAndEmptyJournals(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	cmd := Cmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	require.Error(t, run(cmd))

	_, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "en-US")
	require.NoError(t, err)
	require.NoError(t, run(cmd))
	require.NotEmpty(t, output.String())
}

func TestCommandRunPrintsJournalEntries(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	seedPath := filepath.Join(root, ".skills-seed")
	_, err := config.NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	require.NoError(t, runjournal.Append(seedPath, runjournal.Entry{
		ID:      "run-1",
		Command: "sync",
		Summary: "synchronized",
		Scope:   runjournal.Scope{Kind: runjournal.ScopeProject, Name: "demo"},
	}))
	cmd := Cmd()
	var output bytes.Buffer
	cmd.SetOut(&output)

	require.NoError(t, run(cmd))
	require.Contains(t, output.String(), "run-1")
	require.Contains(t, output.String(), "synchronized")
}

func TestMergeEntriesSortsByCompletionStartedTimeAndID(t *testing.T) {
	base := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	primary := []runjournal.Entry{{ID: "a", FinishedAt: base}}
	secondary := []runjournal.Entry{
		{ID: "b", StartedAt: base.Add(time.Minute)},
		{ID: "c", FinishedAt: base},
	}

	merged := mergeEntries(primary, secondary)
	require.Equal(t, []string{"b", "c", "a"}, []string{merged[0].ID, merged[1].ID, merged[2].ID})
	require.Same(t, &primary[0], &mergeEntries(primary, nil)[0])
}

func TestPrintEntriesRendersDetailsAndChildren(t *testing.T) {
	base := time.Date(2026, 9, 11, 10, 0, 0, 0, time.Local)
	entry := runjournal.Entry{
		ID:         "root",
		Command:    "sync",
		Summary:    "root summary",
		Details:    []string{"", "root summary", "detail"},
		LogPath:    "runtime/logs/sync.log",
		StartedAt:  base,
		FinishedAt: base.Add(1500 * time.Millisecond),
		Scope: runjournal.Scope{
			Kind:        runjournal.ScopeWorkspace,
			Name:        "demo",
			ProjectPath: ".",
		},
		Children: []runjournal.Entry{{
			Command: "learn current",
			Summary: "child summary",
			Scope:   runjournal.Scope{Kind: runjournal.ScopeChild, Name: "api", ProjectPath: "api"},
		}},
	}
	var output bytes.Buffer
	require.NoError(t, printEntries(&output, []runjournal.Entry{entry, {ID: "second", Command: "generate"}}))
	rendered := output.String()
	require.Contains(t, rendered, "root")
	require.Contains(t, rendered, "detail")
	require.Contains(t, rendered, "child summary")
	require.Contains(t, rendered, "runtime/logs/sync.log")

	wantErr := errors.New("write failed")
	require.ErrorIs(t, printEntries(failingWriter{err: wantErr}, []runjournal.Entry{entry}), wantErr)
}

func TestFormattingHelpers(t *testing.T) {
	base := time.Date(2026, 9, 11, 10, 0, 0, 0, time.Local)
	require.Contains(t, formatRunTime(time.Time{}, base), "0s")
	require.Contains(t, formatRunTime(base, time.Time{}), "0s")
	require.NotEmpty(t, formatScope(runjournal.Scope{}))
	require.NotEmpty(t, formatScope(runjournal.Scope{Kind: runjournal.ScopeWorkspace, Name: "workspace"}))
	require.Contains(t, formatScope(runjournal.Scope{Kind: runjournal.ScopeChild, ProjectPath: "api"}), "api")
}

func TestWorkspaceJournalEntriesHandlesNonWorkspaceAndInvalidChildren(t *testing.T) {
	root := t.TempDir()
	seedPath := filepath.Join(root, ".skills-seed")
	repo, err := config.NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	require.Empty(t, mustWorkspaceEntries(t, seedPath))

	cfg := repo.Get()
	cfg.Project.Mode = domain.ModeWorkspace
	cfg.Project.RootPath = ""
	require.NoError(t, repo.Update(cfg))
	require.Empty(t, mustWorkspaceEntries(t, seedPath))

	cfg.Project.RootPath = root
	cfg.Workspace.Projects = []config.WorkspaceProjectConfig{{ID: "escape", Path: "../outside"}, {ID: "missing", Path: "missing"}}
	require.NoError(t, repo.Update(cfg))
	require.Empty(t, mustWorkspaceEntries(t, seedPath))

	_, err = workspaceJournalEntries(filepath.Join(root, "missing-seed"))
	require.Error(t, err)
}

func mustWorkspaceEntries(t *testing.T, seedPath string) []runjournal.Entry {
	t.Helper()
	entries, err := workspaceJournalEntries(seedPath)
	require.NoError(t, err)
	return entries
}
