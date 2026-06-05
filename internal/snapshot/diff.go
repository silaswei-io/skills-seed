package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// ChangeStatus describes how a current file compares with its previous snapshot.
type ChangeStatus string

const (
	ChangeAdded     ChangeStatus = "added"
	ChangeModified  ChangeStatus = "modified"
	ChangeUnchanged ChangeStatus = "unchanged"
)

// FileChange is the comparison result for one current file.
type FileChange struct {
	Path     string
	Status   ChangeStatus
	DiffPath string
}

// Compare compares current files to old snapshots and writes diffs for modified files.
func Compare(currentFiles map[string]string, oldSnapshots map[string]string, runtimeDir string) ([]FileChange, error) {
	paths := make([]string, 0, len(currentFiles))
	for path := range currentFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	changes := make([]FileChange, 0, len(paths))
	for _, path := range paths {
		current := currentFiles[path]
		old, ok := oldSnapshots[path]
		if !ok {
			changes = append(changes, FileChange{Path: path, Status: ChangeAdded})
			continue
		}
		if old == current {
			changes = append(changes, FileChange{Path: path, Status: ChangeUnchanged})
			continue
		}

		diffPath, err := WriteUnifiedDiff(runtimeDir, path, old, current)
		if err != nil {
			return nil, err
		}
		changes = append(changes, FileChange{Path: path, Status: ChangeModified, DiffPath: diffPath})
	}
	return changes, nil
}

// WriteUnifiedDiff writes a deterministic unified-style diff for one file.
func WriteUnifiedDiff(dir, path, oldContent, newContent string) (string, error) {
	diffPath, err := diffOutputPath(dir, path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(diffPath), 0o755); err != nil {
		return "", err
	}
	diff := unifiedDiff(path, oldContent, newContent)
	if err := os.WriteFile(diffPath, []byte(diff), 0o644); err != nil {
		return "", err
	}
	return diffPath, nil
}

func diffOutputPath(dir, path string) (string, error) {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.Trim(normalized, "/")
	if normalized == "" || filepath.IsAbs(path) || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/../") {
		return "", fmt.Errorf("unsafe diff path: %q", path)
	}
	return filepath.Join(dir, "diffs", filepath.FromSlash(normalized)+".diff"), nil
}

func unifiedDiff(path, oldContent, newContent string) string {
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: path,
		ToFile:   path,
		Context:  3,
	})
	if err != nil {
		return fallbackUnifiedDiff(path, oldContent, newContent)
	}
	return diff
}

func fallbackUnifiedDiff(path, oldContent, newContent string) string {
	var b strings.Builder
	b.WriteString("--- ")
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString("+++ ")
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString("@@\n")
	for _, line := range strings.Split(strings.TrimSuffix(oldContent, "\n"), "\n") {
		if line == "" && oldContent == "" {
			continue
		}
		b.WriteByte('-')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, line := range strings.Split(strings.TrimSuffix(newContent, "\n"), "\n") {
		if line == "" && newContent == "" {
			continue
		}
		b.WriteByte('+')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
