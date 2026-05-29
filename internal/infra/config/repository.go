package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	Output    OutputConfig    `yaml:"output"`
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
	InitChildren bool                     `yaml:"init_children"`
	Projects     []WorkspaceProjectConfig `yaml:"projects"`
	Shared       []WorkspacePathConfig    `yaml:"shared"`
	Contracts    []WorkspacePathConfig    `yaml:"contracts"`
	Infra        []WorkspacePathConfig    `yaml:"infra"`
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
	Provider         string            `yaml:"provider"`           // Agent provider
	Commands         map[string]string `yaml:"commands"`           // provider -> CLI 命令
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

// OutputConfig 输出配置
type OutputConfig struct {
	SkillsPaths map[string]string `yaml:"skills_paths"` // provider -> Skills 输出路径
}

// EffectiveSkillsPath 获取指定 provider 的 Skills 输出路径
func EffectiveSkillsPath(provider string, output OutputConfig) string {
	if provider != "" && output.SkillsPaths != nil {
		return output.SkillsPaths[provider]
	}
	return ""
}

func DefaultSkillsPathForProvider(provider string) string {
	switch provider {
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

	// 根据 locale 选择模板文件
	locale := cfg.Project.Locale
	if locale == "" {
		locale = domain.DefaultLocale // 默认中文
	}

	templateName := fmt.Sprintf("templates/config/config.yaml.%s.tmpl", locale)
	templateData, err := embedfs.FS.ReadFile(templateName)
	if err != nil {
		// 如果指定语言的模板不存在，尝试使用中文模板
		if locale != domain.DefaultLocale {
			templateData, err = embedfs.FS.ReadFile("templates/config/config.yaml.zh-CN.tmpl")
			if err != nil {
				// 如果中文模板也失败，使用 Marshal（会丢失注释）
				return r.saveWithMarshal(cfg)
			}
		} else {
			// 如果读取模板失败，使用 Marshal（会丢失注释）
			return r.saveWithMarshal(cfg)
		}
	}

	// 替换模板中的占位符
	content := string(templateData)
	content = r.replaceConfigValues(content, cfg)
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

// replaceConfigValues 替换配置值（保留注释）
func (r *Repository) replaceConfigValues(content string, cfg *Config) string {
	// 项目信息
	content = replaceYAMLValueInSection(content, "project:", "name:", cfg.Project.Name)
	content = replaceYAMLValueInSection(content, "project:", "mode:", cfg.Project.Mode)
	content = replaceYAMLValueInSection(content, "project:", "language:", cfg.Project.Language)
	content = replaceYAMLValueInSection(content, "project:", "locale:", cfg.Project.Locale)
	content = replaceYAMLValueInSection(content, "project:", "git_remote:", cfg.Project.GitRemote)
	content = replaceYAMLValueInSection(content, "project:", "root_path:", cfg.Project.RootPath)
	if cfg.Project.InitializedAt != "" {
		content = replaceYAMLValueInSection(content, "project:", "initialized_at:", cfg.Project.InitializedAt)
	}

	// 工作区配置
	content = replaceYAMLWorkspaceConfig(content, cfg.Workspace)

	// 分析增强配置
	content = replaceYAMLValueInSection(content, "codegraph:", "enabled:", cfg.Analysis.CodeGraph.Enabled)
	content = replaceYAMLValueInSection(content, "codegraph:", "required:", cfg.Analysis.CodeGraph.Required)
	content = replaceYAMLValueInSection(content, "codegraph:", "command:", cfg.Analysis.CodeGraph.Command)
	content = replaceYAMLValueInSection(content, "codegraph:", "auto_init:", cfg.Analysis.CodeGraph.AutoInit)
	content = replaceYAMLValueInSection(content, "codegraph:", "auto_sync:", cfg.Analysis.CodeGraph.AutoSync)
	content = replaceYAMLValueInSection(content, "codegraph:", "max_nodes:", cfg.Analysis.CodeGraph.MaxNodes)
	content = replaceYAMLValueInSection(content, "codegraph:", "max_code:", cfg.Analysis.CodeGraph.MaxCode)

	// Agent 配置
	content = replaceYAMLValueInSection(content, "agent:", "provider:", cfg.Agent.Provider)
	content = replaceYAMLStringMapInSection(content, "agent:", "commands:", cfg.Agent.Commands)
	content = replaceYAMLValueInSection(content, "agent:", "timeout:", cfg.Agent.Timeout)
	content = replaceYAMLValueInSection(content, "agent:", "allow_user_plugins:", cfg.Agent.AllowUserPlugins)
	content = replaceYAMLValueInSection(content, "agent:", "parallelism:", cfg.Agent.Parallelism)

	// 学习配置
	content = replaceYAMLValueInSection(content, "learning:", "max_commits:", cfg.Learning.MaxCommits)
	content = replaceYAMLValueInSection(content, "learning:", "batch_size:", cfg.Learning.BatchSize)

	// 自动修复配置
	content = replaceYAMLValueInSection(content, "autofix:", "strategy:", cfg.AutoFix.Strategy)
	content = replaceYAMLValueInSection(content, "autofix:", "backup_path:", cfg.AutoFix.BackupPath)

	// 输出配置
	content = replaceYAMLStringMapInSection(content, "output:", "skills_paths:", cfg.Output.SkillsPaths)

	// 日志配置
	content = replaceYAMLValueInSection(content, "logging:", "level:", cfg.Logging.Level)
	content = replaceYAMLValueInSection(content, "logging:", "logs_path:", cfg.Logging.LogsPath)
	content = replaceYAMLValueInSection(content, "logging:", "max_log_files:", cfg.Logging.MaxLogFiles)

	content = replaceYAMLStringListAtRoot(content, "exclude:", cfg.Exclude)

	return content
}

// replaceYAMLValue 替换 YAML 值（保留注释和格式）
func replaceYAMLValue(content, key string, value interface{}) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// 跳过纯注释行
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 检查是否包含目标 key
		if strings.Contains(line, key) {
			// 分离行尾注释
			commentIdx := strings.Index(line, " #")
			var comment string
			var codePart string
			var commentColumn int

			if commentIdx > 0 {
				// 找到了注释，记录注释的列位置
				codePart = line[:commentIdx]
				comment = line[commentIdx+1:] // +1 跳过空格
				commentColumn = commentIdx + 1
			} else {
				codePart = line
				comment = ""
				commentColumn = 0
			}

			// 检查 codePart 是否包含 key
			if !strings.Contains(codePart, key) {
				continue
			}

			// 提取缩进
			indent := ""
			idx := strings.Index(codePart, key)
			if idx > 0 {
				indent = codePart[:idx]
			}

			// 根据值类型格式化
			var valueStr string
			switch v := value.(type) {
			case string:
				valueStr = quoteYAMLString(v)
			case bool:
				valueStr = fmt.Sprintf("%v", v)
			case int:
				valueStr = fmt.Sprintf("%d", v)
			default:
				valueStr = fmt.Sprintf("%v", v)
			}

			// 构建新行
			newCodePart := fmt.Sprintf("%s%s %s", indent, key, valueStr)

			// 如果有注释，保持注释在原始列位置
			if comment != "" {
				// 计算需要填充的空格数，确保注释在原始列位置
				currentLen := len(newCodePart)
				if currentLen < commentColumn {
					// 需要填充空格以对齐注释
					padding := strings.Repeat(" ", commentColumn-currentLen)
					lines[i] = fmt.Sprintf("%s %s", newCodePart+padding, comment)
				} else {
					// 新值太长，至少保留一个空格
					lines[i] = fmt.Sprintf("%s %s", newCodePart, comment)
				}
			} else {
				lines[i] = newCodePart
			}
			break
		}
	}
	return strings.Join(lines, "\n")
}

