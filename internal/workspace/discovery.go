package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

var ignoredDirs = map[string]bool{
	".git":         true,
	".skills-seed": true,
	".claude":      true,
	".agents":      true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"coverage":     true,
}

var projectMarkerFiles = []string{
	"package.json",
	"pnpm-workspace.yaml",
	"pnpm-lock.yaml",
	"yarn.lock",
	"package-lock.json",
	"bun.lock",
	"bun.lockb",
	"deno.json",
	"deno.jsonc",
	"tsconfig.json",
	"vite.config.ts",
	"vite.config.js",
	"next.config.js",
	"nuxt.config.ts",
	"go.mod",
	"go.work",
	"pyproject.toml",
	"requirements.txt",
	"setup.py",
	"setup.cfg",
	"Pipfile",
	"poetry.lock",
	"uv.lock",
	"Cargo.toml",
	"pom.xml",
	"mvnw",
	"build.gradle",
	"build.gradle.kts",
	"settings.gradle",
	"settings.gradle.kts",
	"gradlew",
	"build.sbt",
	"composer.json",
	"Gemfile",
	"mix.exs",
	"CMakeLists.txt",
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
	"buf.yaml",
	"buf.work.yaml",
	"openapi.yaml",
	"openapi.yml",
	"Chart.yaml",
	"kustomization.yaml",
	"main.tf",
}

var projectMarkerExtensions = []string{
	".csproj",
	".fsproj",
	".vbproj",
	".sln",
}

var markerGroups = map[string][]string{
	"node": {
		"package.json",
		"pnpm-workspace.yaml",
		"pnpm-lock.yaml",
		"yarn.lock",
		"package-lock.json",
		"bun.lock",
		"bun.lockb",
		"deno.json",
		"deno.jsonc",
		"tsconfig.json",
		"vite.config.ts",
		"vite.config.js",
		"next.config.js",
		"nuxt.config.ts",
	},
	"go":        {"go.mod", "go.work"},
	"python":    {"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg", "Pipfile", "poetry.lock", "uv.lock"},
	"java":      {"pom.xml", "mvnw", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "gradlew"},
	"dotnet":    {".csproj", ".fsproj", ".vbproj", ".sln"},
	"contracts": {"buf.yaml", "buf.work.yaml", "openapi.yaml", "openapi.yml"},
	"infra":     {"Chart.yaml", "kustomization.yaml", "main.tf"},
	"container": {"Dockerfile", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"},
}

// DiscoverProjects 从工作区根目录的第一层目录自动发现子项目候选
func DiscoverProjects(root string) []config.WorkspaceProjectConfig {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	projects := make([]config.WorkspaceProjectConfig, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if ignoredDirs[name] {
			continue
		}

		project, ok := detectProject(root, name)
		if !ok {
			continue
		}
		projects = append(projects, project)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})
	return projects
}

func detectProject(root, rel string) (config.WorkspaceProjectConfig, bool) {
	abs := filepath.Join(root, rel)
	markers := collectProjectMarkers(abs)
	if len(markers) == 0 {
		return config.WorkspaceProjectConfig{}, false
	}

	projectType, language := inferTypeAndLanguage(abs, markers)
	id := sanitizeID(filepath.Base(rel))
	if id == "" {
		id = "project"
	}
	return config.WorkspaceProjectConfig{
		ID:       id,
		Path:     filepath.ToSlash(rel),
		Type:     projectType,
		Language: language,
	}, true
}

func DetectProjectKindAndLanguage(root string) (string, string, bool) {
	markers := collectProjectMarkers(root)
	if len(markers) == 0 {
		return "", "", false
	}
	projectType, language := inferTypeAndLanguage(root, markers)
	return projectType, language, true
}

func collectProjectMarkers(abs string) map[string]bool {
	markers := map[string]bool{}
	for _, marker := range projectMarkerFiles {
		if pathExists(filepath.Join(abs, marker)) {
			markers[marker] = true
		}
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return markers
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		for _, ext := range projectMarkerExtensions {
			if strings.HasSuffix(name, ext) {
				markers[ext] = true
			}
		}
	}
	return markers
}

