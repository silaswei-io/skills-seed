package prompts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestEnsureProjectPromptsWritesContextTemplateHashMetadata(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "en-US",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(seedPath, "context", "project.md"))
	require.NoError(t, err)
	text := string(content)

	require.Contains(t, text, "generated-by: skills-seed "+metadata.ProgramVersion)
	require.Contains(t, text, "prompt-template-sha256:")
	require.NotContains(t, text, metadata.UnavailableHash)
}

func TestEnsureProjectPromptsCreatesContextFilesOnly(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	for _, name := range []string{"README.md", "project.md", "rules.md", "glossary.md"} {
		require.FileExists(t, filepath.Join(seedPath, "context", name))
	}
	_, err = os.Stat(filepath.Join(seedPath, "prompts"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsureProjectPromptsRemovesEmptyDeprecatedInstructions(t *testing.T) {
	seedPath := t.TempDir()
	instructionsDir := filepath.Join(seedPath, "prompts", "instructions")
	require.NoError(t, os.MkdirAll(instructionsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(instructionsDir, "skill-project-summary.md"), []byte(`<!-- generated-by: skills-seed v0.9.0 -->
<!-- prompt-type: skill-project-summary -->
<!--
old scaffold
-->
`), 0644))

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

func TestEnsureProjectPromptsUsesSkillsLocaleForContextFiles(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName:  "demo",
		Language:     "go",
		Locale:       "zh-CN",
		SkillsLocale: "en-US",
	})
	require.NoError(t, err)

	project, err := os.ReadFile(filepath.Join(seedPath, "context", "project.md"))
	require.NoError(t, err)
	require.Contains(t, string(project), "# Project Background")
	require.NotContains(t, string(project), "# 项目背景")

	rules, err := os.ReadFile(filepath.Join(seedPath, "context", "rules.md"))
	require.NoError(t, err)
	require.Contains(t, string(rules), "# Team Rules")
	require.NotContains(t, string(rules), "# 团队规则")
}

func TestEnsureProjectPromptsWritesContextWithoutAnalysisCommands(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectPrompts(seedPath, ProjectPromptData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(seedPath, "context", "project.md"))
	require.NoError(t, err)
	text := string(content)

	require.NotContains(t, text, "请分析")
	require.NotContains(t, text, "分析目标")
	require.NotContains(t, text, "输出目标")
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
	require.NotContains(t, profile, "<user-context-path>")
	require.NotContains(t, profile, "workspace-input.json")
	require.NotContains(t, profile, "workspace-profile.json")
	require.NotContains(t, profile, "user-context.md")
	require.NotContains(t, profile, "hsmwebapi 是主后端")
	require.NotContains(t, spec, "<workspace-input-file>")
	require.NotContains(t, spec, "<workspace-profile-file>")
	require.NotContains(t, spec, "<user-context-path>")
	require.NotContains(t, spec, "workspace-input.json")
	require.NotContains(t, spec, "workspace-profile.json")
	require.NotContains(t, spec, "user-context.md")
	require.NotContains(t, spec, "kmip-go 提供 KMIP 能力")
}
