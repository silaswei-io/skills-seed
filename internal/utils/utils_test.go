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
		// macOS /var is symlink to /private/var, resolve both
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

func TestLoadConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configContent := []byte(`
project:
  name: "test-project"
  language: "go"
  locale: "zh-CN"
agent:
  provider: "claude"
  commands:
    claude: "claude"
  timeout: 300
learning:
  max_commits: 50
  batch_size: 5
output:
  skills_paths:
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

func TestRelativePaths(t *testing.T) {
	projectRoot := filepath.Join("tmp", "project")
	paths := []string{
		filepath.Join(projectRoot, "internal", "service"),
		filepath.Join(projectRoot, "cmd", "skills-seed"),
	}

	result := RelativePaths(projectRoot, paths)

	assert.Equal(t, []string{"internal/service", "cmd/skills-seed"}, result)
}
