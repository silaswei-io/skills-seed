package analyzer

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/utils/sourcefiles"
)

func engineeringKnowledgePaths(projectRoot string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(projectRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(projectRoot, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative != "." && skipEngineeringKnowledgeDir(relative) {
				return filepath.SkipDir
			}
			return nil
		}
		if sourcefiles.IsEngineeringKnowledge(relative) {
			paths = append(paths, relative)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func skipEngineeringKnowledgeDir(path string) bool {
	path = strings.Trim(filepath.ToSlash(path), "/")
	parts := strings.Split(path, "/")
	for index, part := range parts {
		switch part {
		case ".git", "vendor", "node_modules", ".agents", ".claude":
			return true
		case ".skills-seed":
			if index != 0 || (len(parts) > 1 && parts[1] != "context") {
				return true
			}
		}
	}
	return false
}
