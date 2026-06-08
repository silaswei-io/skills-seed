package filetree

import (
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/filefilter"
)

// Walk 递归列出 root 下的文件，并把排除规则同时应用到目录和文件。
// 返回的文件路径使用斜杠分隔，并且相对于 root。
func Walk(root string, exclude []string) ([]domain.FileInfo, error) {
	files := []domain.FileInfo{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if filefilter.MatchExcluded(rel, exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		files = append(files, domain.NewFileInfo(rel, ""))
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}
