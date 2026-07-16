package fileanalysis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

type fakeFileSelector struct {
	result *agent.SelectFilesResult
	req    *agent.SelectFilesRequest
	calls  int
	err    error
}

func (f *fakeFileSelector) SelectFiles(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
	f.calls++
	f.req = req
	if f.err != nil {
		return nil, f.err
	}
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
		ProjectRoot:       root,
		Candidates:        []string{"internal/logic/create.go", "internal/types/types.go"},
		StructuralContext: "## Structural Context\n- internal/logic/create.go is an entry",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"internal/logic/create.go"}, result.SelectedPaths)
	require.Equal(t, []string{"internal/types/types.go"}, result.SkippedPaths)
	require.Equal(t, 2, selector.req.CandidateNum)
	require.Contains(t, selector.req.FileTree, "create.go")
	require.Contains(t, selector.req.FileTree, "internal/")
	require.Contains(t, selector.req.FileTree, "logic/")
	require.Contains(t, selector.req.StructuralContext, "internal/logic/create.go is an entry")
	require.NotContains(t, selector.req.FileTree, "/tmp/outside")
}

func TestApplyAIFileSelectorRejectsNilAgentResult(t *testing.T) {
	root := t.TempDir()
	selector := &fakeFileSelector{}

	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"main.go"},
	})

	require.Error(t, err)
	require.Nil(t, result)
}

func TestApplyAIFileSelectorKeepsRequiredPaths(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "logic"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "types"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "logic", "create.go"), []byte("package logic"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "types", "types.go"), []byte("package types"), 0644))

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{
		SelectedPaths: []string{"internal/logic/create.go"},
		Exclude:       []string{"internal/types/**"},
		Reason:        "prefer implementation",
	}}

	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot:   root,
		Candidates:    []string{"internal/logic/create.go", "internal/types/types.go"},
		RequiredPaths: []string{"internal/types/types.go"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"internal/logic/create.go", "internal/types/types.go"}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
	require.Equal(t, []string{"internal/types/types.go"}, result.ForcedPaths)
	require.Equal(t, []string{"internal/logic/create.go"}, result.AIPaths)
}

func TestApplyAIFileSelectorStillNarrowsWithoutRequiredPaths(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package demo"), 0644))

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"a.go"}}}
	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.go"}, result.SelectedPaths)
	require.Equal(t, []string{"b.go"}, result.SkippedPaths)
	require.Empty(t, result.ForcedPaths)
}

func TestApplyAIFileSelectorKeepsFullCandidateTree(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "server"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "orders"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "server", "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "orders", "handler.go"), []byte("package orders"), 0644))

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"cmd/server/main.go"}}}
	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"cmd/server/main.go", "internal/orders/handler.go"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"cmd/server/main.go"}, result.AIPaths)
	require.Contains(t, selector.req.FileTree, "main.go")
	require.Contains(t, selector.req.FileTree, "handler.go")
}

func TestApplyAIFileSelectorKeepsAINarrowSelectionForLargeCandidateSets(t *testing.T) {
	root := t.TempDir()
	candidates := make([]string, 0, 120)
	for i := 0; i < 120; i++ {
		path := fmt.Sprintf("pkg/f%03d.go", i)
		candidates = append(candidates, path)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("package pkg"), 0644))
	}

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"pkg/f119.go"}}}
	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  candidates,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"pkg/f119.go"}, result.SelectedPaths)
	require.Len(t, result.SkippedPaths, 119)
}

func TestApplyAIFileSelectorIgnoresOverbroadIncludeWhenSelectedPathsExist(t *testing.T) {
	root := t.TempDir()
	candidates := make([]string, 0, 260)
	for i := 0; i < 260; i++ {
		path := fmt.Sprintf("packages/core/f%03d.ts", i)
		candidates = append(candidates, path)
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("export const value = true"), 0644))
	}

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{
		SelectedPaths: []string{"packages/core/f259.ts"},
		Include:       []string{"packages/core/**"},
		Reason:        "selected explicit entry and broad package context",
	}}
	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  candidates,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"packages/core/f259.ts"}, result.SelectedPaths)
	require.Len(t, result.SkippedPaths, 259)
}

func TestApplyAIFileSelectorKeepsOnlyStructuralSignalsFromOverbroadInclude(t *testing.T) {
	root := t.TempDir()
	candidates := make([]string, 0, 260)
	for i := 0; i < 258; i++ {
		path := fmt.Sprintf("packages/core/f%03d.ts", i)
		candidates = append(candidates, path)
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("export const value = true"), 0644))
	}
	for _, path := range []string{"packages/core/route.ts", "packages/core/service.ts"} {
		candidates = append(candidates, path)
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("export const value = true"), 0644))
	}

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{
		Include: []string{"packages/core/**"},
		Reason:  "broad package context",
	}}
	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  candidates,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"packages/core/route.ts", "packages/core/service.ts"}, result.SelectedPaths)
	require.Len(t, result.SkippedPaths, len(candidates)-2)
}

