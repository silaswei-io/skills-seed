package fileanalysis

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type SelectOptions struct {
	Root          string
	Policy        SelectionPolicy
	FocusAbsPaths []string
}

type SkippedFile struct {
	Path   string
	Reason SkipReason
}

type FileSelection struct {
	Files   []domain.FileInfo
	Skipped []SkippedFile
}

func SelectFiles(opts SelectOptions) (*FileSelection, error) {
	focusRelPaths := utils.RelativePaths(opts.Root, opts.FocusAbsPaths)
	selection := &FileSelection{
		Files:   []domain.FileInfo{},
		Skipped: []SkippedFile{},
	}
	err := filepath.WalkDir(opts.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == opts.Root {
				return err
			}
			if relPath := relativePath(opts.Root, path); relPath != "" {
				selection.addSkipped(relPath, SkipReasonUnreadable)
			}
			return nil
		}
		if path == opts.Root {
			return nil
		}

		relPath := relativePath(opts.Root, path)
		if relPath == "" {
			return nil
		}
		if opts.Policy.IsExcluded(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			selection.addSkipped(relPath, SkipReasonExcluded)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !pathInFocus(relPath, focusRelPaths) {
			selection.addSkipped(relPath, SkipReasonOutOfFocus)
			return nil
		}
		decision := opts.Policy.Decide(relPath)
		if !decision.Include {
			selection.addSkipped(relPath, decision.Reason)
			return nil
		}
		selection.Files = append(selection.Files, domain.NewFileInfo(relPath, ""))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(selection.Files, func(i, j int) bool { return selection.Files[i].Path < selection.Files[j].Path })
	sort.Slice(selection.Skipped, func(i, j int) bool { return selection.Skipped[i].Path < selection.Skipped[j].Path })
	return selection, nil
}

func (s *FileSelection) Paths() []string {
	if s == nil {
		return nil
	}
	paths := make([]string, 0, len(s.Files))
	for _, file := range s.Files {
		paths = append(paths, file.Path)
	}
	sort.Strings(paths)
	return paths
}

func (s *FileSelection) SkippedCount(reason SkipReason) int {
	if s == nil {
		return 0
	}
	count := 0
	for _, skipped := range s.Skipped {
		if skipped.Reason == reason {
			count++
		}
	}
	return count
}

func (s *FileSelection) addSkipped(path string, reason SkipReason) {
	if s == nil {
		return
	}
	s.Skipped = append(s.Skipped, SkippedFile{Path: filepath.ToSlash(path), Reason: reason})
}

func relativePath(root string, path string) string {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." {
		return ""
	}
	return strings.TrimPrefix(relPath, "./")
}

func pathInFocus(path string, focusPaths []string) bool {
	if len(focusPaths) == 0 {
		return true
	}
	path = strings.Trim(filepath.ToSlash(path), "/")
	for _, focus := range focusPaths {
		focus = strings.Trim(filepath.ToSlash(focus), "/")
		if focus == "" || focus == "." {
			return true
		}
		if path == focus || strings.HasPrefix(path, focus+"/") {
			return true
		}
	}
	return false
}
