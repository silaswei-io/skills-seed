package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSpecFromProfileUsesRequestedLocaleForFallbackRules(t *testing.T) {
	profile := &domain.WorkspaceProfile{
		Name: "demo",
		Projects: []domain.WorkspaceProject{
			{ID: "backend", Path: "backend"},
		},
		Contracts: []domain.WorkspacePath{{Path: "contracts"}},
		Shared:    []domain.WorkspacePath{{Path: "shared"}},
		Infra:     []domain.WorkspacePath{{Path: "infra"}},
	}

	spec := SpecFromProfile(profile, "en-US")

	require.Len(t, spec.Rules, 1)
	require.Equal(t, "Define cross-project boundaries first", spec.Rules[0].Title)
	for _, route := range spec.Routing {
		require.NotRegexp(t, `[\p{Han}]`, route.Reason)
	}
}

func TestDiscoverProjectsUsesOnlyFirstLevelGitRepositories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "backend", "go.mod"), "module example.com/backend\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend", ".git"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend", "internal"), 0755))
	writeFile(t, filepath.Join(root, "nested", "service", "go.mod"), "module example.com/service\n")
	writeFile(t, filepath.Join(root, "frontend", "package.json"), `{"dependencies":{"react":"latest"}}`)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "frontend", ".git"), 0755))
	writeFile(t, filepath.Join(root, "not-a-repo", "go.mod"), "module example.com/not-a-repo\n")
	writeFile(t, filepath.Join(root, "README.md"), "# workspace\n")

	projects := DiscoverProjects(root)

	require.Len(t, projects, 2)
	require.Equal(t, "backend", projects[0].ID)
	require.Equal(t, "backend", projects[0].Path)
	require.Equal(t, "backend", projects[0].Type)
	require.Equal(t, "go", projects[0].Language)
	require.Equal(t, "frontend", projects[1].ID)
	require.Equal(t, "frontend", projects[1].Path)
	require.Equal(t, "frontend", projects[1].Type)
	require.Equal(t, "javascript", projects[1].Language)
}

func TestDiscoverProjectsClassifiesGitRepositoriesByMarkers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "dotnet", "service.csproj"), "<Project />\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dotnet", ".git"), 0755))
	writeFile(t, filepath.Join(root, "infra", "Chart.yaml"), "apiVersion: v2\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "infra", ".git"), 0755))
	writeFile(t, filepath.Join(root, "php", "composer.json"), "{}\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "php", ".git"), 0755))
	writeFile(t, filepath.Join(root, "rust", "Cargo.toml"), "[package]\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "rust", ".git"), 0755))

	projects := DiscoverProjects(root)

	require.Len(t, projects, 4)
	require.Equal(t, "dotnet", projects[0].ID)
	require.Equal(t, "backend", projects[0].Type)
	require.Equal(t, "csharp", projects[0].Language)
	require.Equal(t, "infra", projects[1].ID)
	require.Equal(t, "infra", projects[1].Type)
	require.Equal(t, "unknown", projects[1].Language)
	require.Equal(t, "php", projects[2].ID)
	require.Equal(t, "php", projects[2].Language)
	require.Equal(t, "rust", projects[3].ID)
	require.Equal(t, "rust", projects[3].Language)
}

func TestDiscoverProjectsIncludesShellGitRepositories(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "base-xengine", ".git"), 0755))
	writeFile(t, filepath.Join(root, "base-xengine", "install.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "base-xengine", "functions.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "base-xengine", "install.ini"), "[install]\n")

	projects := DiscoverProjects(root)

	require.Len(t, projects, 1)
	require.Equal(t, "base-xengine", projects[0].ID)
	require.Equal(t, "base-xengine", projects[0].Path)
	require.Equal(t, "infra", projects[0].Type)
	require.Equal(t, "shell", projects[0].Language)
}

func TestDetectProjectKindAndLanguageDistinguishesNodeLanguage(t *testing.T) {
	jsRoot := t.TempDir()
	writeFile(t, filepath.Join(jsRoot, "package.json"), `{"dependencies":{"react":"latest"}}`)

	projectType, language, ok := DetectProjectKindAndLanguage(jsRoot)
	require.True(t, ok)
	require.Equal(t, "frontend", projectType)
	require.Equal(t, "javascript", language)

	tsRoot := t.TempDir()
	writeFile(t, filepath.Join(tsRoot, "package.json"), `{"devDependencies":{"typescript":"latest","vite":"latest"}}`)
	writeFile(t, filepath.Join(tsRoot, "tsconfig.json"), "{}")

	projectType, language, ok = DetectProjectKindAndLanguage(tsRoot)
	require.True(t, ok)
	require.Equal(t, "frontend", projectType)
	require.Equal(t, "typescript", language)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
