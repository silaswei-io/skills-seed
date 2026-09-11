package runjournal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAppendAndRecent(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	base := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	entries := []Entry{
		{ID: "first", Command: "learn current", Summary: "first", StartedAt: base, FinishedAt: base.Add(time.Minute)},
		{ID: "second", Command: "generate", Summary: "second", StartedAt: base.Add(time.Minute), FinishedAt: base.Add(2 * time.Minute)},
	}
	for _, entry := range entries {
		require.NoError(t, Append(seedPath, entry))
	}

	recent, err := Recent(seedPath, 1)
	require.NoError(t, err)
	require.Equal(t, []Entry{entries[1]}, recent)

	recent, err = Recent(seedPath, 0)
	require.NoError(t, err)
	require.Equal(t, []Entry{entries[1], entries[0]}, recent)
}

func TestAppendFillsDefaultsAndSkipsEmptySeedPath(t *testing.T) {
	require.NoError(t, Append(" ", Entry{}))

	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	require.NoError(t, Append(seedPath, Entry{Command: "learn current"}))
	recent, err := Recent(seedPath, 0)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	require.NotEmpty(t, recent[0].ID)
	require.Equal(t, "learn current", recent[0].Summary)
	require.False(t, recent[0].StartedAt.IsZero())
	require.False(t, recent[0].FinishedAt.IsZero())
}

func TestRecentUsesStartedAtAndIDAsSortFallbacks(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	started := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	for _, id := range []string{"a", "b"} {
		require.NoError(t, Append(seedPath, Entry{ID: id, Command: "run", StartedAt: started}))
	}

	recent, err := Recent(seedPath, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "a"}, []string{recent[0].ID, recent[1].ID})
}

func TestJournalReportsFilesystemAndJSONErrors(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "seed")
	require.NoError(t, os.WriteFile(parentFile, []byte("file"), 0o644))
	require.Error(t, Append(parentFile, Entry{ID: "run", Command: "run"}))

	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	require.NoError(t, os.MkdirAll(Path(seedPath), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(Path(seedPath), "invalid.json"), []byte("{"), 0o644))
	_, err := Recent(seedPath, 0)
	require.Error(t, err)

	_, err = Recent("[", 0)
	require.Error(t, err)
}

func TestJournalPathAndNames(t *testing.T) {
	seedPath := filepath.Join("project", ".skills-seed")
	require.Equal(t, filepath.Join(seedPath, "store", "journal"), Path(seedPath))
	require.Equal(t, filepath.Join(Path(seedPath), "run.json"), entryPath(seedPath, "run"))
	require.Equal(t, "run", safeCommandName(""))
	require.Equal(t, "learn-current", safeCommandName(" learn current "))
	require.Equal(t, "a-b_c", safeCommandName("a/b_c"))

	first := newID("learn current")
	second := newID("learn current")
	require.NotEqual(t, first, second)
	require.True(t, strings.Contains(first, "learn-current"))
}
