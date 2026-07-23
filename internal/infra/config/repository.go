package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"gopkg.in/yaml.v3"
)

// Config 应用配置，映射 .skills-seed/config.yaml 的顶层结构。
type Config struct {
	Project   ProjectConfig   `yaml:"profile"`
	Workspace WorkspaceConfig `yaml:"workspace"`
	Agent     AgentConfig     `yaml:"agent"`
	Learning  LearningConfig  `yaml:"learning"`
	AutoFix   AutoFixConfig   `yaml:"autofix"`
	Skills    SkillsConfig    `yaml:"skills"`
	Logging   LoggingConfig   `yaml:"logging"`
	Exclude   ExcludeConfig   `yaml:"exclude"` // 全局排除配置
}

// ProjectConfig 保存当前项目或工作区根的身份信息。
type ProjectConfig struct {
	Name          string `yaml:"name"`           // 项目或工作区名称
	Mode          string `yaml:"mode"`           // 初始化模式：project 或 workspace
	Language      string `yaml:"language"`       // 主语言，用于提示词上下文
	InitializedAt string `yaml:"initialized_at"` // 改用字符串存储，更易读
	GitRemote     string `yaml:"git_remote"`     // Git 远程地址
	RootPath      string `yaml:"root_path"`      // 项目或工作区根目录绝对路径
	Locale        string `yaml:"locale"`         // 语言设置：zh-CN, en-US
}

// WorkspaceConfig 保存工作区模式下的子项目列表。
type WorkspaceConfig struct {
	Projects []WorkspaceProjectConfig `yaml:"projects"`
}

// WorkspaceProjectConfig 描述一个工作区子项目的路径、类型和语言。
type WorkspaceProjectConfig struct {
	ID       string `yaml:"id"`       // 子项目唯一标识
	Path     string `yaml:"path"`     // 相对工作区根目录的路径
	Type     string `yaml:"type"`     // 子项目类型，如 frontend、backend、library
	Language string `yaml:"language"` // 子项目主语言
}

// ExcludeConfig 控制学习、预览和结构化分析共享的全局排除边界。
type ExcludeConfig struct {
	GitIgnore bool     `yaml:"gitignore"` // 是否叠加 Git ignore 规则过滤文件
	Paths     []string `yaml:"paths"`     // 需要排除的相对路径或 glob

	defaultsApplied bool `yaml:"-"`
}

func defaultExcludeConfig() ExcludeConfig {
	return ExcludeConfig{
		GitIgnore:       true,
		Paths:           DefaultExcludePatterns(),
		defaultsApplied: true,
	}
}

// UnmarshalYAML 在应用默认值的同时保留显式设置的 false 值。
func (c *ExcludeConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawExcludeConfig ExcludeConfig
	defaults := rawExcludeConfig(defaultExcludeConfig())
	if err := value.Decode(&defaults); err != nil {
		return err
	}
	*c = ExcludeConfig(defaults)
	c.defaultsApplied = true
	return nil
}

type StructuralProvider string

const (
	StructuralProviderAuto       StructuralProvider = "auto"       // 使用 CodeGraph，并自动维护项目索引
	StructuralProviderCodeGraph  StructuralProvider = "codegraph"  // 仅使用 CodeGraph，不可用时跳过结构化上下文
	StructuralProviderTreeSitter StructuralProvider = "treesitter" // 仅使用内嵌 tree-sitter
)

// NormalizeStructuralProvider 把结构化上下文 provider 归一化为受支持取值。
func NormalizeStructuralProvider(provider string) StructuralProvider {
	switch StructuralProvider(strings.ToLower(strings.TrimSpace(provider))) {
	case StructuralProviderCodeGraph:
		return StructuralProviderCodeGraph
	case StructuralProviderTreeSitter:
		return StructuralProviderTreeSitter
	default:
		return StructuralProviderAuto
	}
}

