package pathx

import (
	"path/filepath"
	"sort"
	"strings"
)

// CleanRelative 将输入规范为安全的项目相对路径。
func CleanRelative(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = filepath.ToSlash(path)
	if path == "" || path == "." || isAbsolutePath(path) {
		return ""
	}
	path = strings.TrimPrefix(path, "./")
	path = strings.Trim(path, "/")
	if path == "" || path == "." {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return ""
	}
	return clean
}

// CleanRelativeList 规范化、去重并排序项目相对路径列表。
func CleanRelativeList(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = CleanRelative(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// CleanRelativeSet 返回规范化后的项目相对路径集合。
func CleanRelativeSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = CleanRelative(path)
		if path != "" {
			set[path] = true
		}
	}
	return set
}

// CleanEvidenceLocationPath 从证据位置中提取项目相对路径，支持 path:line 格式。
func CleanEvidenceLocationPath(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "` ")
	if value == "" {
		return ""
	}
	path, _, _ := splitLineSuffix(value)
	return CleanRelative(path)
}

// CleanEvidenceLocation 规范化证据位置，保留有效的 path:line 行号。
func CleanEvidenceLocation(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "` ")
	if value == "" {
		return ""
	}
	path, line, hasLine := splitLineSuffix(value)
	path = CleanRelative(path)
	if path == "" {
		return ""
	}
	if hasLine {
		return path + ":" + line
	}
	return path
}

func splitLineSuffix(value string) (path, line string, hasLine bool) {
	idx := strings.LastIndex(value, ":")
	if idx <= 0 || idx == len(value)-1 {
		return value, "", false
	}
	for _, r := range value[idx+1:] {
		if r < '0' || r > '9' {
			return value, "", false
		}
	}
	return value[:idx], value[idx+1:], true
}

func isAbsolutePath(path string) bool {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return true
	}
	if len(path) >= 3 && path[1] == ':' && path[2] == '/' {
		c := path[0]
		return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
	}
	return false
}
