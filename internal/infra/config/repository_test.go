package config

import (
	"os"
	"path/filepath"
	"strings"
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
		assert.NotContains(t, string(content), "\nanalysis:")
		assert.NotContains(t, string(content), "ai_file_selector:")
		assert.NotContains(t, string(content), "\n  max_commits:")
		assert.NotContains(t, string(content), "\n  batch_size:")
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

		require.Contains(t, text, "########################################################################\n# 基础信息\n# 当前配置文件所属项目或工作区的身份信息\n########################################################################\nprofile:")
		require.NotContains(t, text, "\nproject:")
		require.Contains(t, text, "########################################################################\n# 工作区\n# 仅 workspace 模式生效，普通 project 子仓通常不需要配置\n########################################################################\nworkspace:")
		require.Contains(t, text, "# 项目名称，init 时自动填充\n  name: \"\"")
		require.Contains(t, text, "# 子项目列表，例如 [{id: \"project-a\", path: \"project-a\", type: \"application\", language: \"primary-language\"}]\n  projects: []")
		require.NotContains(t, text, `shared:`)
		require.NotContains(t, text, `contracts:`)
		require.NotContains(t, text, `infra:`)
		require.Contains(t, text, "# 启用相关文件筛选，先基于候选文件树收敛 learn current 的分析范围\n    select_relevant_files: true")
		require.Contains(t, text, "# 学习模式：fast 更快，normal 默认平衡，deep 更深入\n    mode: \"normal\"")
		require.Contains(t, text, "# 候选文件数达到该阈值时才调用 AI 文件筛选；小项目直接使用本地过滤结果\n    select_relevant_files_min_candidates: 200")
		require.Contains(t, text, "# 启用有边界的结构化上下文；无边界输入时不会运行\n      enabled: true")
		require.Contains(t, text, "# 全局排除\n# 控制学习、预览、结构化分析等命令共享的文件边界\n########################################################################\nexclude:")
		require.Contains(t, text, "# 是否排除 Git ignore 命中的文件\n  gitignore: true")
		require.Contains(t, text, "# 需要排除的相对路径或 glob；初始化时写入默认静态排除规则\n  paths:")
		require.NotContains(t, text, "\nfile_filter:")
		assertTopLevelModuleBannersHaveBlankLineBefore(t, text)
		assertCommentLinesDoNotEndWithFullStops(t, text)
		require.NotContains(t, text, `name: ""                   #`)
		require.NotContains(t, text, `projects: []               #`)
		require.NotContains(t, text, `enabled: false            #`)
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

	// 校验来自嵌入模板或硬编码后备配置的默认值。
	assert.Empty(t, cfg.Project.Language)
	assert.Equal(t, "zh-CN", cfg.Project.Locale)
	assert.Equal(t, "en-US", cfg.Skills.Locale)
	assert.Equal(t, "claude", cfg.Agent.Engine)
	assert.Equal(t, "claude", cfg.Agent.Commands["claude"])
	assert.Equal(t, "codex", cfg.Agent.Commands["codex"])
	assert.Equal(t, 1800, cfg.Agent.Timeout)
	assert.False(t, cfg.Agent.AllowUserPlugins)
	assert.True(t, cfg.Learning.Current.SelectRelevantFiles)
	assert.Equal(t, 200, cfg.Learning.Current.SelectRelevantFilesMinCandidates)
	assert.True(t, cfg.Learning.Current.Structural.Enabled)
	assert.Equal(t, 30, cfg.Learning.Current.Structural.MaxSymbols)
	assert.Equal(t, 512, cfg.Learning.Current.Structural.MaxFileSize)
	assert.Equal(t, 50, cfg.Learning.History.MaxCommits)
	assert.Equal(t, 5, cfg.Learning.History.BatchSize)
	assert.Equal(t, "patch", cfg.AutoFix.Strategy)
	assert.Equal(t, "claude", cfg.Skills.Target)
	assert.Equal(t, "en-US", cfg.Skills.Locale)
	assert.Equal(t, ".claude/skills/skills-seed-skills", cfg.Skills.Paths["claude"])
	assert.Equal(t, ".agents/skills/skills-seed-skills", cfg.Skills.Paths["codex"])
	assert.Equal(t, "DEBUG", cfg.Logging.Level)
	assert.Equal(t, "runtime/logs", cfg.Logging.LogsPath)
	assert.True(t, cfg.Exclude.GitIgnore)
	assert.NotEmpty(t, cfg.Project.InitializedAt, "initialized_at should be set")
}

