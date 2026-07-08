package config

import (
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
)

const (
	// DefaultToolLocale 控制 CLI 文案、配置模板和 seed context 模板的默认语言。
	DefaultToolLocale = i18n.DefaultLocale
	// DefaultSkillsLocale 控制生成 Skills 模板的默认语言。
	DefaultSkillsLocale = i18n.LocaleEnglish
	// DefaultSaveRenderedPrompts 表示默认保留渲染后的提示词文件，便于排查 AI 调用输入。
	DefaultSaveRenderedPrompts = true
	// DefaultAutoDeleteRenderedPrompts 表示默认不自动删除运行时提示词文件。
	DefaultAutoDeleteRenderedPrompts = false
	// DefaultSaveAgentOutputs 表示默认保留 Agent 原始输出文件，便于排查 AI 调用结果。
	DefaultSaveAgentOutputs = DefaultSaveRenderedPrompts
	// DefaultAutoDeleteAgentOutputs 表示默认不自动删除运行时 Agent 输出文件。
	DefaultAutoDeleteAgentOutputs = DefaultAutoDeleteRenderedPrompts
	// DefaultIncludeSkillsSeedGeneratedNotice 表示默认不在生成文件中写入 skills-seed 生成标记。
	DefaultIncludeSkillsSeedGeneratedNotice = false
	// DefaultAnalyzeSourceFilesOnly 表示默认只把源码和构建配置纳入结构化分析。
	DefaultAnalyzeSourceFilesOnly = true
)

// IsSupportedLocale 判断 CLI 或配置中的语言值是否受支持。
// 空值也视为合法，调用方可在后续流程中套用默认语言。
func IsSupportedLocale(locale string) bool {
	switch strings.TrimSpace(locale) {
	case "", i18n.LocaleChinese, i18n.LocaleEnglish:
		return true
	default:
		return false
	}
}

func NormalizeToolLocale(locale string) string {
	switch strings.TrimSpace(locale) {
	case i18n.LocaleEnglish:
		return i18n.LocaleEnglish
	case i18n.LocaleChinese:
		return i18n.LocaleChinese
	default:
		return DefaultToolLocale
	}
}

func NormalizeSkillsLocale(locale string) string {
	switch strings.TrimSpace(locale) {
	case i18n.LocaleChinese:
		return i18n.LocaleChinese
	case i18n.LocaleEnglish:
		return i18n.LocaleEnglish
	default:
		return DefaultSkillsLocale
	}
}

// TemplateLocaleSuffix 返回模板语言后缀；zh-CN 模板沿用无后缀的内嵌默认文件。
func TemplateLocaleSuffix(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" || strings.EqualFold(locale, DefaultToolLocale) {
		return ""
	}
	return locale
}

func DefaultSkillsPathForTarget(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "claude":
		return filepath.ToSlash(filepath.Join(".claude", "skills", "skills-seed-skills"))
	case "codex":
		return filepath.ToSlash(filepath.Join(".agents", "skills", "skills-seed-skills"))
	default:
		return filepath.ToSlash(filepath.Join(".skills", "skills-seed-skills"))
	}
}

// DefaultExcludePatterns 返回初始化配置写入的全局排除规则。
// 这些规则覆盖依赖目录、构建产物、缓存、临时文件和常见二进制资源，避免学习阶段把噪声纳入分析。
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

// LearnBuiltinExcludePatterns 返回学习阶段必须排除的内置目录，防止分析 Git 元数据和已生成的技能产物。
func LearnBuiltinExcludePatterns() []string {
	return []string{".git/**", ".skills-seed/**", ".claude/**", ".agents/**"}
}

// LearnExcludePatterns 合并用户配置排除规则和生成的 Skills 输出目录，用于 learn/current 的文件选择。
func LearnExcludePatterns(configExcludes, generatedSkillDirs []string) []string {
	excludes := LearnBuiltinExcludePatterns()
	excludes = append(excludes, configExcludes...)
	for _, dir := range generatedSkillDirs {
		dir = strings.TrimSpace(filepath.ToSlash(dir))
		if dir != "" {
			excludes = append(excludes, dir+"/**")
		}
	}
	return excludes
}
