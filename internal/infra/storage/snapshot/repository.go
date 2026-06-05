package snapshot

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Repository stores full-file snapshots under .skills-seed/memory/snapshots.
type Repository struct {
	dir string
}

// NewRepository creates a snapshot repository rooted at the seed path.
func NewRepository(seedPath string) *Repository {
	return &Repository{dir: filepath.Join(seedPath, "memory", "snapshots")}
}

// Load reads all snapshots as path -> content. Missing snapshot directories are empty.
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

// Replace atomically replaces all snapshots with the provided file contents.
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
	if normalized == "" || filepath.IsAbs(path) || normalized == "." || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/../") {
		return "", fmt.Errorf("unsafe snapshot path: %q", path)
	}
	return normalized, nil
}