func TestRepository_GetProjectConfig(t *testing.T) {
	repo := setupTestConfig(t)
	projectCfg := repo.GetProjectConfig()

	assert.Empty(t, projectCfg.Language)
	assert.Equal(t, "zh-CN", projectCfg.Locale)
	assert.NotEmpty(t, projectCfg.InitializedAt)
}

func TestRepository_EffectiveGetterDefaults(t *testing.T) {
	repo := setupTestConfig(t)

	assert.Equal(t, "zh-CN", repo.GetToolLocale())
	assert.Equal(t, "en-US", repo.GetSkillsLocale())
	assert.Equal(t, "en-US", repo.GetPromptLocale("learn-batch"))
	assert.Equal(t, "en-US", repo.GetPromptLocale("fix-generate"))
	assert.Equal(t, "claude", repo.GetEffectiveAgentEngine())
	assert.Equal(t, "claude", repo.GetEffectiveAgentCommand())
	assert.Equal(t, "claude", repo.GetEffectiveSkillsTarget())
	assert.Equal(t, ".claude/skills/skills-seed-skills", repo.GetEffectiveSkillsPath())
	assert.Empty(t, repo.GetCurrentProjectConfig().Language)
	assert.Empty(t, repo.GetWorkspaceProjects())
}

func TestRepository_NormalizesMissingSkillsLocaleToEnglish(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "codex"
  commands:
    codex: "codex"
learning:
  max_commits: 50
autofix:
  strategy: "patch"
skills:
  target: "codex"
  paths:
    codex: ".agents/skills/demo"
logging:
  level: "DEBUG"
exclude:
  paths: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	assert.Equal(t, "zh-CN", repo.GetToolLocale())
	assert.Equal(t, "en-US", repo.GetSkillsLocale())
	assert.Equal(t, "en-US", repo.GetSkillsConfig().Locale)
	assert.Equal(t, ".agents/skills/demo", repo.GetEffectiveSkillsPath())
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
	require.NoError(t, repo.Update(cfg))

	content, err := os.ReadFile(filepath.Join(seedPath, "config.yaml"))
	require.NoError(t, err)
	contentText := string(content)
	require.Contains(t, contentText, "\nprofile:")
	require.NotContains(t, contentText, "\nproject:")
	require.Contains(t, contentText, "# 工作区\n# 仅 workspace 模式生效，普通 project 子仓通常不需要配置")
	require.NotContains(t, contentText, `child_skill_policy`)
	require.NotContains(t, contentText, `init_children:`)
	require.Contains(t, contentText, `id: "frontend"`)
	require.NotContains(t, contentText, `shared:`)
	require.NotContains(t, contentText, `contracts:`)
	require.NotContains(t, contentText, `infra:`)
	assertTopLevelModuleBannersHaveBlankLineBefore(t, contentText)
	require.NotContains(t, contentText, `generation:`)
	require.NotContains(t, contentText, `'**/*.pb.go'`)

	reloaded, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	require.Len(t, reloaded.GetWorkspaceConfig().Projects, 2)
	require.Equal(t, "backend", reloaded.GetWorkspaceConfig().Projects[1].ID)
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
		},
		Agent: AgentConfig{
			Engine: "claude",
			Commands: map[string]string{
				"claude": "claude",
				"codex":  "codex",
			},
			Timeout: 1800,
		},
		Learning: defaultLearningConfig(),
		AutoFix:  AutoFixConfig{Strategy: "patch", BackupPath: "backups"},
		Skills: SkillsConfig{Target: "codex", Paths: map[string]string{
			"claude": ".claude/skills/skills-seed-skills",
			"codex":  ".agents/skills/skills-seed-skills",
		}, Locale: "en-US"},
		Logging: LoggingConfig{Level: "DEBUG", LogsPath: "runtime/logs", MaxLogFiles: 30},
		Exclude: ExcludeConfig{
			GitIgnore: true,
			Paths:     []string{"dist/**", "*.log"},
		},
	}

	content := repo.replaceConfigValues(string(templateData), cfg)
	var parsed Config
	require.NoError(t, yaml.Unmarshal([]byte(content), &parsed), content)
	require.Contains(t, content, "\nprofile:")
	require.NotContains(t, content, "\nproject:")
	require.Contains(t, content, "# 工作区\n# 仅 workspace 模式生效，普通 project 子仓通常不需要配置")
	require.NotContains(t, content, `child_skill_policy`)
	require.NotContains(t, content, `init_children:`)
	require.Contains(t, content, `id: "backend"`)
	require.NotContains(t, content, `shared:`)
	require.NotContains(t, content, `contracts:`)
	require.NotContains(t, content, `infra:`)
	require.NotContains(t, content, `- "**/*.pb.go"`)
	require.NotContains(t, content, `- "**/*.gen.go"`)
	require.Contains(t, content, `- "dist/**"`)
	require.Contains(t, content, `- "*.log"`)
	require.NotContains(t, content, `analysis:`)
	require.NotContains(t, content, `ai_file_selector:`)
	require.Contains(t, content, "# 启用相关文件筛选，先基于候选文件树收敛 learn current 的分析范围\n    select_relevant_files: true")
	require.Contains(t, content, `enabled: true`)
	require.Contains(t, content, "# 全局排除\n# 控制学习、预览、结构化分析等命令共享的文件边界")
	require.Contains(t, content, "# 是否排除 Git ignore 命中的文件\n  gitignore: true")
	require.Contains(t, content, "# 项目名称，init 时自动填充\n  name: \"demo-workspace\"")
	require.Contains(t, content, "# 子项目列表，例如 [{id: \"project-a\", path: \"project-a\", type: \"application\", language: \"primary-language\"}]\n  projects:")
	require.Contains(t, content, "# 启用有边界的结构化上下文；无边界输入时不会运行\n      enabled: true")
	require.Contains(t, content, "# 需要排除的相对路径或 glob；初始化时写入默认静态排除规则\n  paths:")
	assertTopLevelModuleBannersHaveBlankLineBefore(t, content)
	assertCommentLinesDoNotEndWithFullStops(t, content)
	require.NotContains(t, content, `name: "demo-workspace" #`)
	require.NotContains(t, content, `analysis:`)
	require.NotContains(t, content, `enabled: false #`)
	require.NotContains(t, content, `exclude: #`)
	require.NotContains(t, content, `- "dist/**" #`)
	require.NotContains(t, content, `generation:`)
}

