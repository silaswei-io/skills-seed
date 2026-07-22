package sourcecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverGoTestsAssignsTestsToNearestModule(t *testing.T) {
	root := t.TempDir()
	writeGoTestFile(t, root, "go.mod")
	writeGoTestFile(t, root, "root_test.go")
	writeGoTestFile(t, root, "plugins/a/go.mod")
	writeGoTestFile(t, root, "plugins/a/a_test.go")
	writeGoTestFile(t, root, "plugins/a/child/b_test.go")
	writeGoTestFile(t, root, "plugins/b/go.mod")

	inventory, err := DiscoverGoTests(root)

	require.NoError(t, err)
	require.Equal(t, []GoTestModule{
		{Workdir: ".", ModFile: "go.mod", TestFiles: []string{"root_test.go"}},
		{Workdir: "plugins/a", ModFile: "plugins/a/go.mod", TestFiles: []string{"plugins/a/a_test.go", "plugins/a/child/b_test.go"}},
		{Workdir: "plugins/b", ModFile: "plugins/b/go.mod"},
	}, inventory.Modules)
}

func TestDiscoverGoTestsReportsTestsWithoutModule(t *testing.T) {
	root := t.TempDir()
	writeGoTestFile(t, root, "orphan_test.go")

	inventory, err := DiscoverGoTests(root)

	require.NoError(t, err)
	require.Equal(t, []string{"orphan_test.go"}, inventory.UnownedTestFiles)
}

func writeGoTestFile(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte("module test\n"), 0644))
}