func replaceYAMLValueInSection(content, section, key string, value interface{}) string {
	lines := strings.Split(content, "\n")
	inSection := false
	sectionIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		if trimmed == section {
			inSection = true
			sectionIndent = indent
			continue
		}

		if inSection && indent <= sectionIndent && trimmed != "" && strings.HasSuffix(trimmed, ":") {
			inSection = false
		}

		if !inSection {
			continue
		}

		if strings.Contains(line, key) {
			lines[i] = replaceYAMLValue(line, key, value)
			break
		}
	}

	return strings.Join(lines, "\n")
}

func replaceYAMLStringMapInSection(content, section, key string, values map[string]string) string {
	if values == nil {
		return content
	}

	lines := strings.Split(content, "\n")
	inSection := false
	sectionIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		if trimmed == section {
			inSection = true
			sectionIndent = indent
			continue
		}

		if inSection && indent <= sectionIndent && trimmed != "" && strings.HasSuffix(trimmed, ":") {
			inSection = false
		}

		if !inSection || !strings.Contains(line, key) {
			continue
		}

		codePart := line
		if commentIdx := strings.Index(line, " #"); commentIdx > 0 {
			codePart = line[:commentIdx]
		}
		if !strings.Contains(codePart, key) {
			continue
		}

		keyIndent := len(line) - len(strings.TrimLeft(line, " "))
		childIndent := strings.Repeat(" ", keyIndent+2)
		replacement := []string{strings.Repeat(" ", keyIndent) + key}
		for _, mapKey := range sortedStringKeys(values) {
			replacement = append(replacement, fmt.Sprintf("%s%s: %s", childIndent, mapKey, quoteYAMLString(values[mapKey])))
		}

		end := i + 1
		for end < len(lines) {
			next := lines[end]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "#") {
				end++
				continue
			}
			nextIndent := len(next) - len(strings.TrimLeft(next, " "))
			if nextIndent <= keyIndent {
				break
			}
			end++
		}

		lines = spliceYAMLLines(lines, i, end, replacement)
		break
	}

	return strings.Join(lines, "\n")
}

