package config

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/stretchr/testify/require"
)

func TestLocaleDefaultsAndNormalization(t *testing.T) {
	require.Equal(t, i18n.DefaultLocale, DefaultToolLocale)
	require.Equal(t, i18n.LocaleEnglish, DefaultSkillsLocale)
	require.True(t, DefaultSaveRenderedPrompts)
	require.False(t, DefaultAutoDeleteRenderedPrompts)
	require.False(t, DefaultIncludeSkillsSeedGeneratedNotice)
	require.Equal(t, i18n.LocaleChinese, NormalizeToolLocale(""))
	require.Equal(t, i18n.LocaleEnglish, NormalizeSkillsLocale(""))
	require.Equal(t, i18n.LocaleEnglish, NormalizeToolLocale("en-US"))
	require.Equal(t, i18n.LocaleChinese, NormalizeSkillsLocale("zh-CN"))
	require.True(t, IsSupportedLocale(""))
	require.True(t, IsSupportedLocale("zh-CN"))
	require.False(t, IsSupportedLocale("fr-FR"))
}

func TestDefaultSkillsPathForTarget(t *testing.T) {
	require.Equal(t, ".claude/skills/skills-seed-skills", DefaultSkillsPathForTarget("claude"))
	require.Equal(t, ".agents/skills/skills-seed-skills", DefaultSkillsPathForTarget("codex"))
	require.Equal(t, ".skills/skills-seed-skills", DefaultSkillsPathForTarget("custom"))
	require.Equal(t, ".skills/skills-seed-skills", DefaultSkillsPathForTarget(""))
}

func TestLearnExcludePatterns(t *testing.T) {
	excludes := LearnExcludePatterns([]string{"vendor/**"}, []string{".agents/skills/demo", ""})

	require.Contains(t, excludes, ".git/**")
	require.Contains(t, excludes, ".skills-seed/**")
	require.Contains(t, excludes, ".claude/**")
	require.Contains(t, excludes, ".agents/**")
	require.Contains(t, excludes, "vendor/**")
	require.Contains(t, excludes, ".agents/skills/demo/**")
}
