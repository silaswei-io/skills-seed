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
		assert.NotContains(t, string(content), "provider:")
		assert.NotContains(t, string(content), "output:")
		assert.NotContains(t, string(content), "skills_provider:")
		assert.NotContains(t, string(content), "skills_paths:")
		assert.Contains(t, string(content), "engine:")
		assert.Contains(t, string(content), "commands:")
		assert.Contains(t, string(content), "skills:")
		assert.Contains(t, string(content), "target:")
		assert.Contains(t, string(content), "paths:")
	})

	t.Run("generated config uses preceding comments", func(t *testing.T) {
		seedPath := t.TempDir()
		configPath := filepath.Join(seedPath, "config.yaml")

		_, err := NewRepository(seedPath, "zh-CN")
		require.NoError(t, err)

		content, err := os.ReadFile(configPath)
		require.NoError(t, err)
		text := string(content)

		require.Contains(t, text, "# 项目名称(init 时自动填充)\n  name: \"\"")
		require.Contains(t, text, "# 子项目列表，例如: [{id: \"frontend\", path: \"frontend\", type: \"frontend\", language: \"typescript\"}]\n  projects: []")
		require.Contains(t, text, "# 默认启用 CodeGraph 结构化分析增强；未安装时会提醒并降级\n    enabled: true")
		require.Contains(t, text, "# glob 风格匹配（不是正则）；初始化时写入默认静态排除规则\nexclude:")
		require.NotContains(t, text, `name: ""                   #`)
		require.NotContains(t, text, `projects: []               #`)
		require.NotContains(t, text, `enabled: true             #`)
		require.NotContains(t, text, `exclude:                     #`)
		require.NotContains(t, text, `- ".*"                     #`)
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
	assert.Equal(t, "claude", cfg.Agent.Engine)
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
	assert.Equal(t, "claude", cfg.Skills.Target)
	assert.Equal(t, ".claude/skills/skills-seed-skills", cfg.Skills.Paths["claude"])
	assert.Equal(t, ".agents/skills/skills-seed-skills", cfg.Skills.Paths["codex"])
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
	require.NotContains(t, contentText, `init_children:`)
	require.Contains(t, contentText, `id: "frontend"`)
	require.Contains(t, contentText, `path: "proto"`)
	require.Contains(t, contentText, `description: "API contracts"`)
	require.Contains(t, contentText, `shared: []`)
	require.NotContains(t, contentText, `generation:`)
	require.NotContains(t, contentText, `'**/*.pb.go'`)

	reloaded, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
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
			Projects: []WorkspaceProjectConfig{
				{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
			},
			Contracts: []WorkspacePathConfig{{Path: "proto", Description: "API contracts"}},
		},
		Agent: AgentConfig{
			Engine: "claude",
			Commands: map[string]string{
				"claude": "claude",
				"codex":  "codex",
			},
			Timeout: 1800,
		},
		Learning: LearningConfig{MaxCommits: 50, BatchSize: 5},
		AutoFix:  AutoFixConfig{Strategy: "patch", BackupPath: "backups"},
		Skills: SkillsConfig{Target: "codex", Paths: map[string]string{
			"claude": ".claude/skills/skills-seed-skills",
			"codex":  ".agents/skills/skills-seed-skills",
		}},
		Logging: LoggingConfig{Level: "DEBUG", LogsPath: "logs", MaxLogFiles: 30},
		Exclude: []string{"dist/**", "*.log"},
	}

	content := repo.replaceConfigValues(string(templateData), cfg)
	var parsed Config
	require.NoError(t, yaml.Unmarshal([]byte(content), &parsed), content)
	require.Contains(t, content, "# 工作区")
	require.NotContains(t, content, `child_skill_policy`)
	require.NotContains(t, content, `init_children:`)
	require.Contains(t, content, `id: "backend"`)
	require.Contains(t, content, `description: "API contracts"`)
	require.NotContains(t, content, `- "**/*.pb.go"`)
	require.NotContains(t, content, `- "**/*.gen.go"`)
	require.Contains(t, content, `- "dist/**"`)
	require.Contains(t, content, `- "*.log"`)
	require.Contains(t, content, `analysis:`)
	require.Contains(t, content, `enabled: false`)
	require.Contains(t, content, "# 项目名称(init 时自动填充)\n  name: \"demo-workspace\"")
	require.Contains(t, content, "# 子项目列表，例如: [{id: \"frontend\", path: \"frontend\", type: \"frontend\", language: \"typescript\"}]\n  projects:")
	require.Contains(t, content, "# 默认启用 CodeGraph 结构化分析增强；未安装时会提醒并降级\n    enabled: false")
	require.Contains(t, content, "# glob 风格匹配（不是正则）；初始化时写入默认静态排除规则\nexclude:")
	require.NotContains(t, content, `name: "demo-workspace" #`)
	require.NotContains(t, content, `analysis: #`)
	require.NotContains(t, content, `enabled: false #`)
	require.NotContains(t, content, `exclude: #`)
	require.NotContains(t, content, `- "dist/**" #`)
	require.NotContains(t, content, `generation:`)
}

