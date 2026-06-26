package snapshot

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

// Repository 将完整文件快照保存到 .skills-seed/cache/snapshots 下。
type Repository struct {
	dir string
}

// NewRepository 创建以 seedPath 为根的快照仓储。
func NewRepository(seedPath string) *Repository {
	return &Repository{dir: layout.New(seedPath).Snapshots()}
}

// Load 读取所有快照，返回“路径 -> 内容”；快照目录不存在时返回空集合。
func (r *Repository) Load() (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(r.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == r.dir || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(r.dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, err
	}
	return files, nil
}

// Replace 用给定文件内容原子替换全部快照。
func (r *Repository) Replace(files map[string]string) error {
	parent := filepath.Dir(r.dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}

	tmp, err := os.MkdirTemp(parent, ".snapshots-tmp-*")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmp)
		}
	}()

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		safePath, err := safeRelativePath(path)
		if err != nil {
			return err
		}
		target := filepath.Join(tmp, filepath.FromSlash(safePath))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(files[path]), 0o644); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(r.dir); err != nil {
		return err
	}
	if err := os.Rename(tmp, r.dir); err != nil {
		return fmt.Errorf("replace snapshots: %w", err)
	}
	cleanup = false
	return nil
}

func safeRelativePath(path string) (string, error) {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.Trim(normalized, "/")
	if normalized == "" || filepath.IsAbs(path) || normalized == "." || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/../") || strings.HasSuffix(normalized, "/..") {
		return "", fmt.Errorf("unsafe snapshot path: %q", path)
	}
	// Clean the path and verify the result doesn't escape the base directory.
	cleaned := filepath.Clean(normalized)
	if cleaned == "." || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("unsafe snapshot path: %q", path)
	}
	return cleaned, nil
}
