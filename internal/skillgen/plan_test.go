package skillgen

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/stretchr/testify/require"
)

func TestPlanCollectsOperations(t *testing.T) {
	plan := NewPlan("output")
	require.Equal(t, "output", plan.OutputPath)

	data := map[string]string{"name": "demo"}
	plan.AddFile("SKILL.md", CatalogTemplate, "catalog", data)

	require.Equal(t, []File{{Path: "SKILL.md", Kind: CatalogTemplate, Template: "catalog", Data: data}}, plan.Files)
}

func TestRendererValidatesDependenciesPlanAndContext(t *testing.T) {
	plan := NewPlan(t.TempDir())
	require.Error(t, (*Renderer)(nil).Render(context.Background(), plan))
	require.Error(t, NewRenderer(nil).Render(context.Background(), plan))

	renderer := NewRenderer(skills.NewLoaderForAgent("codex", "en-US"))
	require.Error(t, renderer.Render(context.Background(), nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, renderer.Render(ctx, plan), context.Canceled)
}

func TestRendererCreatesDirectoriesForFiles(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "skill")
	require.NoError(t, os.MkdirAll(filepath.Join(outputPath, "obsolete"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outputPath, "obsolete", "file"), []byte("old"), 0o644))
	plan := NewPlan(outputPath)
	plan.AddFile("references/code-navigation.md", RelativeTemplate, "project/references/code-navigation", struct {
		Files, Unindexed   int
		AuthorityPaths     []string
		HasBusinessMethods bool
	}{})
	renderer := NewRenderer(skills.NewLoaderForAgent("codex", "en-US"))

	require.NoError(t, renderer.Render(context.Background(), plan))
	require.FileExists(t, filepath.Join(outputPath, "references", "code-navigation.md"))
	require.FileExists(t, filepath.Join(outputPath, "obsolete", "file"))
}

func TestRendererRejectsUnsupportedTemplateKind(t *testing.T) {
	renderer := NewRenderer(skills.NewLoaderForAgent("codex", "en-US"))
	plan := NewPlan(t.TempDir())
	plan.AddFile("SKILL.md", TemplateKind("unknown"), "", nil)

	require.Error(t, renderer.Render(context.Background(), plan))
}