func TestGeneratedConfigCommentLinesDoNotEndWithFullStops(t *testing.T) {
	for _, locale := range []string{"zh-CN", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			seedPath := t.TempDir()
			configPath := filepath.Join(seedPath, "config.yaml")

			_, err := NewRepository(seedPath, locale)
			require.NoError(t, err)

			content, err := os.ReadFile(configPath)
			require.NoError(t, err)
			assertCommentLinesDoNotEndWithFullStops(t, string(content))
		})
	}
}

func assertCommentLinesDoNotEndWithFullStops(t *testing.T, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		require.Falsef(t, strings.HasSuffix(trimmed, "。") || strings.HasSuffix(trimmed, "."),
			"comment line must not end with a full stop: %q", line)
	}
}

func assertTopLevelModuleBannersHaveBlankLineBefore(t *testing.T, text string) {
	t.Helper()
	banner := strings.Repeat("#", 72)
	lines := strings.Split(text, "\n")
	bannerCount := 0
	for i, line := range lines {
		if line != banner {
			continue
		}
		if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "# ") {
			continue
		}
		bannerCount++
		if bannerCount == 1 {
			continue
		}
		require.Greater(t, i, 0, "module banner should not be first line after first banner")
		require.Equalf(t, "", lines[i-1], "module banner should have one blank line before it at line %d", i+1)
	}
	require.GreaterOrEqual(t, bannerCount, 2, "expected multiple module banners")
}