// StructuralConfig 结构化分析配置。
type StructuralConfig struct {
	Enabled     bool               `yaml:"enabled"`       // 是否启用结构化分析
	Provider    StructuralProvider `yaml:"provider"`      // 结构化上下文与符号校验来源：auto、codegraph、treesitter
	MaxSymbols  int                `yaml:"max_symbols"`   // context 最大符号数
	MaxFileSize int                `yaml:"max_file_size"` // 跳过超过此大小的文件（KB），默认 512

	defaultsApplied bool `yaml:"-"`
}

func defaultStructuralConfig() StructuralConfig {
	return StructuralConfig{
		Enabled:         true,
		Provider:        StructuralProviderAuto,
		MaxSymbols:      30,
		MaxFileSize:     512,
		defaultsApplied: true,
	}
}

// UnmarshalYAML 在应用默认值的同时保留显式设置的 false 值。
func (c *StructuralConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawStructuralConfig StructuralConfig
	defaults := rawStructuralConfig(defaultStructuralConfig())
	if err := value.Decode(&defaults); err != nil {
		return err
	}
	*c = StructuralConfig(defaults)
	c.defaultsApplied = true
	return nil
}

// AgentConfig 控制调用外部 Agent CLI 的引擎、命令、超时和并发策略。
type AgentConfig struct {
	Engine           string            `yaml:"engine"`             // Agent 引擎
	Commands         map[string]string `yaml:"commands"`           // engine -> CLI 命令
	Timeout          int               `yaml:"timeout"`            // 超时时间（秒）
	AllowUserPlugins bool              `yaml:"allow_user_plugins"` // 是否加载用户插件
	Parallelism      int               `yaml:"parallelism"`        // 并发 Agent 数，0 表示自动
	Retry            RetryConfig       `yaml:"retry"`              // 重试配置
}

// RetryConfig 可重试错误（429/529 等）的重试配置
type RetryConfig struct {
	MaxRetries      int `yaml:"max_retries"`      // 最大重试次数，0 表示不重试
	InitialInterval int `yaml:"initial_interval"` // 首次重试等待秒数
	MaxInterval     int `yaml:"max_interval"`     // 最大等待秒数（指数退避上限）
}

// DefaultRetryConfig 返回默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		InitialInterval: 15,
		MaxInterval:     120,
	}
}

// EffectiveMaxRetries 返回有效最大重试次数
func (r RetryConfig) EffectiveMaxRetries() int {
	if r.MaxRetries == 0 {
		return DefaultRetryConfig().MaxRetries
	}
	return r.MaxRetries
}

// EffectiveInitialInterval 返回有效首次重试等待时间
func (r RetryConfig) EffectiveInitialInterval() time.Duration {
	if r.InitialInterval == 0 {
		return time.Duration(DefaultRetryConfig().InitialInterval) * time.Second
	}
	return time.Duration(r.InitialInterval) * time.Second
}

// EffectiveMaxInterval 返回有效最大等待时间
func (r RetryConfig) EffectiveMaxInterval() time.Duration {
	if r.MaxInterval == 0 {
		return time.Duration(DefaultRetryConfig().MaxInterval) * time.Second
	}
	return time.Duration(r.MaxInterval) * time.Second
}

// WaitDuration 计算第 attempt 次（0-based）重试的等待时间，指数退避
func (r RetryConfig) WaitDuration(attempt int) time.Duration {
	initial := r.EffectiveInitialInterval()
	maximum := r.EffectiveMaxInterval()
	wait := initial * time.Duration(1<<uint(attempt)) // initial * 2^attempt
	if wait > maximum {
		wait = maximum
	}
	return wait
}

// LearningConfig 控制 learn current 和 learn history 的默认学习范围。
type LearningConfig struct {
	Backend LearningBackend       `yaml:"backend"` // 学习后端：local、hybrid、agent
	Current CurrentLearningConfig `yaml:"current"` // learn current 的文件范围和结构化上下文配置
	History HistoryLearningConfig `yaml:"history"` // learn history 的提交范围配置

	defaultsApplied bool `yaml:"-"`
}

func defaultLearningConfig() LearningConfig {
	return LearningConfig{
		Backend:         LearningBackendHybrid,
		Current:         defaultCurrentLearningConfig(),
		History:         defaultHistoryLearningConfig(),
		defaultsApplied: true,
	}
}