func inferTypeAndLanguage(abs string, markers map[string]bool) (string, string) {
	if hasAnyMarker(markers, markerGroups["node"]) {
		if isFrontendPackage(filepath.Join(abs, "package.json")) {
			return "frontend", inferNodeLanguage(abs, markers)
		}
		return "backend", inferNodeLanguage(abs, markers)
	}
	if hasAnyMarker(markers, markerGroups["go"]) {
		if pathExists(filepath.Join(abs, "cmd")) || pathExists(filepath.Join(abs, "internal")) {
			return "backend", "go"
		}
		return "library", "go"
	}
	if hasAnyMarker(markers, markerGroups["python"]) {
		return "backend", "python"
	}
	if markers["Cargo.toml"] {
		return "service", "rust"
	}
	if hasAnyMarker(markers, markerGroups["java"]) {
		return "backend", "java"
	}
	if markers["build.sbt"] {
		return "backend", "scala"
	}
	if markers["composer.json"] {
		return "backend", "php"
	}
	if markers["Gemfile"] {
		return "backend", "ruby"
	}
	if markers["mix.exs"] {
		return "backend", "elixir"
	}
	if markers["CMakeLists.txt"] {
		return "library", "cpp"
	}
	if hasAnyMarker(markers, markerGroups["dotnet"]) {
		return "backend", "csharp"
	}
	if hasAnyMarker(markers, markerGroups["contracts"]) {
		return "contracts", "unknown"
	}
	if hasAnyMarker(markers, markerGroups["infra"]) {
		return "infra", "unknown"
	}
	if hasAnyMarker(markers, markerGroups["container"]) {
		return "service", "unknown"
	}
	return "project", "unknown"
}

func inferNodeLanguage(abs string, markers map[string]bool) string {
	if markers["tsconfig.json"] || markers["vite.config.ts"] || markers["nuxt.config.ts"] {
		return "typescript"
	}
	data, err := os.ReadFile(filepath.Join(abs, "package.json"))
	if err != nil {
		return "javascript"
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "javascript"
	}
	for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
		for name := range deps {
			switch strings.ToLower(name) {
			case "typescript", "ts-node", "tsx":
				return "typescript"
			}
		}
	}
	return "javascript"
}

func hasAnyMarker(markers map[string]bool, names []string) bool {
	for _, name := range names {
		if markers[name] {
			return true
		}
	}
	return false
}

func isFrontendPackage(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}

	for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
		for name := range deps {
			switch strings.ToLower(name) {
			case "react", "vue", "@angular/core", "next", "nuxt", "vite", "svelte":
				return true
			}
		}
	}
	if _, ok := pkg.Scripts["dev"]; ok {
		return pathExists(filepath.Join(filepath.Dir(path), "src")) || pathExists(filepath.Join(filepath.Dir(path), "pages"))
	}
	return false
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sanitizeID(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	previousHyphen := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousHyphen = false
			continue
		}
		if !previousHyphen {
			b.WriteRune('-')
			previousHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// ProfileFromConfig 从配置构造工作区画像
func ProfileFromConfig(name, root string, cfg config.WorkspaceConfig) *domain.WorkspaceProfile {
	projects := make([]domain.WorkspaceProject, 0, len(cfg.Projects))
	for _, project := range cfg.Projects {
		projects = append(projects, domain.WorkspaceProject{
			ID:       project.ID,
			Path:     project.Path,
			Type:     project.Type,
			Language: project.Language,
		})
	}

	return &domain.WorkspaceProfile{
		Name:      name,
		RootPath:  root,
		Projects:  projects,
		Shared:    pathsFromConfig(cfg.Shared),
		Contracts: pathsFromConfig(cfg.Contracts),
		Infra:     pathsFromConfig(cfg.Infra),
	}
}

func pathsFromConfig(paths []config.WorkspacePathConfig) []domain.WorkspacePath {
	result := make([]domain.WorkspacePath, 0, len(paths))
	for _, path := range paths {
		result = append(result, domain.WorkspacePath{
			Path:        path.Path,
			Description: path.Description,
		})
	}
	return result
}
