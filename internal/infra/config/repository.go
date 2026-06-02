package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Project   ProjectConfig   `yaml:"project"`
	Workspace WorkspaceConfig `yaml:"workspace"`
	Analysis  AnalysisConfig  `yaml:"analysis"`
	Agent     AgentConfig     `yaml:"agent"`
	Learning  LearningConfig  `yaml:"learning"`
	AutoFix   AutoFixConfig   `yaml:"autofix"`
	Skills    SkillsConfig    `yaml:"skills"`
	Logging   LoggingConfig   `yaml:"logging"`
	Exclude   []string        `yaml:"exclude"` // 全局排除配置
}

// ProjectConfig 项目配置
type ProjectConfig struct {
	Name          string `yaml:"name"`
	Mode          string `yaml:"mode"` // 初始化模式：project 或 workspace
	Language      string `yaml:"language"`
	InitializedAt string `yaml:"initialized_at"` // 改用字符串存储，更易读
	GitRemote     string `yaml:"git_remote"`
	RootPath      string `yaml:"root_path"`
	Locale        string `yaml:"locale"` // 语言设置：zh-CN, en-US
}

// WorkspaceConfig 工作区配置
type WorkspaceConfig struct {
	Projects  []WorkspaceProjectConfig `yaml:"projects"`
	Shared    []WorkspacePathConfig    `yaml:"shared"`
	Contracts []WorkspacePathConfig    `yaml:"contracts"`
	Infra     []WorkspacePathConfig    `yaml:"infra"`
}

// WorkspaceProjectConfig 工作区子项目配置
type WorkspaceProjectConfig struct {
	ID       string `yaml:"id"`
	Path     string `yaml:"path"`
	Type     string `yaml:"type"`
	Language string `yaml:"language"`
}

// WorkspacePathConfig 工作区特殊路径配置
type WorkspacePathConfig struct {
	Path        string `yaml:"path"`
	Description string `yaml:"description,omitempty"`
}

// AnalysisConfig 分析增强配置
type AnalysisConfig struct {
	CodeGraph CodeGraphConfig `yaml:"codegraph"`
}

// CodeGraphConfig CodeGraph 结构化分析增强配置
type CodeGraphConfig struct {
	Enabled  bool   `yaml:"enabled"`   // 是否启用 CodeGraph 增强分析
	Required bool   `yaml:"required"`  // 是否要求 CodeGraph 必须可用
	Command  string `yaml:"command"`   // codegraph 命令路径
	AutoInit bool   `yaml:"auto_init"` // 目标项目未初始化时是否自动执行 init -i
	AutoSync bool   `yaml:"auto_sync"` // 目标项目已有索引时是否自动 sync
	MaxNodes int    `yaml:"max_nodes"` // context 最大符号节点数
	MaxCode  int    `yaml:"max_code"`  // context 最大代码块数；0 表示不包含代码块

	defaultsApplied bool `yaml:"-"`
}

func defaultCodeGraphConfig() CodeGraphConfig {
	return CodeGraphConfig{
		Enabled:         true,
		Required:        false,
		Command:         "codegraph",
		AutoInit:        true,
		AutoSync:        true,
		MaxNodes:        30,
		MaxCode:         0,
		defaultsApplied: true,
	}
}

func DefaultExcludePatterns() []string {
	return []string{
		".*",
		"vendor/**",
		"node_modules/**",
		"dist/**",
		"build/**",
		"out/**",
		"target/**",
		"coverage/**",
		".cache/**",
		"tmp/**",
		"temp/**",
		"*.log",
		"*.tmp",
		"*.bak",
		"*.swp",
		"*.zip",
		"*.tar",
		"*.tar.gz",
		"*.tgz",
		"*.rar",
		"*.7z",
		"*.png",
		"*.jpg",
		"*.jpeg",
		"*.gif",
		"*.webp",
		"*.ico",
		"*.pdf",
		"*.mp4",
		"*.mov",
	}
}

// UnmarshalYAML 在应用默认值的同时保留显式设置的 false 值。
func (c *CodeGraphConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawCodeGraphConfig CodeGraphConfig
	defaults := rawCodeGraphConfig(defaultCodeGraphConfig())
	if err := value.Decode(&defaults); err != nil {
		return err
	}
	*c = CodeGraphConfig(defaults)
	c.defaultsApplied = true
	return nil
}

// AgentConfig Agent 配置
type AgentConfig struct {
	Engine           string            `yaml:"engine"`             // Agent 引擎
	Commands         map[string]string `yaml:"commands"`           // engine -> CLI 命令
	Timeout          int               `yaml:"timeout"`            // 超时时间（秒）
	AllowUserPlugins bool              `yaml:"allow_user_plugins"` // 是否加载用户插件
	Parallelism      int               `yaml:"parallelism"`        // 并发 Agent 数，0 表示自动
}

