package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
)

// EngineeringKnowledgeRevision 返回项目级权威工程知识的内容版本。
// 空项目不产生版本，避免没有权威来源的已有画像发生无意义刷新。
func EngineeringKnowledgeRevision(projectRoot string) (string, error) {
	paths, err := engineeringKnowledgePaths(projectRoot)
	if err != nil {
		return "", err
	}
	return engineeringKnowledgeRevisionForPaths(projectRoot, paths)
}

// engineeringKnowledgeRevisionForPaths 对本次实际送入画像同步的权威文件求内容版本。
func engineeringKnowledgeRevisionForPaths(projectRoot string, paths []string) (string, error) {
	paths, err := cleanEngineeringKnowledgePaths(paths)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}

	hash := sha256.New()
	for _, path := range paths {
		resolved, err := projectpath.CanonicalWithinRoot(projectRoot, filepath.Join(projectRoot, filepath.FromSlash(path)))
		if err != nil {
			return "", fmt.Errorf("resolve authority source %q: %w", path, err)
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return "", fmt.Errorf("read authority source %q: %w", path, err)
		}
		hash.Write([]byte(path))
		hash.Write([]byte{0})
		hash.Write(data)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func cleanEngineeringKnowledgePaths(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path, ok := projectRelativePath(path)
		if !ok {
			return nil, fmt.Errorf("invalid authority path %q", path)
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

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
		if isProjectEngineeringAuthority(relative) {
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

func isProjectEngineeringAuthority(path string) bool {
	path = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(path)), "./")
	return path != "" && sourcecode.IsInstructionAuthority(path)
}

func skipEngineeringKnowledgeDir(path string) bool {
	path = strings.Trim(filepath.ToSlash(path), "/")
	parts := strings.Split(path, "/")
	for index, part := range parts {
		switch part {
		case ".git", "vendor", "node_modules", ".agents", ".claude":
			return true
		case ".skills-seed":
			if index != 0 || (len(parts) > 1 && parts[1] != "rules") {
				return true
			}
		}
	}
	return false
}
