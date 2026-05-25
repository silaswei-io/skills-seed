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

// DiscoverProjects 从仓库根目录自动发现工作区子项目候选
func DiscoverProjects(root string) []config.WorkspaceProjectConfig {
	candidates := map[string]config.WorkspaceProjectConfig{}

	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			return nil
		}

		name := entry.Name()
		if ignoredDirs[name] {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		if strings.Count(rel, string(filepath.Separator)) > 2 {
			return filepath.SkipDir
		}

		project, ok := detectProject(root, rel)
		if !ok {
			return nil
		}
		candidates[project.Path] = project
		return filepath.SkipDir
	})

	projects := make([]config.WorkspaceProjectConfig, 0, len(candidates))
	for _, project := range candidates {
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})
	return projects
}

func detectProject(root, rel string) (config.WorkspaceProjectConfig, bool) {
	abs := filepath.Join(root, rel)
	markers := map[string]bool{}
	for _, marker := range []string{
		"package.json",
		"go.mod",
		"pyproject.toml",
		"requirements.txt",
		"Cargo.toml",
		"pom.xml",
		"build.gradle",
		"Dockerfile",
		"buf.yaml",
		"openapi.yaml",
		"openapi.yml",
	} {
		if _, err := os.Stat(filepath.Join(abs, marker)); err == nil {
			markers[marker] = true
		}
	}
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

func inferTypeAndLanguage(abs string, markers map[string]bool) (string, string) {
	if markers["package.json"] {
		if isFrontendPackage(filepath.Join(abs, "package.json")) {
			return "frontend", "typescript"
		}
		return "backend", "typescript"
	}
	if markers["go.mod"] {
		if pathExists(filepath.Join(abs, "cmd")) || pathExists(filepath.Join(abs, "internal")) {
			return "backend", "go"
		}
		return "library", "go"
	}
	if markers["pyproject.toml"] || markers["requirements.txt"] {
		return "backend", "python"
	}
	if markers["buf.yaml"] || markers["openapi.yaml"] || markers["openapi.yml"] {
		return "contracts", "unknown"
	}
	if markers["Dockerfile"] {
		return "service", "unknown"
	}
	return "project", "unknown"
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
