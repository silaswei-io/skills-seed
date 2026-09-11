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
	plan.AddDir("references")
	plan.RemovePath("obsolete.md")

	require.Equal(t, []File{{Path: "SKILL.md", Kind: CatalogTemplate, Template: "catalog", Data: data}}, plan.Files)
	require.Equal(t, []string{"references"}, plan.CreateDirs)
	require.Equal(t, []string{"obsolete.md"}, plan.RemovePaths)
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

func TestRendererCreatesAndRemovesPlanPaths(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "skill")
	require.NoError(t, os.MkdirAll(filepath.Join(outputPath, "obsolete"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outputPath, "obsolete", "file"), []byte("old"), 0o644))
	plan := NewPlan(outputPath)
	plan.AddDir("references")
	plan.RemovePath("obsolete")
	renderer := NewRenderer(skills.NewLoaderForAgent("codex", "en-US"))

	require.NoError(t, renderer.Render(context.Background(), plan))
	require.DirExists(t, filepath.Join(outputPath, "references"))
	require.NoDirExists(t, filepath.Join(outputPath, "obsolete"))
}

func TestRendererRejectsUnsupportedTemplateKind(t *testing.T) {
	renderer := NewRenderer(skills.NewLoaderForAgent("codex", "en-US"))
	plan := NewPlan(t.TempDir())
	plan.AddFile("SKILL.md", TemplateKind("unknown"), "", nil)

	require.Error(t, renderer.Render(context.Background(), plan))
}