// LearningBackend 控制学习阶段使用本地确定性能力还是外部 Agent。
type LearningBackend string

const (
	LearningBackendLocal  LearningBackend = "local"  // 全部学习阶段只使用本地能力
	LearningBackendHybrid LearningBackend = "hybrid" // 本地优先，仅把疑难项交给 Agent
	LearningBackendAgent  LearningBackend = "agent"  // 使用完整 Agent 学习流程
)

// NormalizeLearningBackend 把配置值归一化为受支持的学习后端。
func NormalizeLearningBackend(backend string) LearningBackend {
	switch LearningBackend(strings.ToLower(strings.TrimSpace(backend))) {
	case LearningBackendLocal:
		return LearningBackendLocal
	case LearningBackendAgent:
		return LearningBackendAgent
	default:
		return LearningBackendHybrid
	}
}

// UnmarshalYAML 在应用默认值的同时保留显式设置的 false 值。
func (c *LearningConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawLearningConfig LearningConfig
	defaults := rawLearningConfig(defaultLearningConfig())
	if err := value.Decode(&defaults); err != nil {
		return err
	}
	*c = LearningConfig(defaults)
	c.defaultsApplied = true
	return nil
}

// LearningMode 控制 learn current 在速度和学习深度之间的取舍。
type LearningMode string

const (
	LearningModeFast   LearningMode = "fast"   // 更快，合并更多相近能力，只学习高价值稳定模式
	LearningModeNormal LearningMode = "normal" // 默认，兼顾质量和速度
	LearningModeDeep   LearningMode = "deep"   // 更深入，保留更多业务边界和细节
)

// NormalizeLearningMode 把配置中的学习模式归一化为受支持的取值。
func NormalizeLearningMode(mode string) LearningMode {
	switch LearningMode(strings.ToLower(strings.TrimSpace(mode))) {
	case LearningModeFast:
		return LearningModeFast
	case LearningModeDeep:
		return LearningModeDeep
	default:
		return LearningModeNormal
	}
}

// LearningScope 控制 learn current 规划分析单元时采用的切分范围。
type LearningScope string

const (
	LearningScopeDomain LearningScope = "domain" // 优先按业务域合并能力，跨插件/接口/模块也尽量归并
	LearningScopeFlow   LearningScope = "flow"   // 按业务流程、资源动作或外部系统职责拆分
	LearningScopeModule LearningScope = "module" // 允许按插件、接口、子模块等工程边界更细拆分
)

// NormalizeLearningScope 把配置中的学习范围归一化为受支持取值。
func NormalizeLearningScope(scope string) LearningScope {
	switch LearningScope(strings.ToLower(strings.TrimSpace(scope))) {
	case LearningScopeDomain:
		return LearningScopeDomain
	case LearningScopeModule:
		return LearningScopeModule
	default:
		return LearningScopeFlow
	}
}

// CurrentLearningConfig 控制 learn current 的文件选择和结构化上下文。
type CurrentLearningConfig struct {
	Mode                             LearningMode     `yaml:"mode"`                                 // 学习模式：fast、normal、deep
	Scope                            LearningScope    `yaml:"scope"`                                // 分析单元切分范围：domain、flow、module
	Parallelism                      int              `yaml:"parallelism"`                          // 单项目分析单元并发数，0 或 1 表示串行
	MaxUnitsPerCall                  int              `yaml:"max_units_per_call"`                   // 单次 AI 调用最多分析的单元数，1 表示不合批
	SelectRelevantFiles              bool             `yaml:"select_relevant_files"`                // 是否先筛选最值得分析的相关文件
	SelectRelevantFilesMinCandidates int              `yaml:"select_relevant_files_min_candidates"` // 候选文件数达到该阈值时才调用 AI 文件筛选
	Structural                       StructuralConfig `yaml:"structural"`                           // 结构化上下文配置

	defaultsApplied bool `yaml:"-"`
}