func TestRepository_UpdatePreservesExistingComments(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`# 自定义项目注释
profile:
  name: "old-name" # 自定义项目名称注释
  mode: "project"
  language: "go"
  locale: "zh-CN"
  git_remote: ""
  root_path: ""
  initialized_at: "2026-05-26 12:00:00"

# 自定义工作区注释
workspace:
  projects: [] # 自定义子项目注释

agent:
  engine: "claude"
  commands:
    claude: "claude"
    codex: "codex"
  timeout: 1800
  allow_user_plugins: false
  parallelism: 0

learning:
  current:
    select_relevant_files: true
    select_relevant_files_min_candidates: 25
    structural:
      enabled: true # 自定义结构化上下文注释
      max_symbols: 30
      max_file_size: 512
  history:
    max_commits: 50
    batch_size: 5

autofix:
  strategy: "patch"
  backup_path: "backups"

skills:
  target: "claude"
  locale: "en-US"
  paths:
    claude: ".claude/skills/skills-seed-skills"
    codex: ".agents/skills/skills-seed-skills"

logging:
  level: "DEBUG"
  logs_path: "runtime/logs"
  max_log_files: 30

exclude:
  gitignore: true
  paths:
    - ".*" # 保留点号文件注释
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
	cfg.Learning.Current.Structural.Enabled = false
	cfg.Learning.Current.SelectRelevantFiles = false
	cfg.Learning.Current.SelectRelevantFilesMinCandidates = 40
	cfg.Exclude.GitIgnore = false
	cfg.Exclude.Paths = []string{".*", "dist/**"}
	require.NoError(t, repo.Update(cfg))

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "# 自定义项目注释")
	require.Contains(t, text, "# 自定义项目名称注释\n  name: \"new-name\"")
	require.Contains(t, text, "# 自定义工作区注释")
	require.Contains(t, text, "# 自定义子项目注释\n  projects:")
	require.Contains(t, text, "# 自定义结构化上下文注释\n      enabled: false")
	require.Contains(t, text, "exclude:\n  gitignore: false")
	require.Contains(t, text, "# 保留点号文件注释\n    - \".*\"")
	require.NotContains(t, text, "\nfile_filter:")
	require.NotContains(t, text, `name: "new-name" # 自定义项目名称注释`)
	require.NotContains(t, text, `projects: # 自定义子项目注释`)
	require.NotContains(t, text, `shared:`)
	require.NotContains(t, text, `contracts:`)
	require.NotContains(t, text, `infra:`)
	require.NotContains(t, text, `analysis:`)
	require.NotContains(t, text, `enabled: false # 自定义结构化分析注释`)
	require.NotContains(t, text, `- ".*" # 保留点号文件注释`)

	reloaded, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	require.Equal(t, "new-name", reloaded.GetProjectConfig().Name)
	require.Equal(t, "workspace", reloaded.GetProjectConfig().Mode)
	require.False(t, reloaded.GetCurrentLearningConfig().Structural.Enabled)
	require.Equal(t, LearningModeNormal, reloaded.GetCurrentLearningConfig().Mode)
	require.False(t, reloaded.GetCurrentLearningConfig().SelectRelevantFiles)
	require.Equal(t, 40, reloaded.GetCurrentLearningConfig().SelectRelevantFilesMinCandidates)
	require.False(t, reloaded.GetExcludeConfig().GitIgnore)
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
	require.Contains(t, text, `gitignore: true`)
	require.Contains(t, text, `paths:`)
}

func TestRepository_NormalizeExcludeDefaults(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
learning:
  history:
    max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
logging:
  level: "DEBUG"
exclude:
  paths: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	require.True(t, repo.GetExcludeConfig().GitIgnore)
}

func TestRepository_PreservesExplicitGitIgnoreDisabled(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
learning:
  history:
    max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
logging:
  level: "DEBUG"
exclude:
  gitignore: false
  paths: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	require.False(t, repo.GetExcludeConfig().GitIgnore)
}

func TestRepository_RejectsLegacyExcludeList(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
learning:
  history:
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
	require.Error(t, err)
	require.Nil(t, repo)
}

func TestRepository_RejectsDeprecatedFileFilter(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
file_filter:
  apply_git_ignore: false
learning:
  history:
    max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
logging:
  level: "DEBUG"
exclude:
  paths: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.Error(t, err)
	require.Nil(t, repo)
}

func TestRepository_NormalizeCurrentLearningDefaults(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
learning:
  history:
    max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
logging:
  level: "DEBUG"
exclude:
  paths: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := repo.GetCurrentLearningConfig().Structural
	require.Equal(t, LearningModeNormal, repo.GetCurrentLearningConfig().Mode)
	require.True(t, repo.GetCurrentLearningConfig().SelectRelevantFiles)
	require.Equal(t, 200, repo.GetCurrentLearningConfig().SelectRelevantFilesMinCandidates)
	require.True(t, cfg.Enabled)
	require.Equal(t, 30, cfg.MaxSymbols)
	require.Equal(t, 512, cfg.MaxFileSize)
}

