// Package repositoryscope 定义仓库知识采集的统一路径边界。
package repositoryscope

import (
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/infra/gitignore"
)

// PathClass 描述路径在仓库知识采集中的归属。
type PathClass string

const (
	// PathClassProject 表示项目当前工作树中的可采集路径。
	PathClassProject PathClass = "project"
	// PathClassExcluded 表示由范围策略或 Git ignore 排除的路径。
	PathClassExcluded PathClass = "excluded"
)

// Scope 是所有知识扫描器共享的仓库路径边界。
type Scope struct {
	excludePatterns []string
	gitIgnore       *gitignore.Matcher
}

// New 创建使用指定排除规则的仓库范围。
func New(excludePatterns []string) Scope {
	return Scope{excludePatterns: append([]string(nil), excludePatterns...)}
}

// NewWithGitIgnore 创建带 Git ignore 规则的仓库范围。
// 配置到范围的编译由 infra/config 完成，避免范围核心依赖配置仓储。
func NewWithGitIgnore(excludePatterns []string, matcher *gitignore.Matcher) Scope {
	return Scope{
		excludePatterns: append([]string(nil), excludePatterns...),
		gitIgnore:       matcher,
	}
}

// Classify 返回路径的知识采集归属。
func (s Scope) Classify(path string) PathClass {
	path = normalize(path)
	if path == "" {
		return PathClassExcluded
	}
	if matchExcluded(path, s.excludePatterns) || s.gitIgnore.Match(path) {
		return PathClassExcluded
	}
	return PathClassProject
}

// AllowsKnowledge 报告路径是否允许作为项目知识的证据或扫描对象。
func (s Scope) AllowsKnowledge(path string) bool {
	return s.Classify(path) == PathClassProject
}

func matchExcluded(filePath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(filePath, normalize(pattern)) {
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
		if ok, _ := filepath.Match(suffixPattern, filepath.Base(filePath)); ok {
			return true
		}
		return strings.HasSuffix(filePath, strings.TrimPrefix(suffixPattern, "*"))
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := filepath.Match(pattern, filepath.Base(filePath)); ok {
			return true
		}
	}
	ok, _ := filepath.Match(pattern, filePath)
	return ok
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
	value = strings.TrimSpace(filepath.ToSlash(value))
	value = strings.TrimPrefix(value, "./")
	return strings.Trim(value, "/")
}
