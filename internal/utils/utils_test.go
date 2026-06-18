package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSeedPath(t *testing.T) {
	t.Run("found in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		seedDir := filepath.Join(tmpDir, ".skills-seed")
		require.NoError(t, os.MkdirAll(seedDir, 0755))

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(origDir)

		result, err := GetSeedPath()
		assert.NoError(t, err)
		// macOS 中 /var 是 /private/var 的符号链接，这里同时解析两边。
		expectedResolved, _ := filepath.EvalSymlinks(seedDir)
		resultResolved, _ := filepath.EvalSymlinks(result)
		assert.Equal(t, expectedResolved, resultResolved)
	})

	t.Run("not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(origDir)

		_, err := GetSeedPath()
		assert.Error(t, err)
		require.NoError(t, i18n.Init("zh-CN"))
		assert.Equal(t, i18n.Get("ErrNotInitialized"), err.Error())
	})
}

func TestFindSeedPathFromStopsWhenParentDoesNotChange(t *testing.T) {
	parentCalls := 0
	parentOf := func(path string) string {
		parentCalls++
		switch path {
		case `C:\repo\child`:
			return `C:\repo`
		case `C:\repo`:
			return `C:\`
		case `C:\`:
			return `C:\`
		default:
			return path
		}
	}

	_, found := findSeedPathFrom(
		`C:\repo\child`,
		func(string) bool { return false },
		parentOf,
	)

	assert.False(t, found)
	assert.LessOrEqual(t, parentCalls, 3)
}

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configContent := []byte(`
profile:
  name: "test-project"
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
  commands:
    claude: "claude"
  timeout: 300
learning:
  max_commits: 50
  batch_size: 5
skills:
  target: "claude"
  paths:
    claude: ".claude/skills"
logging:
  level: "INFO"
`)
		configPath := filepath.Join(tmpDir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, configContent, 0644))

		cfg, err := LoadConfig(tmpDir)
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "test-project", cfg.Project.Name)
		assert.Equal(t, "go", cfg.Project.Language)
		assert.Equal(t, 300, cfg.Agent.Timeout)
	})

	t.Run("non-existent path", func(t *testing.T) {
		_, err := LoadConfig("/non/existent/path")
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("invalid: [yaml: content"), 0644))

		_, err := LoadConfig(tmpDir)
		assert.Error(t, err)
	})
}

func TestResolvePathExpandsWindowsStyleHomePrefix(t *testing.T) {
	projectRoot := t.TempDir()

	resolved, err := ResolvePath(projectRoot, `~\skills-seed`)

	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(resolved))
	assert.Contains(t, filepath.ToSlash(resolved), "/skills-seed")
	assert.NotContains(t, resolved, "~")
}

func TestResolveProjectOutputPathRejectsPathsOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	projectRoot := filepath.Join(parent, "repo")
	require.NoError(t, os.MkdirAll(projectRoot, 0755))

	inside, err := ResolveProjectOutputPath(projectRoot, ".agents/skills/demo")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(projectRoot, ".agents", "skills", "demo"), inside)

	_, err = ResolveProjectOutputPath(projectRoot, "../outside")
	require.Error(t, err)
	assert.Contains(t, err.Error(), i18n.GetWithParams("GenerateOutputPathOutsideProjectRoot", map[string]interface{}{
		"OutputPath":  "../outside",
		"ProjectRoot": projectRoot,
	}))

	outsidePath := filepath.Join(parent, "outside")
	_, err = ResolveProjectOutputPath(projectRoot, outsidePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), i18n.GetWithParams("GenerateOutputPathOutsideProjectRoot", map[string]interface{}{
		"OutputPath":  outsidePath,
		"ProjectRoot": projectRoot,
	}))
}

func TestRelativePaths(t *testing.T) {
	projectRoot := filepath.Join("tmp", "project")
	paths := []string{
		filepath.Join(projectRoot, "internal", "service"),
		filepath.Join(projectRoot, "cmd", "skills-seed"),
	}

	result := RelativePaths(projectRoot, paths)

	assert.Equal(t, []string{"internal/service", "cmd/skills-seed"}, result)
}
