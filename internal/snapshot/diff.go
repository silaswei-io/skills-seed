package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// ChangeStatus 表示当前文件相对上一份快照的变化状态。
type ChangeStatus string

const (
	// ChangeAdded 表示文件是本次新增。
	ChangeAdded ChangeStatus = "added"
	// ChangeModified 表示文件内容相对上一份快照发生变化。
	ChangeModified ChangeStatus = "modified"
	// ChangeUnchanged 表示文件内容与上一份快照一致。
	ChangeUnchanged ChangeStatus = "unchanged"
	// ChangeDeleted 表示上一份快照中的文件在当前范围内已删除。
	ChangeDeleted ChangeStatus = "deleted"
)

// FileChange 表示单个文件的快照对比结果。
type FileChange struct {
	Path     string
	Status   ChangeStatus
	DiffPath string
}

// Compare 对比当前文件与旧快照，并为修改过的文件写入 diff。
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

// CompareScoped 对比当前文件，并报告 scopePaths 范围内旧快照已删除的文件。
// scopePaths 为空表示当前文件集合是完整范围。
func CompareScoped(currentFiles map[string]string, oldSnapshots map[string]string, runtimeDir string, scopePaths []string) ([]FileChange, error) {
	changes, err := Compare(currentFiles, oldSnapshots, runtimeDir)
	if err != nil {
		return nil, err
	}

	deleted := make([]string, 0)
	for path := range oldSnapshots {
		if _, ok := currentFiles[path]; ok {
			continue
		}
		if len(scopePaths) > 0 && !pathInScope(path, scopePaths) {
			continue
		}
		deleted = append(deleted, path)
	}
	sort.Strings(deleted)

	for _, path := range deleted {
		diffPath, err := WriteUnifiedDiff(runtimeDir, path, oldSnapshots[path], "")
		if err != nil {
			return nil, err
		}
		changes = append(changes, FileChange{Path: path, Status: ChangeDeleted, DiffPath: diffPath})
	}
	return changes, nil
}

func pathInScope(path string, scopePaths []string) bool {
	path = strings.Trim(filepath.ToSlash(path), "/")
	for _, scope := range scopePaths {
		scope = strings.Trim(filepath.ToSlash(scope), "/")
		if scope == "" {
			continue
		}
		if path == scope || strings.HasPrefix(path, scope+"/") {
			return true
		}
	}
	return false
}

// WriteUnifiedDiff 为单个文件写入确定性的 unified 风格 diff。
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
