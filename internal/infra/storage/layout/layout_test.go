package layout

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunJournalUsesStoreRoot(t *testing.T) {
	l := New("/tmp/demo")
	require.Equal(t, filepath.Join("/tmp/demo", "store", "journal"), l.RunJournal())
}

func TestLayoutPaths(t *testing.T) {
	seedPath := filepath.Join("project", ".skills-seed")
	l := New(seedPath)

	require.Equal(t, filepath.Join(seedPath, "store", "nested"), l.Store("nested"))
	require.Equal(t, filepath.Join(seedPath, "store", "documents", "nested"), l.StoreDocuments("nested"))
	require.Equal(t, filepath.Join(seedPath, "cache", "nested"), l.Cache("nested"))
	require.Equal(t, filepath.Join(seedPath, "runtime", "nested"), l.Runtime("nested"))
	require.Equal(t, filepath.Join(seedPath, "store", "project.db"), l.ProjectDB())
	require.Equal(t, filepath.Join(seedPath, "config.yaml"), l.Config())
	require.Equal(t, filepath.Join(seedPath, "store", "documents", "project-profile.json"), l.ProjectProfile())
	require.Equal(t, filepath.Join(seedPath, "store", "documents", "projects", "api", "profile.json"), l.ProjectDocument("api", "profile.json"))
	require.Equal(t, filepath.Join(seedPath, "store", "documents", "workspace-profile.json"), l.WorkspaceProfile())
	require.Equal(t, filepath.Join(seedPath, "store", "documents", "workspace-spec.json"), l.WorkspaceSpec())
	require.Equal(t, filepath.Join(seedPath, "store", "documents", "state.json"), l.State())
	require.Equal(t, filepath.Join(seedPath, "store", "documents", "change-log.json"), l.ChangeLog())
	require.Equal(t, filepath.Join(seedPath, "cache", "snapshots"), l.Snapshots())
	require.Equal(t, filepath.Join(seedPath, "cache", "commands", "learn", "state.json"), l.CommandState("learn"))
	require.Equal(t, filepath.Join(seedPath, "runtime", "logs"), l.RuntimeLogs())
	require.Equal(t, filepath.Join(seedPath, "rules"), l.Rules())
	require.Equal(t, filepath.Join(seedPath, "workflows"), l.Workflows())
	require.Equal(t, filepath.Join(seedPath, "cache", "commands"), l.CommandStates())
}
