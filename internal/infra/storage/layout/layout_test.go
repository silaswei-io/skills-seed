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