func TestRepository_UpdatePreservesExistingComments(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`# Custom project comment
project:
  name: "old-name" # custom project name comment
  mode: "project"
  language: "go"
  locale: "zh-CN"
  git_remote: ""
  root_path: ""
  initialized_at: "2026-05-26 12:00:00"

# Custom workspace comment
workspace:
  projects: [] # custom projects comment
  shared: []
  contracts: []
  infra: []

analysis:
  codegraph:
    enabled: true # custom codegraph comment
    required: false
    command: "codegraph"
    auto_init: true
    auto_sync: true
    max_nodes: 30
    max_code: 0

agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"
  timeout: 1800
  allow_user_plugins: false
  parallelism: 0

learning:
  max_commits: 50
  batch_size: 5

autofix:
  strategy: "patch"
  backup_path: "backups"

skills:
  target: "claude"
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "DEBUG"
  logs_path: "logs"
  max_log_files: 30

exclude:
  - ".*" # keep dotfiles comment
  - "*.log"
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := repo.Get()
	cfg.Project.Name = "new-name"
	cfg.Project.Mode = "workspace"
	cfg.Workspace.Projects = []WorkspaceProjectConfig{
		{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
	}
	cfg.Analysis.CodeGraph.Enabled = false
	cfg.Exclude = []string{".*", "dist/**"}
	require.NoError(t, repo.Update(cfg))

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "# Custom project comment")
	require.Contains(t, text, "# custom project name comment\n  name: \"new-name\"")
	require.Contains(t, text, "# Custom workspace comment")
	require.Contains(t, text, "# custom projects comment\n  projects:")
	require.Contains(t, text, "# custom codegraph comment\n    enabled: false")
	require.Contains(t, text, "# keep dotfiles comment\n  - \".*\"")
	require.NotContains(t, text, `name: "new-name" # custom project name comment`)
	require.NotContains(t, text, `projects: # custom projects comment`)
	require.NotContains(t, text, `analysis: # custom projects comment`)
	require.NotContains(t, text, `enabled: false # custom codegraph comment`)
	require.NotContains(t, text, `- ".*" # keep dotfiles comment`)

	reloaded, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "new-name", reloaded.GetProjectConfig().Name)
	require.Equal(t, "workspace", reloaded.GetProjectConfig().Mode)
	require.False(t, reloaded.GetAnalysisConfig().CodeGraph.Enabled)
	require.Len(t, reloaded.GetWorkspaceConfig().Projects, 1)
	require.Equal(t, []string{".*", "dist/**"}, reloaded.GetExclude())
}

func TestNewRepositoryUsesDefaultExcludePatterns(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".skills-seed"), 0755))

	repo, err := NewRepository(filepath.Join(projectRoot, ".skills-seed"), "zh-CN")
	require.NoError(t, err)

	require.Equal(t, DefaultExcludePatterns(), repo.GetExclude())

	content, err := os.ReadFile(filepath.Join(projectRoot, ".skills-seed", "config.yaml"))
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, `- "vendor/**"`)
	require.Contains(t, text, `- "*.log"`)
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
  engine: "claude"
learning:
  max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
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
  engine: "claude"
learning:
  max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
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

	assert.Equal(t, "claude", agentCfg.Engine)
	assert.Equal(t, "claude", agentCfg.Commands["claude"])
	assert.Equal(t, "codex", agentCfg.Commands["codex"])
	assert.Equal(t, 1800, agentCfg.Timeout)
	assert.False(t, agentCfg.AllowUserPlugins)
}

func TestEffectiveSkillsPath(t *testing.T) {
	skills := SkillsConfig{
		Target: "beta",
		Paths: map[string]string{
			"alpha": "alpha/skills",
			"beta":  "beta/skills",
		},
	}

	assert.Equal(t, "alpha/skills", EffectiveSkillsPath("alpha", skills))
	assert.Equal(t, "beta/skills", EffectiveSkillsPath("beta", skills))
	assert.Equal(t, "", EffectiveSkillsPath("gamma", skills))
	assert.Equal(t, "", EffectiveSkillsPath("", skills))
	assert.Equal(t, "beta", EffectiveSkillsTarget(AgentConfig{Engine: "alpha"}, skills))
	assert.Equal(t, "alpha", EffectiveSkillsTarget(AgentConfig{Engine: "alpha"}, SkillsConfig{}))
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

func TestRepository_GetSkillsConfig(t *testing.T) {
	repo := setupTestConfig(t)
	skillsCfg := repo.GetSkillsConfig()

	assert.NotEmpty(t, skillsCfg.Paths)
}

func TestDefaultSkillsPathForTarget(t *testing.T) {
	assert.Equal(t, ".claude/skills/skills-seed-skills", DefaultSkillsPathForTarget("claude"))
	assert.Equal(t, ".agents/skills/skills-seed-skills", DefaultSkillsPathForTarget("codex"))
	assert.Equal(t, ".skills/skills-seed-skills", DefaultSkillsPathForTarget("custom"))
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
	for _, pattern := range DefaultExcludePatterns() {
		assert.Contains(t, exclude, pattern)
	}
	assert.NotContains(t, exclude, "**/*.pb.go")
	assert.NotContains(t, exclude, "**/*.gen.go")
	assert.NotContains(t, exclude, "**/mocks/**")
	assert.NotContains(t, exclude, "**/testdata/**")
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
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	repo, err := NewRepository(seedPath, "")
	require.NoError(t, err)
	require.NotNil(t, repo)

	cfg := repo.Get()
	assert.Equal(t, "zh-CN", cfg.Project.Locale)
}

func TestNormalizeLocaleDefaultsToChinese(t *testing.T) {
	assert.Equal(t, "zh-CN", normalizeLocale(""))
	assert.Equal(t, "zh-CN", normalizeLocale("en_US.UTF-8"))
}

func TestNormalizeLocalePreservesExplicitEnglish(t *testing.T) {
	assert.Equal(t, "en-US", normalizeLocale("en-US"))
}

func TestNewRepository_EnUSLocale(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	require.NotNil(t, repo)

	cfg := repo.Get()
	assert.Equal(t, "en-US", cfg.Project.Locale)
}