func TestApplyAIFileSelectorDoesNotRefillAIExcludedPaths(t *testing.T) {
	root := t.TempDir()
	candidates := make([]string, 0, 120)
	for i := 0; i < 60; i++ {
		path := fmt.Sprintf("packages/core/f%03d.ts", i)
		candidates = append(candidates, path)
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("export const value = true"), 0644))
	}
	for i := 0; i < 60; i++ {
		path := fmt.Sprintf("integration-tests/f%03d.ts", i)
		candidates = append(candidates, path)
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("export const testValue = true"), 0644))
	}

	selector := &fakeFileSelector{result: &agent.SelectFilesResult{
		SelectedPaths: []string{"packages/core/f059.ts"},
		Exclude:       []string{"integration-tests/**"},
	}}
	result, err := ApplyAIFileSelector(context.Background(), selector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  candidates,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"packages/core/f059.ts"}, result.SelectedPaths)
	for _, path := range result.SelectedPaths {
		require.NotContains(t, path, "integration-tests/")
	}
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

func TestApplyAIFileSelectorReusesCachedSelectionForSameInput(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package demo"), 0644))
	cachePath := filepath.Join(t.TempDir(), "selection.json")

	firstSelector := &fakeFileSelector{result: &agent.SelectFilesResult{
		SelectedPaths: []string{"a.go"},
		Reason:        "stable high-signal selection",
	}}
	first, err := ApplyAIFileSelector(context.Background(), firstSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"b.go", "a.go"},
		UserContext: "prefer runtime behavior",
		CachePath:   cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.go"}, first.SelectedPaths)
	require.Equal(t, 1, firstSelector.calls)

	secondSelector := &fakeFileSelector{err: fmt.Errorf("selector should not be called")}
	second, err := ApplyAIFileSelector(context.Background(), secondSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go"},
		UserContext: "prefer runtime behavior",
		CachePath:   cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.go"}, second.SelectedPaths)
	require.Equal(t, []string{"b.go"}, second.SkippedPaths)
	require.Equal(t, "stable high-signal selection", second.Reason)
	require.Zero(t, secondSelector.calls)
}

func TestApplyAIFileSelectorInvalidatesCacheWhenCandidatesChange(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package demo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "c.go"), []byte("package demo"), 0644))
	cachePath := filepath.Join(t.TempDir(), "selection.json")

	firstSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"a.go"}}}
	_, err := ApplyAIFileSelector(context.Background(), firstSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go"},
		CachePath:   cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, 1, firstSelector.calls)

	secondSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"c.go"}}}
	second, err := ApplyAIFileSelector(context.Background(), secondSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go", "c.go"},
		CachePath:   cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"c.go"}, second.SelectedPaths)
	require.Equal(t, 1, secondSelector.calls)
}

func TestApplyAIFileSelectorInvalidatesCacheWhenUserContextChanges(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package demo"), 0644))
	cachePath := filepath.Join(t.TempDir(), "selection.json")

	firstSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"a.go"}}}
	_, err := ApplyAIFileSelector(context.Background(), firstSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go"},
		UserContext: "prefer entry points",
		CachePath:   cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, 1, firstSelector.calls)

	secondSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"b.go"}}}
	second, err := ApplyAIFileSelector(context.Background(), secondSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go"},
		UserContext: "prefer background jobs",
		CachePath:   cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"b.go"}, second.SelectedPaths)
	require.Equal(t, 1, secondSelector.calls)
}

func TestApplyAIFileSelectorInvalidatesCacheWhenRequiredPathsChange(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package demo"), 0644))
	cachePath := filepath.Join(t.TempDir(), "selection.json")

	firstSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"a.go"}}}
	first, err := ApplyAIFileSelector(context.Background(), firstSelector, AISelectorOptions{
		ProjectRoot:   root,
		Candidates:    []string{"a.go", "b.go"},
		RequiredPaths: []string{"a.go"},
		CachePath:     cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.go"}, first.SelectedPaths)
	require.Equal(t, 1, firstSelector.calls)

	secondSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"a.go"}}}
	second, err := ApplyAIFileSelector(context.Background(), secondSelector, AISelectorOptions{
		ProjectRoot:   root,
		Candidates:    []string{"a.go", "b.go"},
		RequiredPaths: []string{"b.go"},
		CachePath:     cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, second.SelectedPaths)
	require.Equal(t, 1, secondSelector.calls)
}

func TestApplyAIFileSelectorInvalidatesCacheWhenChangeHashChanges(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package demo"), 0644))
	cachePath := filepath.Join(t.TempDir(), "selection.json")

	firstSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"a.go"}}}
	_, err := ApplyAIFileSelector(context.Background(), firstSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go"},
		Changes: &FileChanges{
			AddedOrModified: []string{"a.go", "b.go"},
			Records: []domain.FileAnalysisRecord{
				{Path: "a.go", Hash: "hash-a-1"},
				{Path: "b.go", Hash: "hash-b-1"},
			},
		},
		CachePath: cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, 1, firstSelector.calls)

	secondSelector := &fakeFileSelector{result: &agent.SelectFilesResult{SelectedPaths: []string{"b.go"}}}
	second, err := ApplyAIFileSelector(context.Background(), secondSelector, AISelectorOptions{
		ProjectRoot: root,
		Candidates:  []string{"a.go", "b.go"},
		Changes: &FileChanges{
			AddedOrModified: []string{"a.go", "b.go"},
			Records: []domain.FileAnalysisRecord{
				{Path: "a.go", Hash: "hash-a-2"},
				{Path: "b.go", Hash: "hash-b-1"},
			},
		},
		CachePath: cachePath,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"b.go"}, second.SelectedPaths)
	require.Equal(t, 1, secondSelector.calls)
}
