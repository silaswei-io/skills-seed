package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestConfig creates a temporary config repository for testing.
func setupTestConfig(t *testing.T) *Repository {
	t.Helper()
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	return repo
}

func TestNewRepository(t *testing.T) {
	t.Run("creates config directory and file", func(t *testing.T) {
		seedPath := t.TempDir()
		configPath := filepath.Join(seedPath, "config.yaml")

		// Verify config file does not exist before
		_, err := os.Stat(configPath)
		require.True(t, os.IsNotExist(err))

		repo, err := NewRepository(seedPath, "zh-CN")
		require.NoError(t, err)
		require.NotNil(t, repo)

		// Verify config file was created
		_, err = os.Stat(configPath)
		assert.NoError(t, err, "config file should be created")
	})

	t.Run("generated config omits legacy agent and output fields", func(t *testing.T) {
		seedPath := t.TempDir()
		configPath := filepath.Join(seedPath, "config.yaml")

		_, err := NewRepository(seedPath, "zh-CN")
		require.NoError(t, err)

		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		assert.NotContains(t, string(content), "\n  command:")
		assert.NotContains(t, string(content), "claude_command:")
		assert.NotContains(t, string(content), "codex_command:")
		assert.NotContains(t, string(content), "skills_path:")
		assert.Contains(t, string(content), "commands:")
		assert.Contains(t, string(content), "skills_paths:")
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		// Use a path that cannot be written to
		repo, err := NewRepository("/proc/nonexistent/path/that/cannot/be/created", "zh-CN")
		assert.Error(t, err)
		assert.Nil(t, repo)
	})
}

func TestRepository_Get(t *testing.T) {
	repo := setupTestConfig(t)
	cfg := repo.Get()

	require.NotNil(t, cfg, "Get() should return non-nil config")

	// Verify default values from embedded template or hardcoded defaults
	assert.Equal(t, "go", cfg.Project.Language)
	assert.Equal(t, "zh-CN", cfg.Project.Locale)
	assert.Equal(t, "claude", cfg.Agent.Provider)
	assert.Equal(t, "claude", cfg.Agent.Commands["claude"])
	assert.Equal(t, "codex", cfg.Agent.Commands["codex"])
	assert.Equal(t, 1800, cfg.Agent.Timeout)
	assert.False(t, cfg.Agent.AllowUserPlugins)
	assert.Equal(t, 50, cfg.Learning.MaxCommits)
	assert.Equal(t, "patch", cfg.AutoFix.Strategy)
	assert.Equal(t, ".claude/skills/skills-seed-skills", cfg.Output.SkillsPaths["claude"])
	assert.Equal(t, ".agents/skills/skills-seed-skills", cfg.Output.SkillsPaths["codex"])
	assert.Equal(t, "DEBUG", cfg.Logging.Level)
	assert.Equal(t, "logs", cfg.Logging.LogsPath)
	assert.NotEmpty(t, cfg.Project.InitializedAt, "initialized_at should be set")
}

func TestRepository_GetProjectConfig(t *testing.T) {
	repo := setupTestConfig(t)
	projectCfg := repo.GetProjectConfig()

	assert.Equal(t, "go", projectCfg.Language)
	assert.Equal(t, "zh-CN", projectCfg.Locale)
	assert.NotEmpty(t, projectCfg.InitializedAt)
}

func TestRepository_GetAgentConfig(t *testing.T) {
	repo := setupTestConfig(t)
	agentCfg := repo.GetAgentConfig()

	assert.Equal(t, "claude", agentCfg.Provider)
	assert.Equal(t, "claude", agentCfg.Commands["claude"])
	assert.Equal(t, "codex", agentCfg.Commands["codex"])
	assert.Equal(t, 1800, agentCfg.Timeout)
	assert.False(t, agentCfg.AllowUserPlugins)
}

func TestEffectiveSkillsPath(t *testing.T) {
	output := OutputConfig{
		SkillsPaths: map[string]string{
			"alpha": "alpha/skills",
			"beta":  "beta/skills",
		},
	}

	assert.Equal(t, "alpha/skills", EffectiveSkillsPath("alpha", output))
	assert.Equal(t, "beta/skills", EffectiveSkillsPath("beta", output))
	assert.Equal(t, "", EffectiveSkillsPath("gamma", output))
	assert.Equal(t, "", EffectiveSkillsPath("", output))
}