// LearningConfig 学习配置
type LearningConfig struct {
	MaxCommits int `yaml:"max_commits"` // 默认分析的提交数量
	BatchSize  int `yaml:"batch_size"`  // 批量分析 commit 数量（默认10）
}

// AutoFixConfig 自动修复配置
type AutoFixConfig struct {
	Strategy   string `yaml:"strategy"`    // 修复策略：patch, backup, stash, branch
	BackupPath string `yaml:"backup_path"` // 备份路径（相对于 .skills-seed 目录）
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level       string `yaml:"level"`         // 日志级别：DEBUG, INFO, WARN, ERROR
	LogsPath    string `yaml:"logs_path"`     // 日志路径（相对于 .skills-seed 目录）
	MaxLogFiles int    `yaml:"max_log_files"` // 最大日志文件数量
}

// SkillsConfig Skills 输出配置
type SkillsConfig struct {
	Target string            `yaml:"target"` // 目标 Agent Skills 类型
	Paths  map[string]string `yaml:"paths"`  // target -> Skills 输出路径
}

func EffectiveSkillsTarget(agent AgentConfig, skills SkillsConfig) string {
	if strings.TrimSpace(skills.Target) != "" {
		return strings.TrimSpace(skills.Target)
	}
	return strings.TrimSpace(agent.Engine)
}

// EffectiveSkillsPath 获取指定 target 的 Skills 输出路径
func EffectiveSkillsPath(target string, skills SkillsConfig) string {
	if target != "" && skills.Paths != nil {
		return skills.Paths[target]
	}
	return ""
}

func DefaultSkillsPathForTarget(target string) string {
	switch target {
	case "claude":
		return ".claude/skills/skills-seed-skills"
	case "codex":
		return ".agents/skills/skills-seed-skills"
	default:
		return ".skills/skills-seed-skills"
	}
}

// Repository 配置仓储
type Repository struct {
	configPath string
	config     *Config
}

// NewRepository 创建配置仓储
func NewRepository(seedPath string, locale string) (*Repository, error) {
	configPath := filepath.Join(seedPath, "config.yaml")

	repo := &Repository{
		configPath: configPath,
	}

	// 加载配置
	cfg, err := repo.load()
	if err != nil {
		// 如果配置文件不存在，创建默认配置
		var pathErr *os.PathError
		if errors.As(err, &pathErr) || errors.Is(err, os.ErrNotExist) {
			cfg = repo.defaultConfig(locale)
			cfg.Exclude = DefaultExcludePatterns()
			if err := repo.save(cfg); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	repo.config = cfg
	return repo, nil
}

// Get 获取配置
func (r *Repository) Get() *Config {
	return r.config
}

// Update 更新配置
func (r *Repository) Update(cfg *Config) error {
	if err := r.save(cfg); err != nil {
		return err
	}
	r.config = cfg
	return nil
}

// load 加载配置
func (r *Repository) load() (*Config, error) {
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ConfigReadFailed"), err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ConfigParseFailed"), err)
	}

	r.normalizeConfig(&cfg)
	return &cfg, nil
}

// save 保存配置（保留注释）
func (r *Repository) save(cfg *Config) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(r.configPath), 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ConfigCreateDirFailed"), err)
	}

	root, err := r.loadConfigYAMLNode(cfg)
	if err != nil {
		return r.saveWithMarshal(cfg)
	}

	applyConfigNodeValues(root, cfg)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		_ = encoder.Close()
		return r.saveWithMarshal(cfg)
	}
	if err := encoder.Close(); err != nil {
		return r.saveWithMarshal(cfg)
	}

	content := buf.String()
	var parsed Config
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return r.saveWithMarshal(cfg)
	}

	// 写入文件
	if err := os.WriteFile(r.configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ConfigWriteFailed"), err)
	}

	return nil
}

func (r *Repository) loadConfigYAMLNode(cfg *Config) (*yaml.Node, error) {
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		data, err = r.defaultConfigTemplate(cfg)
		if err != nil {
			return nil, err
		}
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config yaml root is not a mapping")
	}
	return &root, nil
}

func (r *Repository) defaultConfigTemplate(cfg *Config) ([]byte, error) {
	locale := domain.DefaultLocale
	if cfg != nil && cfg.Project.Locale != "" {
		locale = cfg.Project.Locale
	} else if r.config != nil && r.config.Project.Locale != "" {
		locale = r.config.Project.Locale
	}
	templateName := fmt.Sprintf("templates/config/config.yaml.%s.tmpl", locale)
	templateData, err := embedfs.FS.ReadFile(templateName)
	if err == nil {
		return templateData, nil
	}
	if locale != domain.DefaultLocale {
		return embedfs.FS.ReadFile("templates/config/config.yaml.zh-CN.tmpl")
	}
	return nil, err
}

