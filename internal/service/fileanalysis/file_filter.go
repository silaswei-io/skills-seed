package fileanalysis

import (
	"path"
	"strings"
)

// matchExcluded 判断 filePath 是否命中任一配置的排除模式。
func matchExcluded(filePath string, patterns []string) bool {
	normalized := normalize(filePath)
	if normalized == "" {
		return false
	}
	for _, pattern := range patterns {
		if matchPattern(normalized, normalize(pattern)) {
			return true
		}
	}
	return false
}

func matchPattern(filePath, pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == ".*" {
		return hasDotPrefixedSegment(filePath)
	}
	if pattern == filePath {
		return true
	}

	if strings.HasSuffix(pattern, "/**") && !strings.HasPrefix(pattern, "**/") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return filePath == prefix || strings.HasPrefix(filePath, prefix+"/")
	}

	if strings.HasPrefix(pattern, "**/") {
		suffixPattern := strings.TrimPrefix(pattern, "**/")
		if strings.HasSuffix(suffixPattern, "/**") {
			dir := strings.TrimSuffix(suffixPattern, "/**")
			return filePath == dir || strings.HasPrefix(filePath, dir+"/") || strings.Contains(filePath, "/"+dir+"/")
		}
		if ok, _ := path.Match(suffixPattern, path.Base(filePath)); ok {
			return true
		}
		return strings.HasSuffix(filePath, strings.TrimPrefix(suffixPattern, "*"))
	}

	if !strings.Contains(pattern, "/") {
		if ok, _ := path.Match(pattern, path.Base(filePath)); ok {
			return true
		}
	}

	if ok, _ := path.Match(pattern, filePath); ok {
		return true
	}
	return false
}

func hasDotPrefixedSegment(filePath string) bool {
	for _, segment := range strings.Split(filePath, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func normalize(value string) string {
	value = strings.TrimSpace(filepathToSlash(value))
	value = strings.TrimPrefix(value, "./")
	return strings.Trim(value, "/")
}

func filepathToSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}
