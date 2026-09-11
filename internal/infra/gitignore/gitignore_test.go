package gitignore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatcherParsesPathsAndMatchesDirectoryPrefixes(t *testing.T) {
	matcher := newMatcherFromGitOutput([]byte(" ignored.txt\x00./cache/\x00nested/file.log\x00\x00"))

	require.Equal(t, []string{"cache/", "ignored.txt", "nested/file.log"}, matcher.Paths())
	require.True(t, matcher.Match("./ignored.txt"))
	require.True(t, matcher.Match("cache/results/output.json"))
	require.True(t, matcher.Match("nested/file.log"))
	require.False(t, matcher.Match("nested/other.log"))
	require.False(t, matcher.Match("."))
}

func TestNilAndEmptyMatcher(t *testing.T) {
	var matcher *Matcher
	require.False(t, matcher.Match("anything"))
	require.Nil(t, matcher.Paths())

	empty, err := NewMatcher(context.Background(), " ")
	require.NoError(t, err)
	require.False(t, empty.Match("anything"))
	require.Empty(t, empty.Paths())
}

func TestNewMatcherReadsGitIgnoreRules(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\ncache/\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("ignored"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(root, "cache"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cache", "value"), []byte("ignored"), 0o644))

	matcher, err := NewMatcher(absentContext(), root)
	require.NoError(t, err)
	require.True(t, matcher.Match("ignored.txt"))
	require.True(t, matcher.Match("cache/value"))

	_, err = NewMatcher(context.Background(), filepath.Join(root, "missing"))
	require.Error(t, err)
}

func absentContext() context.Context {
	return nil
}

func TestNormalizePath(t *testing.T) {
	require.Equal(t, "dir/file", normalizePath(" ./dir/file "))
	require.Equal(t, "dir/", normalizePath("/dir//"))
	require.Empty(t, normalizePath("."))
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}