// saveWithMarshal 使用 yaml.Marshal 保存（后备方案，会丢失注释）
func (r *Repository) saveWithMarshal(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ConfigMarshalFailed"), err)
	}

	if err := os.WriteFile(r.configPath, data, 0644); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ConfigWriteFailed"), err)
	}

	return nil
}

// defaultConfig 默认配置
func (r *Repository) defaultConfig(locale string) *Config {
	// 如果指定了 locale，使用指定的；否则检测系统语言
	if locale == "" {
		locale = r.detectSystemLocale()
	}

	// 根据语言选择对应的模板文件
	templateName := fmt.Sprintf("templates/config/config.yaml.%s.tmpl", locale)
	templateData, err := embedfs.FS.ReadFile(templateName)
	if err != nil {
		// 如果指定语言的模板不存在，使用中文模板
		if locale != domain.DefaultLocale {
			templateData, err = embedfs.FS.ReadFile("templates/config/config.yaml.zh-CN.tmpl")
			if err != nil {
				// 如果中文模板也失败，使用最小后备配置
				return r.fallbackDefaultConfig(locale)
			}
		} else {
			// 如果读取模板失败，使用最小后备配置
			return r.fallbackDefaultConfig(locale)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(templateData, &cfg); err != nil {
		// 如果解析失败，使用最小后备配置
		return r.fallbackDefaultConfig(locale)
	}

	// 确保语言设置正确
	if cfg.Project.Locale == "" {
		cfg.Project.Locale = locale
	}

	// 设置初始化时间
	cfg.Project.InitializedAt = time.Now().Format("2006-01-02 15:04:05")
	r.normalizeConfig(&cfg)

	return &cfg
}

func (r *Repository) normalizeConfig(cfg *Config) {
	if !cfg.Analysis.CodeGraph.defaultsApplied {
		cfg.Analysis.CodeGraph = defaultCodeGraphConfig()
	}
	if cfg.Analysis.CodeGraph.Command == "" {
		cfg.Analysis.CodeGraph.Command = "codegraph"
	}
	if cfg.Analysis.CodeGraph.MaxNodes <= 0 {
		cfg.Analysis.CodeGraph.MaxNodes = 30
	}
	if cfg.Analysis.CodeGraph.MaxCode < 0 {
		cfg.Analysis.CodeGraph.MaxCode = 0
	}

	if cfg.Agent.Commands == nil {
		cfg.Agent.Commands = map[string]string{}
	}

	if cfg.Agent.Engine == "" {
		if len(cfg.Agent.Commands) > 0 {
			cfg.Agent.Engine = sortedStringKeys(cfg.Agent.Commands)[0]
		} else {
			cfg.Agent.Engine = "claude"
		}
	}

	if cfg.Agent.Commands[cfg.Agent.Engine] == "" {
		cfg.Agent.Commands[cfg.Agent.Engine] = cfg.Agent.Engine
	}

	if cfg.Agent.Timeout == 0 {
		cfg.Agent.Timeout = 1800
	}

	if strings.TrimSpace(cfg.Skills.Target) == "" {
		cfg.Skills.Target = cfg.Agent.Engine
	}
	if cfg.Skills.Paths == nil {
		cfg.Skills.Paths = map[string]string{}
	}
	if cfg.Skills.Paths[cfg.Skills.Target] == "" {
		cfg.Skills.Paths[cfg.Skills.Target] = DefaultSkillsPathForTarget(cfg.Skills.Target)
	}

	if cfg.Project.Mode == "" {
		cfg.Project.Mode = domain.ModeProject
	}
	if cfg.Project.Mode != domain.ModeProject && cfg.Project.Mode != domain.ModeWorkspace {
		cfg.Project.Mode = domain.ModeProject
	}

	if cfg.Skills.Paths == nil {
		cfg.Skills.Paths = map[string]string{}
	}
}

// detectSystemLocale 检测系统语言
func (r *Repository) detectSystemLocale() string {
	// 检查 LANG 环境变量
	lang := os.Getenv("LANG")
	if strings.Contains(lang, "zh_CN") || strings.Contains(lang, "zh-CN") {
		return "zh-CN"
	}

	// 检查 LC_ALL 环境变量
	lcAll := os.Getenv("LC_ALL")
	if strings.Contains(lcAll, "zh_CN") || strings.Contains(lcAll, "zh-CN") {
		return "zh-CN"
	}

	// macOS 检查
	if _, err := os.Stat("/usr/bin/defaults"); err == nil {
		cmd := exec.Command("defaults", "read", "-g", "AppleLocale")
		if output, err := cmd.Output(); err == nil {
			locale := strings.TrimSpace(string(output))
			if strings.Contains(locale, "zh_CN") || strings.Contains(locale, "zh-CN") {
				return "zh-CN"
			}
		}
	}

	// 默认使用英文
	return "en-US"
}

// fallbackDefaultConfig 是模板不可用时的最小后备配置
func (r *Repository) fallbackDefaultConfig(locale string) *Config {
	if locale == "" {
		locale = domain.DefaultLocale
	}

	return &Config{
		Project: ProjectConfig{
			Name:          "project",
			Mode:          domain.ModeProject,
			Language:      "go",
			InitializedAt: time.Now().Format("2006-01-02 15:04:05"),
			Locale:        locale,
		},
		Agent: AgentConfig{
			Engine: "agent",
			Commands: map[string]string{
				"agent": "agent",
			},
			Timeout:          1800,
			AllowUserPlugins: false,
			Parallelism:      0,
		},
		Analysis: AnalysisConfig{
			CodeGraph: defaultCodeGraphConfig(),
		},
		Learning: LearningConfig{
			MaxCommits: 50,
			BatchSize:  5,
		},
		AutoFix: AutoFixConfig{
			Strategy:   "patch",
			BackupPath: "backups",
		},
		Skills: SkillsConfig{
			Target: "agent",
			Paths: map[string]string{
				"agent": ".skills/skills-seed-skills",
			},
		},
		Workspace: WorkspaceConfig{},
		Logging: LoggingConfig{
			Level:       "DEBUG",
			LogsPath:    "logs",
			MaxLogFiles: 30,
		},
		Exclude: DefaultExcludePatterns(),
	}
}

// Reader 配置读取接口（供 service 层依赖）
type Reader interface {
	GetProjectConfig() ProjectConfig
	GetWorkspaceConfig() WorkspaceConfig
	GetAnalysisConfig() AnalysisConfig
	GetAgentConfig() AgentConfig
	GetLearningConfig() LearningConfig
	GetAutoFixConfig() AutoFixConfig
	GetSkillsConfig() SkillsConfig
	GetLoggingConfig() LoggingConfig
	GetExclude() []string
}

// GetProjectConfig 获取项目配置
func (r *Repository) GetProjectConfig() ProjectConfig {
	return r.config.Project
}

// GetWorkspaceConfig 获取工作区配置
func (r *Repository) GetWorkspaceConfig() WorkspaceConfig {
	return r.config.Workspace
}

// GetAnalysisConfig 获取分析增强配置
func (r *Repository) GetAnalysisConfig() AnalysisConfig {
	return r.config.Analysis
}

// GetAgentConfig 获取 Agent 配置
func (r *Repository) GetAgentConfig() AgentConfig {
	return r.config.Agent
}

// GetLearningConfig 获取学习配置
func (r *Repository) GetLearningConfig() LearningConfig {
	return r.config.Learning
}

// GetAutoFixConfig 获取自动修复配置
func (r *Repository) GetAutoFixConfig() AutoFixConfig {
	return r.config.AutoFix
}

// GetSkillsConfig 获取 Skills 配置
func (r *Repository) GetSkillsConfig() SkillsConfig {
	return r.config.Skills
}

// GetLoggingConfig 获取日志配置
func (r *Repository) GetLoggingConfig() LoggingConfig {
	return r.config.Logging
}

// GetExclude 获取排除配置
func (r *Repository) GetExclude() []string {
	return r.config.Exclude
}

// SetProjectName 设置项目名称
func (r *Repository) SetProjectName(name string) error {
	r.config.Project.Name = name
	return r.Update(r.config)
}

// SetProjectMode 设置初始化模式
func (r *Repository) SetProjectMode(mode string) error {
	r.config.Project.Mode = mode
	return r.Update(r.config)
}

// SetProjectLanguage 设置项目语言
func (r *Repository) SetProjectLanguage(language string) error {
	r.config.Project.Language = language
	return r.Update(r.config)
}

// SetGitRemote 设置 Git Remote
func (r *Repository) SetGitRemote(gitRemote string) error {
	r.config.Project.GitRemote = gitRemote
	return r.Update(r.config)
}

// SetRootPath 设置根路径
func (r *Repository) SetRootPath(rootPath string) error {
	r.config.Project.RootPath = rootPath
	return r.Update(r.config)
}

// SetLocale 设置语言
func (r *Repository) SetLocale(locale string) error {
	r.config.Project.Locale = locale
	return r.Update(r.config)
}

// SetWorkspaceConfig 设置工作区配置
func (r *Repository) SetWorkspaceConfig(workspace WorkspaceConfig) error {
	r.config.Workspace = workspace
	return r.Update(r.config)
}

// SetAutoFixStrategy 设置自动修复策略
func (r *Repository) SetAutoFixStrategy(strategy string) error {
	r.config.AutoFix.Strategy = strategy
	return r.Update(r.config)
}