func TestRepository_PreservesExplicitStructuralDisabled(t *testing.T) {
	seedPath := t.TempDir()
	configPath := filepath.Join(seedPath, "config.yaml")
	require.NoError(t, os.MkdirAll(seedPath, 0755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
learning:
  current:
    select_relevant_files: false
    structural:
      enabled: false
      max_symbols: 12
      max_file_size: 256
  history:
    max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
logging:
  level: "DEBUG"
exclude:
  paths: []
`), 0644))

	repo, err := NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)

	cfg := repo.GetCurrentLearningConfig().Structural
	require.Equal(t, LearningModeNormal, repo.GetCurrentLearningConfig().Mode)
	require.False(t, repo.GetCurrentLearningConfig().SelectRelevantFiles)
	require.Equal(t, 200, repo.GetCurrentLearningConfig().SelectRelevantFilesMinCandidates)
	require.False(t, cfg.Enabled)
	require.Equal(t, 12, cfg.MaxSymbols)
	require.Equal(t, 256, cfg.MaxFileSize)
}

func TestRepository_NormalizesCurrentLearningMode(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want LearningMode
	}{
		{name: "fast", raw: "fast", want: LearningModeFast},
		{name: "normal", raw: "normal", want: LearningModeNormal},
		{name: "deep", raw: "deep", want: LearningModeDeep},
		{name: "invalid", raw: "maximum", want: LearningModeNormal},
		{name: "blank", raw: "", want: LearningModeNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seedPath := t.TempDir()
			configPath := filepath.Join(seedPath, "config.yaml")
			require.NoError(t, os.MkdirAll(seedPath, 0755))
			require.NoError(t, os.WriteFile(configPath, []byte(`
profile:
  language: "go"
  locale: "zh-CN"
agent:
  engine: "claude"
learning:
  current:
    mode: "`+tt.raw+`"
    structural:
      enabled: true
  history:
    max_commits: 50
autofix:
  strategy: "patch"
skills:
  paths: {}
logging:
  level: "DEBUG"
exclude:
  paths: []
`), 0644))

			repo, err := NewRepository(seedPath, "zh-CN")
			require.NoError(t, err)
			require.Equal(t, tt.want, repo.GetCurrentLearningConfig().Mode)
		})
	}
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

	assert.True(t, learningCfg.Current.SelectRelevantFiles)
	assert.Equal(t, 200, learningCfg.Current.SelectRelevantFilesMinCandidates)
	assert.True(t, learningCfg.Current.Structural.Enabled)
	assert.Equal(t, 50, learningCfg.History.MaxCommits)
	assert.Equal(t, 5, learningCfg.History.BatchSize)
	assert.Equal(t, learningCfg.Current, repo.GetCurrentLearningConfig())
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

	assert.Equal(t, "en-US", skillsCfg.Locale)
	assert.NotEmpty(t, skillsCfg.Paths)
}

func TestRepositoryDefaultSkillsPathForTargetCompatibility(t *testing.T) {
	assert.Equal(t, ".claude/skills/skills-seed-skills", DefaultSkillsPathForTarget("claude"))
	assert.Equal(t, ".agents/skills/skills-seed-skills", DefaultSkillsPathForTarget("codex"))
	assert.Equal(t, ".skills/skills-seed-skills", DefaultSkillsPathForTarget("custom"))
}

func TestRepository_GetLoggingConfig(t *testing.T) {
	repo := setupTestConfig(t)
	loggingCfg := repo.GetLoggingConfig()

	assert.Equal(t, "DEBUG", loggingCfg.Level)
	assert.Equal(t, "runtime/logs", loggingCfg.LogsPath)
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
	cfg.Learning.History.MaxCommits = 100

	err := repo.Update(cfg)
	require.NoError(t, err)

	updated := repo.Get()
	assert.Equal(t, 3600, updated.Agent.Timeout)
	assert.Equal(t, 100, updated.Learning.History.MaxCommits)
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
	assert.Equal(t, "en-US", cfg.Skills.Locale)
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
	assert.Equal(t, "en-US", cfg.Skills.Locale)
}
