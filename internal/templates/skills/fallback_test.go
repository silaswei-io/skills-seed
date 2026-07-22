package skills

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestProjectSkillProvidersResolveThroughCommonTemplate(t *testing.T) {
	entry, ok := TemplateCatalogEntry("project-skill")
	require.True(t, ok)

	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			loader := NewLoaderForAgent(provider, "zh-CN")
			path, err := loader.catalogTemplatePath(entry)
			require.NoError(t, err)
			require.Equal(t, metadata.SkillsTemplatePath(metadata.CommonTemplateProvider, entry.RelativeName, "zh-CN", entry.Ext), path)

			enLoader := NewLoaderForAgent(provider, "en-US")
			enPath, err := enLoader.catalogTemplatePath(entry)
			require.NoError(t, err)
			require.Equal(t, metadata.SkillsTemplatePath(metadata.CommonTemplateProvider, entry.RelativeName, "en-US", entry.Ext), enPath)
		})
	}
}