func replaceYAMLStringListAtRoot(content, key string, values []string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, key) {
			continue
		}

		commentIdx := strings.Index(line, " #")
		comment := ""
		commentColumn := 0
		if commentIdx > 0 {
			comment = line[commentIdx+1:]
			commentColumn = commentIdx + 1
		}

		replacement := []string{}
		if len(values) == 0 {
			replacement = append(replacement, appendYAMLLineComment(key+" []", yamlLineComment{text: comment, column: commentColumn}))
		} else {
			replacement = append(replacement, appendYAMLLineComment(key, yamlLineComment{text: comment, column: commentColumn}))
			for _, value := range values {
				replacement = append(replacement, "  - "+quoteYAMLString(value))
			}
		}

		end := i + 1
		for end < len(lines) {
			next := lines[end]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "#") {
				end++
				continue
			}
			nextIndent := len(next) - len(strings.TrimLeft(next, " "))
			if nextIndent == 0 {
				break
			}
			end++
		}

		lines = spliceYAMLLines(lines, i, end, replacement)
		break
	}

	return strings.Join(lines, "\n")
}

func quoteYAMLString(value string) string {
	return fmt.Sprintf("%q", value)
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func replaceYAMLWorkspaceConfig(content string, workspace WorkspaceConfig) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "workspace:" {
			start = i
			break
		}
	}
	if start < 0 {
		return content
	}

	end := start + 1
	for end < len(lines) {
		line := lines[end]
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			break
		}
		end++
	}

	comments := workspaceKeyComments(lines[start:end])
	replacement := renderWorkspaceConfig(workspace, comments)

	return strings.Join(spliceYAMLLines(lines, start, end, replacement), "\n")
}

func spliceYAMLLines(lines []string, start, end int, replacement []string) []string {
	result := make([]string, 0, len(lines)-end+start+len(replacement))
	result = append(result, lines[:start]...)
	result = append(result, replacement...)
	result = append(result, lines[end:]...)
	return result
}

type yamlLineComment struct {
	text   string
	column int
}

