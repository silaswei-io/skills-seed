package fileanalysis

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/stretchr/testify/require"
)

type fakeFileSelector struct {
	result *agent.SelectFilesResult
	req    *agent.SelectFilesRequest
}

func (f *fakeFileSelector) SelectFiles(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
	f.req = req
	return f.result, nil
}

func TestApplyAIFileSelectorAppliesIncludeExcludeSafely(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "logic"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "types"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "logic", "create.go"), []byte("package logic"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "types", "types.go"), []byte("package types"), 0644))

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{
		Include:       []string{"internal/**", "/tmp/outside", "../bad"},
		Exclude:       []string{"internal/types/**"},
		SelectedPaths: []string{"internal/logic/create.go"},
		Reason:        "high-signal handwritten implementation",
	}}

	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"internal/logic/create.go", "internal/types/types.go"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"internal/logic/create.go"}, result.SelectedPaths)
	require.Equal(t, []string{"internal/types/types.go"}, result.SkippedPaths)
	require.Equal(t, 2, selector.req.CandidateNum)
	require.Contains(t, selector.req.FileTree, "create.go")
	require.Contains(t, selector.req.FileTree, "internal/")
	require.Contains(t, selector.req.FileTree, "logic/")
	require.NotContains(t, selector.req.FileTree, "/tmp/outside")
}

func TestApplyAIFileSelectorFallsBackWhenAISelectsNothing(t *testing.T) {
	selector := &fakeFileSelector{result: &agent.SelectFilesResult{
		Include: []string{"../bad"},
		Exclude: []string{"**/*"},
	}}

	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: t.TempDir(),
		Candidates:  []string{"a.go", "b.go"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
}

func TestApplyAIFileSelectorHandlesNilAIResult(t *testing.T) {
	selector := &fakeFileSelector{}

	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: t.TempDir(),
		Candidates:  []string{"a.go", "b.go"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
}
