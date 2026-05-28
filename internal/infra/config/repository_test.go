package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// setupTestConfig 创建测试用临时配置仓储。
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

		// 确认配置文件初始不存在。
		_, err := os.Stat(configPath)
		require.True(t, os.IsNotExist(err))

		repo, err := NewRepository(seedPath, "zh-CN")
		require.NoError(t, err)
		require.NotNil(t, repo)

		// 确认配置文件已创建。
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
		// 使用不可写路径触发错误。
		repo, err := NewRepository("/proc/nonexistent/path/that/cannot/be/created", "zh-CN")
		assert.Error(t, err)
		assert.Nil(t, repo)
	})
}

func TestRepository_Get(t *testing.T) {
	repo := setupTestConfig(t)
	cfg := repo.Get()

	require.NotNil(t, cfg, "Get() should return non-nil config")

	// 校验来自嵌入模板或硬编码 fallback 的默认值。
	assert.Equal(t, "go", cfg.Project.Language)
	assert.Equal(t, "zh-CN", cfg.Project.Locale)
	assert.Equal(t, "claude", cfg.Agent.Provider)
	assert.Equal(t, "claude", cfg.Agent.Commands["claude"])
	assert.Equal(t, "codex", cfg.Agent.Commands["codex"])
	assert.Equal(t, 1800, cfg.Agent.Timeout)
	assert.False(t, cfg.Agent.AllowUserPlugins)
	assert.True(t, cfg.Analysis.CodeGraph.Enabled)
	assert.False(t, cfg.Analysis.CodeGraph.Required)
	assert.Equal(t, "codegraph", cfg.Analysis.CodeGraph.Command)
	assert.True(t, cfg.Analysis.CodeGraph.AutoInit)
	assert.True(t, cfg.Analysis.CodeGraph.AutoSync)
	assert.Equal(t, 30, cfg.Analysis.CodeGraph.MaxNodes)
	assert.Equal(t, 0, cfg.Analysis.CodeGraph.MaxCode)
	assert.Equal(t, 50, cfg.Learning.MaxCommits)
	assert.Equal(t, "patch", cfg.AutoFix.Strategy)
	assert.Equal(t, ".claude/skills/skills-seed-skills", cfg.Output.SkillsPaths["claude"])
	assert.Equal(t, ".agents/skills/skills-seed-skills", cfg.Output.SkillsPaths["codex"])
	assert.False(t, cfg.Workspace.InitChildren)
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

func TestRepository_UpdatePersistsWorkspaceConfig(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := repo.Get()
	cfg.Project.Mode = "workspace"
	cfg.Workspace.InitChildren = true
	cfg.Workspace.Projects = []WorkspaceProjectConfig{
		{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
	}
	cfg.Workspace.Contracts = []WorkspacePathConfig{{Path: "proto", Description: "API contracts"}}
	require.NoError(t, repo.Update(cfg))

	content, err := os.ReadFile(filepath.Join(seedPath, "config.yaml"))
	require.NoError(t, err)
	contentText := string(content)
	require.Contains(t, contentText, "# 工作区")
	require.NotContains(t, contentText, `child_skill_policy`)
	require.Contains(t, contentText, `init_children: true`)
	require.Contains(t, contentText, `id: "frontend"`)
	require.Contains(t, contentText, `path: "proto"`)
	require.Contains(t, contentText, `description: "API contracts"`)
	require.Contains(t, contentText, `shared: []`)
	require.NotContains(t, contentText, `generation:`)
	require.NotContains(t, contentText, `'**/*.pb.go'`)

	reloaded, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	require.True(t, reloaded.GetWorkspaceConfig().InitChildren)
	require.Len(t, reloaded.GetWorkspaceConfig().Projects, 2)
	require.Equal(t, "backend", reloaded.GetWorkspaceConfig().Projects[1].ID)
	require.Equal(t, "API contracts", reloaded.GetWorkspaceConfig().Contracts[0].Description)
}

func TestRepository_RenderWorkspaceConfigPreservesTemplateStyle(t *testing.T) {
	templateData, err := embedfs.FS.ReadFile("templates/config/config.yaml.zh-CN.tmpl")
	require.NoError(t, err)

	repo := &Repository{}
	cfg := &Config{
		Project: ProjectConfig{
			Name:          "demo-workspace",
			Mode:          "workspace",
			Language:      "go",
			Locale:        "zh-CN",
			InitializedAt: "2026-05-26 12:00:00",
		},
		Workspace: WorkspaceConfig{
			InitChildren: true,
			Projects: []WorkspaceProjectConfig{
				{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
			},
			Contracts: []WorkspacePathConfig{{Path: "proto", Description: "API contracts"}},
		},
		Agent: AgentConfig{
			Provider: "claude",
			Commands: map[string]string{
				"claude": "claude",
				"codex":  "codex",
			},
			Timeout: 1800,
		},
		Learning: LearningConfig{MaxCommits: 50, BatchSize: 5},
		AutoFix:  AutoFixConfig{Strategy: "patch", BackupPath: "backups"},
		Output: OutputConfig{SkillsPaths: map[string]string{
			"claude": ".claude/skills/skills-seed-skills",
			"codex":  ".agents/skills/skills-seed-skills",
		}},
		Logging: LoggingConfig{Level: "DEBUG", LogsPath: "logs", MaxLogFiles: 30},
	}

	content := repo.replaceConfigValues(string(templateData), cfg)
	var parsed Config
	require.NoError(t, yaml.Unmarshal([]byte(content), &parsed), content)
	require.Contains(t, content, "# 工作区")
	require.NotContains(t, content, `child_skill_policy`)
	require.Contains(t, content, `init_children: true`)
	require.Contains(t, content, `id: "backend"`)
	require.Contains(t, content, `description: "API contracts"`)
	require.Contains(t, content, `- "**/*.pb.go"`)
	require.Contains(t, content, `- ".*"`)
	require.Contains(t, content, `analysis:`)
	require.Contains(t, content, `enabled: false`)
	require.NotContains(t, content, `generation:`)
}

func TestRepository_NormalizeAnalysisCodeGraphDefaults(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
project:
  language: "go"
  locale: "zh-CN"
agent:
  provider: "claude"
learning:
  max_commits: 50
autofix:
  strategy: "patch"
output:
  skills_paths: {}
logging:
  level: "DEBUG"
exclude: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := repo.GetAnalysisConfig().CodeGraph
	require.True(t, cfg.Enabled)
	require.False(t, cfg.Required)
	require.Equal(t, "codegraph", cfg.Command)
	require.True(t, cfg.AutoInit)
	require.True(t, cfg.AutoSync)
	require.Equal(t, 30, cfg.MaxNodes)
	require.Equal(t, 0, cfg.MaxCode)
}

func TestRepository_PreservesExplicitCodeGraphDisabled(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
project:
  language: "go"
  locale: "zh-CN"
analysis:
  codegraph:
    enabled: false
    required: false
    command: "custom-codegraph"
    auto_init: true
    auto_sync: false
    max_nodes: 12
    max_code: 3
agent:
  provider: "claude"
learning:
  max_commits: 50
autofix:
  strategy: "patch"
output:
  skills_paths: {}
logging:
  level: "DEBUG"
exclude: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := repo.GetAnalysisConfig().CodeGraph
	require.False(t, cfg.Enabled)
	require.False(t, cfg.Required)
	require.Equal(t, "custom-codegraph", cfg.Command)
	require.True(t, cfg.AutoInit)
	require.False(t, cfg.AutoSync)
	require.Equal(t, 12, cfg.MaxNodes)
	require.Equal(t, 3, cfg.MaxCode)
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

	// 校验内存中的值。
	assert.Equal(t, "my-test-project", repo.Get().Project.Name)

	// 重新读取磁盘，校验已持久化。
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

	// 校验内存中的值。
	assert.Equal(t, "en-US", repo.Get().Project.Locale)

	// 重新读取磁盘，校验已持久化。
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

	// 校验内存中的值。
	assert.Equal(t, "backup", repo.Get().AutoFix.Strategy)

	// 重新读取磁盘，校验已持久化。
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

func TestDefaultSkillsPathForProvider(t *testing.T) {
	assert.Equal(t, ".claude/skills/skills-seed-skills", DefaultSkillsPathForProvider("claude"))
	assert.Equal(t, ".agents/skills/skills-seed-skills", DefaultSkillsPathForProvider("codex"))
	assert.Equal(t, ".skills/skills-seed-skills", DefaultSkillsPathForProvider("custom"))
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
	assert.Contains(t, exclude, ".*")
	assert.Contains(t, exclude, "vendor/**")
	assert.Contains(t, exclude, "node_modules/**")
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
	// 传入空 locale 时，应使用检测到的系统语言或默认语言。
	repo, err := NewRepository(seedPath, "")
	require.NoError(t, err)
	require.NotNil(t, repo)

	cfg := repo.Get()
	// locale 应该被设置为非空值。
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
