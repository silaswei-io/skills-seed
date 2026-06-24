package prompts

import (
	"os"
	"path/filepath"
	"strings"
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

func TestEnsureProjectPromptsRemovesEmptyDeprecatedInstructions(t *testing.T) {
	seedPath := t.TempDir()
	instructionsDir := filepath.Join(seedPath, "prompts", "instructions")
	require.NoError(t, os.MkdirAll(instructionsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(instructionsDir, "skill-project-summary.md"), []byte(strings.TrimSpace(`
<!-- generated-by: skills-seed v0.9.0 -->
<!-- prompt-type: skill-project-summary -->
<!--
old scaffold
-->
`)), 0644))

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	require.NoFileExists(t, filepath.Join(instructionsDir, "skill-project-summary.md"))
}

func TestEnsureProjectPromptsKeepsDeprecatedInstructionsWithBody(t *testing.T) {
	seedPath := t.TempDir()
	instructionsDir := filepath.Join(seedPath, "prompts", "instructions")
	require.NoError(t, os.MkdirAll(instructionsDir, 0755))
	path := filepath.Join(instructionsDir, "skill-project-summary.md")
	require.NoError(t, os.WriteFile(path, []byte("用户补充内容"), 0644))

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	require.FileExists(t, path)
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
	require.Contains(t, string(skillsPrompt), "Add user-confirmed project constraints")
	require.NotContains(t, string(skillsPrompt), "# 用户补充指令")

	toolPrompt, err := os.ReadFile(filepath.Join(seedPath, "prompts", "instructions", "fix-generate.md"))
	require.NoError(t, err)
	require.Contains(t, string(toolPrompt), "Add user-confirmed project constraints")
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
	require.Contains(t, string(common), "Add user constraints shared by all prompts here.")
	require.NotContains(t, string(common), "# 项目专属约束")

	fixInstructions, err := os.ReadFile(filepath.Join(seedPath, "prompts", "instructions", "fix-generate.md"))
	require.NoError(t, err)
	require.Contains(t, string(fixInstructions), "Add user-confirmed project constraints")
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

	require.NotContains(t, text, "未记录")
	require.NotContains(t, text, "请补充")
	require.NotContains(t, text, "请分析")
	require.NotContains(t, text, "分析目标")
	require.NotContains(t, text, "输出目标")
}

func TestEnsureProjectPromptsNormalizesNbspInStructureSummary(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
		Structure:   "demo\n\u00a0\u00a0cmd\n&nbsp;&nbsp;main.go   \n",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(seedPath, "prompts", "project", "project-profile.md"))
	require.NoError(t, err)
	text := string(content)

	require.Contains(t, text, "demo\n  cmd\n  main.go")
	require.NotContains(t, text, "\u00a0")
	require.NotContains(t, text, "&nbsp;")
	require.NotContains(t, text, "main.go   ")
}

func TestRenderWorkspacePromptsDoNotIncludeRuntimeInputFilePaths(t *testing.T) {
	data := WorkspacePromptData{
		WorkspaceName: "hsm-workspace",
		WorkspaceRoot: "/tmp/hsm-workspace",
		Projects: []WorkspacePromptProject{
			{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
		},
		Locale: "zh-CN",
	}

	profile, err := renderWorkspaceTemplate("skill-workspace-profile", "zh-CN", data)
	require.NoError(t, err)
	spec, err := renderWorkspaceTemplate("skill-workspace-spec", "zh-CN", data)
	require.NoError(t, err)

	require.Contains(t, profile, "`hsmwebapi`: `hsmwebapi`")
	require.NotContains(t, profile, "<workspace-input-file>")
	require.NotContains(t, profile, "<workspace-profile-file>")
	require.NotContains(t, profile, "<user-context-file>")
	require.NotContains(t, profile, "workspace-input.json")
	require.NotContains(t, profile, "workspace-profile.json")
	require.NotContains(t, profile, "user-context.md")
	require.NotContains(t, profile, "hsmwebapi 是主后端")
	require.NotContains(t, spec, "<workspace-input-file>")
	require.NotContains(t, spec, "<workspace-profile-file>")
	require.NotContains(t, spec, "<user-context-file>")
	require.NotContains(t, spec, "workspace-input.json")
	require.NotContains(t, spec, "workspace-profile.json")
	require.NotContains(t, spec, "user-context.md")
	require.NotContains(t, spec, "kmip-go 提供 KMIP 能力")
}