func defaultCurrentLearningConfig() CurrentLearningConfig {
	return CurrentLearningConfig{
		Mode:                             LearningModeNormal,
		Scope:                            LearningScopeFlow,
		Parallelism:                      1,
		MaxUnitsPerCall:                  1,
		SelectRelevantFiles:              true,
		SelectRelevantFilesMinCandidates: 200,
		Structural:                       defaultStructuralConfig(),
		defaultsApplied:                  true,
	}
}

// UnmarshalYAML 在应用默认值的同时保留显式设置的 false 值。
func (c *CurrentLearningConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawCurrentLearningConfig CurrentLearningConfig
	defaults := rawCurrentLearningConfig(defaultCurrentLearningConfig())
	if err := value.Decode(&defaults); err != nil {
		return err
	}
	*c = CurrentLearningConfig(defaults)
	c.defaultsApplied = true
	return nil
}

// HistoryLearningConfig 控制 learn history 的提交范围。
type HistoryLearningConfig struct {
	MaxCommits int `yaml:"max_commits"` // 默认处理的提交数量
	BatchSize  int `yaml:"batch_size"`  // 单次本地证据事务处理的 commit 数量
}

func defaultHistoryLearningConfig() HistoryLearningConfig {
	return HistoryLearningConfig{
		MaxCommits: 50,
		BatchSize:  5,
	}
}

// AutoFixConfig 控制检查自动修复的修复产物和回滚策略。
type AutoFixConfig struct {
	Strategy   string `yaml:"strategy"`    // 修复策略：patch, backup, stash, branch
	BackupPath string `yaml:"backup_path"` // 备份路径（相对于 .skills-seed 目录）
}

// LoggingConfig 控制命令运行日志的级别、目录和保留数量。
type LoggingConfig struct {
	Level       string `yaml:"level"`         // 日志级别：DEBUG, INFO, WARN, ERROR
	LogsPath    string `yaml:"logs_path"`     // 日志路径（相对于 .skills-seed 目录）
	MaxLogFiles int    `yaml:"max_log_files"` // 最大日志文件数量
}

// SkillsConfig 控制生成的 Skills 类型、输出路径和 AI/Skills 内容语言。
type SkillsConfig struct {
	Target string            `yaml:"target"` // 目标 Agent Skills 类型
	Locale string            `yaml:"locale"` // AI 输出、沉淀内容和生成 Skills 的语言：zh-CN, en-US
	Paths  map[string]string `yaml:"paths"`  // target -> Skills 输出路径
}

func EffectiveSkillsTarget(agent AgentConfig, skills SkillsConfig) string {
	if strings.TrimSpace(skills.Target) != "" {
		return strings.TrimSpace(skills.Target)
	}
	return strings.TrimSpace(agent.Engine)
}

// EffectiveSkillsPath 获取指定目标类型的 Skills 输出路径。
func EffectiveSkillsPath(target string, skills SkillsConfig) string {
	if target != "" && skills.Paths != nil {
		return skills.Paths[target]
	}
	return ""
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
	if cfg != nil {
		r.normalizeConfig(cfg)
	}
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
	if err := rejectDeprecatedConfigKeys(data); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ConfigParseFailed"), err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ConfigParseFailed"), err)
	}

	r.normalizeConfig(&cfg)
	return &cfg, nil
}

func rejectDeprecatedConfigKeys(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	doc := configDocument(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == "file_filter" {
			return fmt.Errorf("deprecated config key file_filter; use exclude.gitignore")
		}
	}
	return nil
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

	content := formatTopLevelModuleSpacing(buf.String())
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
		return nil, fmt.Errorf("%s", i18n.Get("ConfigYAMLRootNotMapping"))
	}
	return &root, nil
}

