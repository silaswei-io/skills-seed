package embedfs

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedTemplatesAreAvailable(t *testing.T) {
	entries, err := fs.ReadDir(FS, "templates")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	data, err := FS.ReadFile("templates/README.md")
	require.NoError(t, err)
	require.NotEmpty(t, data)
}
