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

	_, err = os.Stat(filepath.Join(seedPath, "prompts", "project", "learn-analyze.project.md"))
	require.ErrorIs(t, err, os.ErrNotExist)

	_, err = os.Stat(filepath.Join(seedPath, "prompts", "custom", "learn-analyze.override.md"))
	require.NoError(t, err)
}
