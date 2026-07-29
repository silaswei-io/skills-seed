package changelog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuilderWritesRecentEntry(t *testing.T) {
	seedPath := t.TempDir()

	builder := Start(seedPath, "sync")
	builder.Detail("learned 2 patterns")
	require.NoError(t, builder.Save("sync updated skills"))

	entries, err := Recent(seedPath, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "sync", entries[0].Command)
	require.Equal(t, "sync updated skills", entries[0].Summary)
	require.Equal(t, []string{"learned 2 patterns"}, entries[0].Details)
}

func TestRecentSortsNewestFirst(t *testing.T) {
	seedPath := t.TempDir()

	require.NoError(t, Append(seedPath, Entry{Command: "first", Summary: "first"}))
	require.NoError(t, Append(seedPath, Entry{Command: "second", Summary: "second"}))

	entries, err := Recent(seedPath, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "second", entries[0].Command)
}