func (r *Repository) defaultConfigTemplate(cfg *Config) ([]byte, error) {
	locale := DefaultToolLocale
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
	if locale != DefaultToolLocale {
		return embedfs.FS.ReadFile("templates/config/config.yaml." + DefaultToolLocale + ".tmpl")
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
	locale = normalizeLocale(locale)

	// 根据语言选择对应的模板文件
	templateName := fmt.Sprintf("templates/config/config.yaml.%s.tmpl", locale)
	templateData, err := embedfs.FS.ReadFile(templateName)
	if err != nil {
		// 如果指定语言的模板不存在，使用中文模板
		if locale != DefaultToolLocale {
			templateData, err = embedfs.FS.ReadFile("templates/config/config.yaml." + DefaultToolLocale + ".tmpl")
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
	if !cfg.Exclude.defaultsApplied {
		cfg.Exclude = defaultExcludeConfig()
	}
	if !cfg.Learning.defaultsApplied {
		cfg.Learning = defaultLearningConfig()
	}
	cfg.Learning.Backend = NormalizeLearningBackend(string(cfg.Learning.Backend))
	if !cfg.Learning.Current.defaultsApplied {
		cfg.Learning.Current = defaultCurrentLearningConfig()
	}
	if !cfg.Learning.Current.Structural.defaultsApplied {
		cfg.Learning.Current.Structural = defaultStructuralConfig()
	}
	cfg.Learning.Current.Mode = NormalizeLearningMode(string(cfg.Learning.Current.Mode))
	cfg.Learning.Current.Scope = NormalizeLearningScope(string(cfg.Learning.Current.Scope))
	if cfg.Learning.Current.Parallelism < 0 {
		cfg.Learning.Current.Parallelism = 1
	}
	if cfg.Learning.Current.MaxUnitsPerCall <= 0 {
		cfg.Learning.Current.MaxUnitsPerCall = 1
	}
	if cfg.Learning.Current.Structural.MaxSymbols <= 0 {
		cfg.Learning.Current.Structural.MaxSymbols = 30
	}
	if cfg.Learning.Current.Structural.MaxFileSize <= 0 {
		cfg.Learning.Current.Structural.MaxFileSize = 512
	}
	cfg.Learning.Current.Structural.Provider = NormalizeStructuralProvider(string(cfg.Learning.Current.Structural.Provider))
	if cfg.Learning.Current.SelectRelevantFilesMinCandidates <= 0 {
		cfg.Learning.Current.SelectRelevantFilesMinCandidates = 200
	}
	cfg.Learning.Current.defaultsApplied = true
	if cfg.Learning.History.MaxCommits <= 0 {
		cfg.Learning.History.MaxCommits = 50
	}
	if cfg.Learning.History.BatchSize <= 0 {
		cfg.Learning.History.BatchSize = 5
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
	cfg.Skills.Locale = normalizeSkillsLocale(cfg.Skills.Locale)
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

func normalizeLocale(locale string) string {
	return NormalizeToolLocale(locale)
}

func normalizeSkillsLocale(locale string) string {
	return NormalizeSkillsLocale(locale)
}

// fallbackDefaultConfig 是模板不可用时的最小后备配置
func (r *Repository) fallbackDefaultConfig(locale string) *Config {
	if locale == "" {
		locale = DefaultToolLocale
	}

	return &Config{
		Project: ProjectConfig{
			Name:          "project",
			Mode:          domain.ModeProject,
			Language:      "",
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
		Learning: defaultLearningConfig(),
		AutoFix: AutoFixConfig{
			Strategy:   "patch",
			BackupPath: "backups",
		},
		Skills: SkillsConfig{
			Target: "agent",
			Locale: DefaultSkillsLocale,
			Paths: map[string]string{
				"agent": DefaultSkillsPathForTarget("agent"),
			},
		},
		Workspace: WorkspaceConfig{},
		Logging: LoggingConfig{
			Level:       "DEBUG",
			LogsPath:    "runtime/logs",
			MaxLogFiles: 30,
		},
		Exclude: defaultExcludeConfig(),
	}
}

// Reader 配置读取接口（供 service 层依赖）
type Reader interface {
	GetProjectConfig() ProjectConfig
	GetWorkspaceConfig() WorkspaceConfig
	GetAgentConfig() AgentConfig
	GetLearningConfig() LearningConfig
	GetLearningBackend() LearningBackend
	GetCurrentLearningConfig() CurrentLearningConfig
	GetAutoFixConfig() AutoFixConfig
	GetSkillsConfig() SkillsConfig
	GetLoggingConfig() LoggingConfig
	GetExcludeConfig() ExcludeConfig
	GetExclude() []string
	GetToolLocale() string
	GetSkillsLocale() string
	GetEffectiveAgentEngine() string
	GetEffectiveAgentCommand() string
	GetEffectiveSkillsTarget() string
	GetEffectiveSkillsPath() string
	GetWorkspaceProjects() []WorkspaceProjectConfig
}

// GetProjectConfig 获取项目配置
func (r *Repository) GetProjectConfig() ProjectConfig {
	return r.config.Project
}

// GetWorkspaceConfig 获取工作区配置
func (r *Repository) GetWorkspaceConfig() WorkspaceConfig {
	return r.config.Workspace
}

// GetAgentConfig 获取 Agent 配置
func (r *Repository) GetAgentConfig() AgentConfig {
	return r.config.Agent
}

// GetLearningConfig 获取学习配置
func (r *Repository) GetLearningConfig() LearningConfig {
	return r.config.Learning
}

// GetLearningBackend 获取学习阶段采用的执行后端。
func (r *Repository) GetLearningBackend() LearningBackend {
	return r.config.Learning.Backend
}

// GetCurrentLearningConfig 获取 learn current 配置。
func (r *Repository) GetCurrentLearningConfig() CurrentLearningConfig {
	return r.config.Learning.Current
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

// GetExcludeConfig 获取全局排除配置。
func (r *Repository) GetExcludeConfig() ExcludeConfig {
	return r.config.Exclude
}

// GetExclude 获取排除配置
func (r *Repository) GetExclude() []string {
	return r.config.Exclude.Paths
}

// GetToolLocale 返回 CLI 输出和运行界面使用的语言。
func (r *Repository) GetToolLocale() string {
	return normalizeLocale(r.config.Project.Locale)
}

// GetSkillsLocale 返回 AI 输出、沉淀内容和生成 Skills 使用的语言。
func (r *Repository) GetSkillsLocale() string {
	return normalizeSkillsLocale(r.config.Skills.Locale)
}

// GetEffectiveAgentEngine 返回应用默认值后的 Agent 引擎。
func (r *Repository) GetEffectiveAgentEngine() string {
	return strings.TrimSpace(r.config.Agent.Engine)
}

// GetEffectiveAgentCommand 返回当前有效 Agent 引擎对应的 CLI 命令。
func (r *Repository) GetEffectiveAgentCommand() string {
	engine := r.GetEffectiveAgentEngine()
	if engine == "" {
		return ""
	}
	if r.config.Agent.Commands != nil && strings.TrimSpace(r.config.Agent.Commands[engine]) != "" {
		return strings.TrimSpace(r.config.Agent.Commands[engine])
	}
	return engine
}

// GetEffectiveSkillsTarget 返回生成 Skills 使用的目标类型。
func (r *Repository) GetEffectiveSkillsTarget() string {
	return EffectiveSkillsTarget(r.config.Agent, r.config.Skills)
}

// GetEffectiveSkillsPath 返回当前有效目标类型对应的 Skills 输出路径。
func (r *Repository) GetEffectiveSkillsPath() string {
	return EffectiveSkillsPath(r.GetEffectiveSkillsTarget(), r.config.Skills)
}

// GetWorkspaceProjects 返回配置中的工作区子项目列表副本。
func (r *Repository) GetWorkspaceProjects() []WorkspaceProjectConfig {
	projects := make([]WorkspaceProjectConfig, len(r.config.Workspace.Projects))
	copy(projects, r.config.Workspace.Projects)
	return projects
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

// SetLocale 设置工具输出、配置模板和 seed context 模板语言。
func (r *Repository) SetLocale(locale string) error {
	r.config.Project.Locale = locale
	return r.Update(r.config)
}

// SetSkillsLocale 设置 AI 输出、沉淀内容和生成 Skills 使用的语言。
func (r *Repository) SetSkillsLocale(locale string) error {
	r.config.Skills.Locale = locale
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
