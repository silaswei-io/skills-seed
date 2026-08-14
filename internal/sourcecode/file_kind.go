package sourcecode

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

// engineeringKnowledgeFiles 是工程约束、构建、验证和代码生成的权威入口文件。
var engineeringKnowledgeFiles = map[string]bool{
	"agents.md":       true,
	"claude.md":       true,
	"taskfile.yml":    true,
	"taskfile.yaml":   true,
	"makefile":        true,
	"justfile":        true,
	"jenkinsfile":     true,
	".gitlab-ci.yml":  true,
	".gitlab-ci.yaml": true,
	"build.sh":        true,
	"_build.sh":       true,
	"build.ps1":       true,
	"build.bat":       true,
	"buf.gen.yaml":    true,
	"buf.gen.yml":     true,
	"sqlc.yaml":       true,
	"sqlc.yml":        true,
}

// testPathSegments 是明确承载测试实现或测试资源的目录名。
var testPathSegments = map[string]bool{
	"test":      true,
	"tests":     true,
	"__tests__": true,
	"fixtures":  true,
	"mocks":     true,
	"e2e":       true,
}

// procedureRootSegments 是仓库根下明确只承载验收流程或用例的目录名。
var procedureRootSegments = map[string]bool{
	"acceptance": true,
	"uat":        true,
}

// operationRootSegments 可能同时包含操作脚本和声明式运行时配置，不能整目录排除。
var operationRootSegments = map[string]bool{
	"deploy":     true,
	"deployment": true,
	"release":    true,
}

// procedureFileNames 是明确承担测试、部署或持续交付流程的文件名。
var procedureFileNames = map[string]bool{
	"jenkinsfile":     true,
	".gitlab-ci.yml":  true,
	".gitlab-ci.yaml": true,
	"deploy.sh":       true,
	"deploy.ps1":      true,
	"release.sh":      true,
	"release.ps1":     true,
	"acceptance.sh":   true,
	"acceptance.ps1":  true,
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

// IsProcedureSource 判断路径是否只承担测试、部署或验收流程。
func IsProcedureSource(path string) bool {
	normalized := strings.ToLower(strings.TrimSpace(filepath.ToSlash(path)))
	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "" {
		return false
	}
	parts := strings.Split(normalized, "/")
	if len(parts) >= 2 && parts[0] == ".github" && parts[1] == "workflows" {
		return true
	}
	for _, part := range parts[:len(parts)-1] {
		if testPathSegments[part] {
			return true
		}
	}
	if procedureRootSegments[parts[0]] {
		return true
	}
	base := parts[len(parts)-1]
	if operationRootSegments[parts[0]] && !isDeclarativeConfig(base) {
		return true
	}
	if procedureFileNames[base] || isTestSourceFile(base) {
		return true
	}
	return false
}

func isDeclarativeConfig(base string) bool {
	switch strings.ToLower(filepath.Ext(base)) {
	case ".yaml", ".yml", ".json", ".toml", ".xml", ".ini", ".conf", ".properties", ".env":
		return true
	default:
		return false
	}
}

func isTestSourceFile(base string) bool {
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".test.jsx") ||
		strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".spec.js") || strings.HasSuffix(base, ".spec.jsx") ||
		strings.HasSuffix(base, ".spec.ts") || strings.HasSuffix(base, ".spec.tsx") ||
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") ||
		strings.HasSuffix(base, "_test.py")
}

// IsEngineeringKnowledge 判断文件是否属于独立采集的权威工程知识源。
func IsEngineeringKnowledge(path string) bool {
	normalized := strings.ToLower(strings.TrimSpace(filepath.ToSlash(path)))
	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "" {
		return false
	}
	if engineeringKnowledgeFiles[filepath.Base(normalized)] {
		return true
	}
	isWorkflow := strings.HasPrefix(normalized, ".github/workflows/") &&
		(strings.HasSuffix(normalized, ".yml") || strings.HasSuffix(normalized, ".yaml"))
	return isWorkflow || IsUserRuleAuthority(normalized)
}

// IsUserRuleAuthority 判断路径是否精确指向用户维护的 Rule 原文。
func IsUserRuleAuthority(path string) bool {
	parts := strings.Split(strings.ToLower(strings.Trim(filepath.ToSlash(path), "/")), "/")
	return len(parts) == 4 && parts[0] == ".skills-seed" && parts[1] == "rules" && parts[3] == "rule.md"
}
