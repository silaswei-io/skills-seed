package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverProjectsUsesOnlyFirstLevelDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "backend", "go.mod"), "module example.com/backend\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend", "internal"), 0755))
	writeFile(t, filepath.Join(root, "nested", "service", "go.mod"), "module example.com/service\n")
	writeFile(t, filepath.Join(root, "frontend", "package.json"), `{"dependencies":{"react":"latest"}}`)
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
	require.Equal(t, "typescript", projects[1].Language)
}

func TestDiscoverProjectsSupportsAdditionalMarkers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "dotnet", "service.csproj"), "<Project />\n")
	writeFile(t, filepath.Join(root, "infra", "Chart.yaml"), "apiVersion: v2\n")
	writeFile(t, filepath.Join(root, "php", "composer.json"), "{}\n")
	writeFile(t, filepath.Join(root, "rust", "Cargo.toml"), "[package]\n")

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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
