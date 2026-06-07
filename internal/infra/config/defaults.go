package config

import (
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
)

const (
	DefaultToolLocale   = i18n.DefaultLocale
	DefaultSkillsLocale = i18n.LocaleEnglish
)

// IsSupportedLocale reports whether a CLI/config locale value is supported.
// Empty is accepted so callers can apply the appropriate default later.
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

// TemplateLocaleSuffix keeps zh-CN templates as the unsuffixed embedded default.
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

func LearnBuiltinExcludePatterns() []string {
	return []string{".git/**", ".skills-seed/**", ".claude/**", ".agents/**"}
}

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
