package sourcefiles

import (
	"path/filepath"
	"strings"
)

// documentExtensions 是仅作为文档处理的常见扩展名，learn 当前代码时默认不按源码分析。
var documentExtensions = map[string]bool{
	".md":   true,
	".mdx":  true,
	".rst":  true,
	".txt":  true,
	".adoc": true,
}

// documentNamePrefixes 是无固定扩展名或多语言后缀文档的常见文件名前缀。
var documentNamePrefixes = []string{
	"readme",
	"changelog",
	"license",
	"security",
	"contributing",
}

// sourceExtensions 是可进入源码分析和结构化上下文的文件扩展名。
var sourceExtensions = map[string]bool{
	".go":      true,
	".js":      true,
	".jsx":     true,
	".ts":      true,
	".tsx":     true,
	".py":      true,
	".rs":      true,
	".java":    true,
	".kt":      true,
	".kts":     true,
	".scala":   true,
	".c":       true,
	".h":       true,
	".cc":      true,
	".cpp":     true,
	".cxx":     true,
	".hpp":     true,
	".cs":      true,
	".php":     true,
	".rb":      true,
	".swift":   true,
	".sql":     true,
	".proto":   true,
	".api":     true,
	".graphql": true,
	".gql":     true,
	".yaml":    true,
	".yml":     true,
	".toml":    true,
	".json":    true,
	".xml":     true,
}

// buildAndDependencyFiles 是无源码扩展名但会影响工程结构、依赖或构建方式的文件名。
var buildAndDependencyFiles = map[string]bool{
	"go.mod":              true,
	"go.sum":              true,
	"package.json":        true,
	"package-lock.json":   true,
	"pnpm-lock.yaml":      true,
	"yarn.lock":           true,
	"tsconfig.json":       true,
	"vite.config.ts":      true,
	"vite.config.js":      true,
	"webpack.config.js":   true,
	"pyproject.toml":      true,
	"requirements.txt":    true,
	"poetry.lock":         true,
	"cargo.toml":          true,
	"cargo.lock":          true,
	"pom.xml":             true,
	"build.gradle":        true,
	"build.gradle.kts":    true,
	"settings.gradle":     true,
	"settings.gradle.kts": true,
	"composer.json":       true,
	"composer.lock":       true,
	"gemfile":             true,
	"gemfile.lock":        true,
	"dockerfile":          true,
	"docker-compose.yml":  true,
	"docker-compose.yaml": true,
	"makefile":            true,
	"cmakelists.txt":      true,
}

func IsDocument(path string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(filepath.ToSlash(path))))
	if base == "" || base == "." {
		return false
	}
	if documentExtensions[strings.ToLower(filepath.Ext(base))] {
		return true
	}
	for _, prefix := range documentNamePrefixes {
		if base == prefix || strings.HasPrefix(base, prefix+".") || strings.HasPrefix(base, prefix+"-") || strings.HasPrefix(base, prefix+"_") {
			return true
		}
	}
	return false
}

func IsAnalyzable(path string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(filepath.ToSlash(path))))
	if base == "" || base == "." {
		return false
	}
	if buildAndDependencyFiles[base] {
		return true
	}
	if sourceExtensions[strings.ToLower(filepath.Ext(base))] {
		return true
	}
	if IsDocument(path) {
		return false
	}
	return false
}
