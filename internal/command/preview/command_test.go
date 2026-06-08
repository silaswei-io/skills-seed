package preview

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestBuildFullFilesPreviewSkipsDocumentsButKeepsDocsSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "examples"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.MD"), []byte("# readme\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "Guide.MD"), []byte("# guide\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "examples", "demo.go"), []byte("package examples\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "generated"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "generated", "wire.go"), []byte("package generated\n"), 0o644))

	configRepo, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Project.RootPath = root
	cfg.Exclude = append(cfg.Exclude, "internal/generated/**")
	require.NoError(t, configRepo.Update(cfg))

	preview, err := buildFilesPreview(context.Background(), &container.Container{
		SeedPath:   filepath.Join(root, ".skills-seed"),
		Config:     cfg,
		ConfigRepo: configRepo,
	}, filesOptions{mode: "full"})

	require.NoError(t, err)
	require.ElementsMatch(t, []string{"docs/examples/demo.go", "main.go"}, preview.Included)
	require.Equal(t, 2, preview.SkippedDocuments)
}

func TestWriteFilesPreview(t *testing.T) {
	var buf bytes.Buffer
	err := writeFilesPreview(&buf, &filesPreview{
		Mode:             "full",
		Included:         []string{"main.go"},
		SkippedDocuments: 2,
	}, 10)

	require.NoError(t, err)
	require.Contains(t, buf.String(), "included")
	require.Contains(t, buf.String(), "main.go")
	require.Contains(t, buf.String(), "skipped_documents")
}
