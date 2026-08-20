package preview

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
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
	cfg.Exclude.Paths = append(cfg.Exclude.Paths, "internal/generated/**")
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

func TestWriteFilesPreviewReport(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	root := t.TempDir()
	seedPath := filepath.Join(root, ".skills-seed")
	reportPath, err := writeFilesPreviewReport(seedPath, &filesPreview{
		Mode:             "full",
		Included:         []string{"main.go", "docs/examples/demo.go"},
		Deleted:          []string{"old.go"},
		SkippedDocuments: 2,
	})

	require.NoError(t, err)
	require.FileExists(t, reportPath)
	require.Contains(t, filepath.ToSlash(reportPath), "/.skills-seed/runtime/preview/files/")

	var buf bytes.Buffer
	err = writeFilesPreviewPath(&buf, previewReportDisplayPath(root, filepath.Dir(reportPath)), filepath.Base(reportPath))
	require.NoError(t, err)
	require.Contains(t, buf.String(), ".skills-seed/runtime/preview/files")
	require.Contains(t, buf.String(), filepath.Base(reportPath))
	require.Contains(t, buf.String(), "可删除")

	content, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "# 文件预览报告")
	require.Contains(t, text, "## 审查摘要")
	require.Contains(t, text, "| 模式 | full |")
	require.Contains(t, text, "| --- | --- |")
	require.Contains(t, text, "## 已包含文件树")
	require.Contains(t, text, "```text")
	require.Contains(t, text, "├── docs/")
	require.Contains(t, text, "└── main.go")
	require.Contains(t, text, "## 已删除文件树")
	require.Contains(t, text, "old.go")
	require.Contains(t, text, "可删除")
	require.NotContains(t, text, "included")
	require.NotContains(t, text, "skipped_documents")
}
