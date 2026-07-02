package commandutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveRuntimeContextPrefersInlineContext(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "context.md")
	require.NoError(t, os.WriteFile(contextPath, []byte("file context"), 0644))

	got, err := ResolveRuntimeContext("inline context", contextPath)

	require.NoError(t, err)
	require.Equal(t, "inline context", got)
}

func TestResolveRuntimeContextReadsContextPath(t *testing.T) {
	contextPath := filepath.Join(t.TempDir(), "context.md")
	require.NoError(t, os.WriteFile(contextPath, []byte("file context\n"), 0644))

	got, err := ResolveRuntimeContext("", contextPath)

	require.NoError(t, err)
	require.Equal(t, "file context", got)
}

func TestResolveRuntimeContextReadsMultipleContextPaths(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a.md")
	dir := filepath.Join(root, "notes")
	second := filepath.Join(dir, "b.md")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(first, []byte("first"), 0644))
	require.NoError(t, os.WriteFile(second, []byte("second"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skip.bin"), []byte{0, 1, 2}, 0644))

	got, err := ResolveRuntimeContext("", dir, first)

	require.NoError(t, err)
	require.Contains(t, got, "# "+filepath.ToSlash(first))
	require.Contains(t, got, "first")
	require.Contains(t, got, "# "+filepath.ToSlash(second))
	require.Contains(t, got, "second")
	require.NotContains(t, got, "skip.bin")
}
