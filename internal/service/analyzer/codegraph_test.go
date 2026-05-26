package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLICodeGraphCollectorUnavailableCommand(t *testing.T) {
	collector := &cliCodeGraphCollector{command: "missing-codegraph-command"}

	_, err := collector.Collect(context.Background(), t.TempDir(), codeGraphContextRequest{})

	require.ErrorIs(t, err, errCodeGraphUnavailable)
}

func TestCLICodeGraphCollectorRequiresInitializedIndex(t *testing.T) {
	collector := &cliCodeGraphCollector{command: fakeExecutable(t, "codegraph", ""), autoInit: false}

	_, err := collector.Collect(context.Background(), t.TempDir(), codeGraphContextRequest{})

	require.ErrorIs(t, err, errCodeGraphNotInitialized)
}

func TestCLICodeGraphCollectorBuildsContext(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(projectRoot, ".codegraph"), 0755))
	collector := &cliCodeGraphCollector{
		command: fakeExecutable(t, "codegraph", `case "$1" in
  sync) echo "synced" ;;
  status) echo "Files indexed: 10" ;;
  context) echo "Handler calls Service" ;;
  *) echo "unexpected $1"; exit 2 ;;
esac`),
		autoSync: true,
	}

	result, err := collector.Collect(context.Background(), projectRoot, codeGraphContextRequest{
		ProjectName: "demo",
		Language:    "go",
		MaxNodes:    3,
		MaxCode:     0,
	})

	require.NoError(t, err)
	require.Contains(t, result, "Files indexed: 10")
	require.Contains(t, result, "Handler calls Service")
}

func fakeExecutable(t *testing.T, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fake executable test is not supported on windows")
	}
	path := filepath.Join(t.TempDir(), name)
	content := "#!/bin/sh\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0755))
	return path
}
