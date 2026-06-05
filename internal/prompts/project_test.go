package prompts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestEnsureProjectPromptsWritesTemplateHashMetadata(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "en-US",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(seedPath, "prompts", "project", "project-profile.md"))
	require.NoError(t, err)
	text := string(content)

	require.Contains(t, text, "generated-by: skills-seed "+metadata.ProgramVersion)
	require.Contains(t, text, "prompt-template-sha256:")
	require.NotContains(t, text, metadata.UnavailableHash)
}

func TestEnsureProjectPromptsUsesCommonProjectPrompt(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(seedPath, "prompts", "system"))
	require.ErrorIs(t, err, os.ErrNotExist)

	commonPath := filepath.Join(seedPath, "prompts", "project", "common.md")
	commonContent, err := os.ReadFile(commonPath)
	require.NoError(t, err)
	require.Contains(t, string(commonContent), "prompt-type: common")

	_, err = os.Stat(filepath.Join(seedPath, "prompts", "project", "learn-analyze.md"))
	require.ErrorIs(t, err, os.ErrNotExist)

	_, err = os.Stat(filepath.Join(seedPath, "prompts", "instructions", "learn-analyze.md"))
	require.NoError(t, err)
}

func TestEnsureProjectPromptsUsesSkillsLocaleForPersistedSkillPrompts(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName:  "demo",
		Language:     "go",
		Locale:       "zh-CN",
		SkillsLocale: "en-US",
	})
	require.NoError(t, err)

	skillsPrompt, err := os.ReadFile(filepath.Join(seedPath, "prompts", "instructions", "learn-batch.md"))
	require.NoError(t, err)
	require.Contains(t, string(skillsPrompt), "# User Instructions")
	require.NotContains(t, string(skillsPrompt), "# 用户补充指令")

	toolPrompt, err := os.ReadFile(filepath.Join(seedPath, "prompts", "instructions", "fix-generate.md"))
	require.NoError(t, err)
	require.Contains(t, string(toolPrompt), "# User Instructions")
	require.NotContains(t, string(toolPrompt), "# 用户补充指令")
}

func TestEnsureProjectPromptsUsesSkillsLocaleForAllPromptFiles(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName:  "demo",
		Language:     "go",
		Locale:       "zh-CN",
		SkillsLocale: "en-US",
	})
	require.NoError(t, err)

	profile, err := os.ReadFile(filepath.Join(seedPath, "prompts", "project", "project-profile.md"))
	require.NoError(t, err)
	require.Contains(t, string(profile), "# Project Profile")
	require.NotContains(t, string(profile), "# 项目画像")

	common, err := os.ReadFile(filepath.Join(seedPath, "prompts", "project", "common.md"))
	require.NoError(t, err)
	require.Contains(t, string(common), "# Project-Specific Constraints")
	require.NotContains(t, string(common), "# 项目专属约束")

	fixInstructions, err := os.ReadFile(filepath.Join(seedPath, "prompts", "instructions", "fix-generate.md"))
	require.NoError(t, err)
	require.Contains(t, string(fixInstructions), "# User Instructions")
	require.NotContains(t, string(fixInstructions), "# 用户补充指令")
}

func TestEnsureProjectPromptsWritesContextProfileWithoutAnalysisCommands(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(seedPath, "prompts", "project", "project-profile.md"))
	require.NoError(t, err)
	text := string(content)

	require.Contains(t, text, "未记录")
	require.NotContains(t, text, "请补充")
	require.NotContains(t, text, "请分析")
	require.NotContains(t, text, "分析目标")
	require.NotContains(t, text, "输出目标")
}