func TestRepository_GetLearningConfig(t *testing.T) {
	repo := setupTestConfig(t)
	learningCfg := repo.GetLearningConfig()

	assert.Equal(t, 50, learningCfg.MaxCommits)
	assert.Equal(t, 5, learningCfg.BatchSize)
}

func TestRepository_SetProjectName(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	err = repo.SetProjectName("my-test-project")
	require.NoError(t, err)

	// Verify in-memory value
	assert.Equal(t, "my-test-project", repo.Get().Project.Name)

	// Verify persisted by re-reading from disk
	repo2, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	assert.Equal(t, "my-test-project", repo2.Get().Project.Name)
}

func TestRepository_SetLocale(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	err = repo.SetLocale("en-US")
	require.NoError(t, err)

	// Verify in-memory value
	assert.Equal(t, "en-US", repo.Get().Project.Locale)

	// Verify persisted by re-reading from disk
	repo2, err := NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	assert.Equal(t, "en-US", repo2.Get().Project.Locale)
}

func TestRepository_SetAutoFixStrategy(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	err = repo.SetAutoFixStrategy("backup")
	require.NoError(t, err)

	// Verify in-memory value
	assert.Equal(t, "backup", repo.Get().AutoFix.Strategy)

	// Verify persisted by re-reading from disk
	repo2, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	assert.Equal(t, "backup", repo2.Get().AutoFix.Strategy)
}

func TestRepository_GetAutoFixConfig(t *testing.T) {
	repo := setupTestConfig(t)
	autoFixCfg := repo.GetAutoFixConfig()

	assert.Equal(t, "patch", autoFixCfg.Strategy)
	assert.Equal(t, "backups", autoFixCfg.BackupPath)
}

func TestRepository_GetOutputConfig(t *testing.T) {
	repo := setupTestConfig(t)
	outputCfg := repo.GetOutputConfig()

	assert.NotEmpty(t, outputCfg.SkillsPaths)
}

func TestRepository_GetLoggingConfig(t *testing.T) {
	repo := setupTestConfig(t)
	loggingCfg := repo.GetLoggingConfig()

	assert.Equal(t, "DEBUG", loggingCfg.Level)
	assert.Equal(t, "logs", loggingCfg.LogsPath)
	assert.Equal(t, 30, loggingCfg.MaxLogFiles)
}

func TestRepository_GetExclude(t *testing.T) {
	repo := setupTestConfig(t)
	exclude := repo.GetExclude()

	assert.NotEmpty(t, exclude, "exclude list should not be empty")
	// Verify common exclusions are present
	found := false
	for _, pattern := range exclude {
		if pattern == "vendor/**" {
			found = true
			break
		}
	}
	assert.True(t, found, "exclude list should contain vendor/**")
}

func TestRepository_Update(t *testing.T) {
	repo := setupTestConfig(t)

	cfg := repo.Get()
	cfg.Agent.Timeout = 3600
	cfg.Learning.MaxCommits = 100

	err := repo.Update(cfg)
	require.NoError(t, err)

	updated := repo.Get()
	assert.Equal(t, 3600, updated.Agent.Timeout)
	assert.Equal(t, 100, updated.Learning.MaxCommits)
}

func TestRepository_SetProjectLanguage(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	err = repo.SetProjectLanguage("python")
	require.NoError(t, err)

	assert.Equal(t, "python", repo.Get().Project.Language)
}

func TestRepository_SetGitRemote(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	err = repo.SetGitRemote("https://github.com/test/repo.git")
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/test/repo.git", repo.Get().Project.GitRemote)
}

func TestRepository_SetRootPath(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	err = repo.SetRootPath("/home/user/project")
	require.NoError(t, err)

	assert.Equal(t, "/home/user/project", repo.Get().Project.RootPath)
}

func TestNewRepository_DefaultLocale(t *testing.T) {
	seedPath := t.TempDir()
	// Pass empty locale - should use detected system locale or default
	repo, err := NewRepository(seedPath, "")
	require.NoError(t, err)
	require.NotNil(t, repo)

	cfg := repo.Get()
	// Locale should be set to something (not empty)
	assert.NotEmpty(t, cfg.Project.Locale)
}

func TestNewRepository_EnUSLocale(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	require.NotNil(t, repo)

	cfg := repo.Get()
	assert.Equal(t, "en-US", cfg.Project.Locale)
}
