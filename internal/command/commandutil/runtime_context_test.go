package commandutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveRuntimeContextPrefersInlineContext(t *testing.T) {
	contextFile := filepath.Join(t.TempDir(), "context.md")
	require.NoError(t, os.WriteFile(contextFile, []byte("file context"), 0644))

	got, err := ResolveRuntimeContext("inline context", contextFile)

	require.NoError(t, err)
	require.Equal(t, "inline context", got)
}

func TestResolveRuntimeContextReadsContextFile(t *testing.T) {
	contextFile := filepath.Join(t.TempDir(), "context.md")
	require.NoError(t, os.WriteFile(contextFile, []byte("file context\n"), 0644))

	got, err := ResolveRuntimeContext("", contextFile)

	require.NoError(t, err)
	require.Equal(t, "file context", got)
}