func workspaceKeyComments(lines []string) map[string]yamlLineComment {
	comments := map[string]yamlLineComment{}
	keys := map[string]string{
		"init_children:": "init_children",
		"projects:":      "projects",
		"shared:":        "shared",
		"contracts:":     "contracts",
		"infra:":         "infra",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for yamlKey, commentKey := range keys {
			if !strings.HasPrefix(trimmed, yamlKey) {
				continue
			}
			commentIdx := strings.Index(line, " #")
			if commentIdx < 0 {
				continue
			}
			comments[commentKey] = yamlLineComment{
				text:   line[commentIdx+1:],
				column: commentIdx + 1,
			}
		}
	}

	return comments
}

func renderWorkspaceConfig(workspace WorkspaceConfig, comments map[string]yamlLineComment) []string {
	lines := []string{"workspace:"}
	lines = append(lines, appendYAMLLineComment(fmt.Sprintf("  init_children: %v", workspace.InitChildren), comments["init_children"]))
	lines = append(lines, renderWorkspaceProjects(workspace.Projects, comments["projects"])...)
	lines = append(lines, renderWorkspacePathList("shared", workspace.Shared, comments["shared"])...)
	lines = append(lines, renderWorkspacePathList("contracts", workspace.Contracts, comments["contracts"])...)
	lines = append(lines, renderWorkspacePathList("infra", workspace.Infra, comments["infra"])...)
	return lines
}

func renderWorkspaceProjects(projects []WorkspaceProjectConfig, comment yamlLineComment) []string {
	if len(projects) == 0 {
		return []string{appendYAMLLineComment("  projects: []", comment)}
	}

	lines := []string{appendYAMLLineComment("  projects:", comment)}
	for _, project := range projects {
		lines = append(lines,
			fmt.Sprintf("    - id: %s", quoteYAMLString(project.ID)),
			fmt.Sprintf("      path: %s", quoteYAMLString(project.Path)),
			fmt.Sprintf("      type: %s", quoteYAMLString(project.Type)),
			fmt.Sprintf("      language: %s", quoteYAMLString(project.Language)),
		)
	}
	return lines
}

func renderWorkspacePathList(key string, paths []WorkspacePathConfig, comment yamlLineComment) []string {
	if len(paths) == 0 {
		return []string{appendYAMLLineComment(fmt.Sprintf("  %s: []", key), comment)}
	}

	lines := []string{appendYAMLLineComment(fmt.Sprintf("  %s:", key), comment)}
	for _, path := range paths {
		lines = append(lines, fmt.Sprintf("    - path: %s", quoteYAMLString(path.Path)))
		if path.Description != "" {
			lines = append(lines, fmt.Sprintf("      description: %s", quoteYAMLString(path.Description)))
		}
	}
	return lines
}

func appendYAMLLineComment(code string, comment yamlLineComment) string {
	if comment.text == "" {
		return code
	}
	spaces := 1
	if len(code) < comment.column {
		spaces = comment.column - len(code)
	}
	return code + strings.Repeat(" ", spaces) + comment.text
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

	if cfg.Agent.Provider == "" {
		if len(cfg.Agent.Commands) > 0 {
			cfg.Agent.Provider = sortedStringKeys(cfg.Agent.Commands)[0]
		} else {
			cfg.Agent.Provider = "claude"
		}
	}

	if cfg.Agent.Commands[cfg.Agent.Provider] == "" {
		cfg.Agent.Commands[cfg.Agent.Provider] = cfg.Agent.Provider
	}

	if cfg.Agent.Timeout == 0 {
		cfg.Agent.Timeout = 1800
	}

	if cfg.Project.Mode == "" {
		cfg.Project.Mode = domain.ModeProject
	}
	if cfg.Project.Mode != domain.ModeProject && cfg.Project.Mode != domain.ModeWorkspace {
		cfg.Project.Mode = domain.ModeProject
	}

	if cfg.Output.SkillsPaths == nil {
		cfg.Output.SkillsPaths = map[string]string{}
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
			Provider: "agent",
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
		Output: OutputConfig{
			SkillsPaths: map[string]string{
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
	GetOutputConfig() OutputConfig
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

// GetOutputConfig 获取输出配置
func (r *Repository) GetOutputConfig() OutputConfig {
	return r.config.Output
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
