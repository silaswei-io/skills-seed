package sourcecode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInspectFilesReturnsDeterministicSourceFacts(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0o755))
	source := "package internal\n\ntype Service struct{}\n\nfunc (Service) Run() error { return nil }\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "service.go"), []byte(source), 0o644))

	facts := InspectFiles(context.Background(), root, []string{"internal/service.go", "internal/service.go", "../outside.go"})

	require.Len(t, facts, 1)
	require.Equal(t, "internal/service.go", facts[0].Path)
	require.Equal(t, int64(len(source)), facts[0].SizeBytes)
	require.Equal(t, 5, facts[0].LineCount)
	require.Equal(t, 3, facts[0].NonBlankLines)
	require.Len(t, facts[0].Symbols, 1)
	require.Equal(t, "Run", facts[0].Symbols[0].Name)
}

func TestInspectFilesKeepsDeclarativeFilesWithoutSymbols(t *testing.T) {
	root := t.TempDir()
	content := "enabled: true\nlimit: 3\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "config.yaml"), []byte(content), 0o644))

	facts := InspectFiles(context.Background(), root, []string{"config.yaml"})

	require.Len(t, facts, 1)
	require.Equal(t, 2, facts[0].LineCount)
	require.Equal(t, 2, facts[0].NonBlankLines)
	require.Empty(t, facts[0].Symbols)
}
