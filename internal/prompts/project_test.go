package prompts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestEnsureProjectContextWritesContextTemplateHashMetadata(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectContext(seedPath, ProjectContextData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "en-US",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(seedPath, "context", "background.md"))
	require.NoError(t, err)
	text := string(content)

	require.Contains(t, text, "generated-by: skills-seed "+metadata.ProgramVersion)
	require.Contains(t, text, "seed-template-sha256:")
	require.NotContains(t, text, metadata.UnavailableHash)
}

func TestEnsureProjectContextCreatesContextFilesOnly(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectContext(seedPath, ProjectContextData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	for _, name := range []string{"README.md", "background.md", "constraints.md", "terminology.md"} {
		require.FileExists(t, filepath.Join(seedPath, "context", name))
	}
	_, err = os.Stat(filepath.Join(seedPath, "prompts"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsureProjectContextRemovesDeprecatedPromptScaffold(t *testing.T) {
	seedPath := t.TempDir()
	instructionsDir := filepath.Join(seedPath, "prompts", "instructions")
	require.NoError(t, os.MkdirAll(instructionsDir, 0755))
	path := filepath.Join(instructionsDir, "skill-project-summary.md")
	require.NoError(t, os.WriteFile(path, []byte(`<!-- generated-by: skills-seed v0.9.0 -->
<!-- prompt-type: skill-project-summary -->
<!--
old scaffold
-->
`), 0644))

	err := EnsureProjectContext(seedPath, ProjectContextData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	require.NoFileExists(t, path)
	require.NoDirExists(t, filepath.Join(seedPath, "prompts"))
}

func TestEnsureProjectContextRemovesDeprecatedPromptBody(t *testing.T) {
	seedPath := t.TempDir()
	instructionsDir := filepath.Join(seedPath, "prompts", "instructions")
	require.NoError(t, os.MkdirAll(instructionsDir, 0755))
	path := filepath.Join(instructionsDir, "skill-project-summary.md")
	require.NoError(t, os.WriteFile(path, []byte("用户补充内容"), 0644))

	err := EnsureProjectContext(seedPath, ProjectContextData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	require.NoFileExists(t, path)
	require.NoDirExists(t, filepath.Join(seedPath, "prompts"))
}

func TestEnsureProjectContextRemovesDeprecatedContextFiles(t *testing.T) {
	seedPath := t.TempDir()
	contextDir := filepath.Join(seedPath, "context")
	require.NoError(t, os.MkdirAll(contextDir, 0755))
	for _, name := range []string{"project.md", "rules.md", "glossary.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, name), []byte("deprecated"), 0644))
	}

	err := EnsureProjectContext(seedPath, ProjectContextData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	for _, name := range []string{"project.md", "rules.md", "glossary.md"} {
		require.NoFileExists(t, filepath.Join(contextDir, name))
	}
	for _, name := range []string{"background.md", "constraints.md", "terminology.md"} {
		require.FileExists(t, filepath.Join(contextDir, name))
	}
}

func TestEnsureProjectContextUsesSkillsLocaleForContextFiles(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectContext(seedPath, ProjectContextData{
		ProjectName:  "demo",
		Language:     "go",
		Locale:       "zh-CN",
		SkillsLocale: "en-US",
	})
	require.NoError(t, err)

	project, err := os.ReadFile(filepath.Join(seedPath, "context", "background.md"))
	require.NoError(t, err)
	require.Contains(t, string(project), "# Background and External Facts")
	require.NotContains(t, string(project), "# 背景与外部事实")

	rules, err := os.ReadFile(filepath.Join(seedPath, "context", "constraints.md"))
	require.NoError(t, err)
	require.Contains(t, string(rules), "# Constraints and Boundaries")
	require.NotContains(t, string(rules), "# 约束与边界")
}

func TestEnsureProjectContextWritesContextWithoutAnalysisCommands(t *testing.T) {
	seedPath := t.TempDir()

	err := EnsureProjectContext(seedPath, ProjectContextData{
		ProjectName: "demo",
		Language:    "go",
		Locale:      "zh-CN",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(seedPath, "context", "background.md"))
	require.NoError(t, err)
	text := string(content)

	require.NotContains(t, text, "请分析")
	require.NotContains(t, text, "分析目标")
	require.NotContains(t, text, "输出目标")
}
