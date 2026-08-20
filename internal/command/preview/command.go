package preview

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/spf13/cobra"
)

type filesOptions struct {
	mode       string
	focusPaths []string
}

type filesPreview struct {
	Mode             string
	Included         []string
	Deleted          []string
	SkippedDocuments int
	SkippedOther     int
}

func Cmd(cont *container.Container) *cobra.Command {
	previewCmd := &cobra.Command{
		Use:     "preview",
		Short:   i18n.Get("PreviewShort"),
		Long:    i18n.Get("PreviewLongDesc"),
		Example: i18n.Get("PreviewExample"),
	}
	previewCmd.AddCommand(filesCmd(cont))
	return previewCmd
}

func filesCmd(cont *container.Container) *cobra.Command {
	opts := filesOptions{mode: "full"}
	cmd := &cobra.Command{
		Use:     "files",
		Short:   i18n.Get("PreviewFilesShort"),
		Long:    i18n.Get("PreviewFilesLongDesc"),
		Example: i18n.Get("PreviewFilesExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			preview, err := buildFilesPreview(cmd.Context(), cont, opts)
			if err != nil {
				return err
			}
			reportPath, err := writeFilesPreviewReport(cont.SeedPath, preview)
			if err != nil {
				return err
			}
			return writeFilesPreviewPath(
				cmd.OutOrStdout(),
				previewReportDisplayPath(cont.Config.Project.RootPath, filepath.Dir(reportPath)),
				filepath.Base(reportPath),
			)
		},
	}
	cmd.Flags().StringVar(&opts.mode, "mode", opts.mode, i18n.Get("PreviewFilesFlagMode"))
	cmd.Flags().StringArrayVarP(&opts.focusPaths, "focus", "f", nil, i18n.Get("PreviewFilesFlagFocus"))
	return cmd
}

func buildFilesPreview(ctx context.Context, cont *container.Container, opts filesOptions) (*filesPreview, error) {
	projectRoot := cont.Config.Project.RootPath
	if strings.TrimSpace(projectRoot) == "" {
		projectRoot = filepath.Dir(cont.SeedPath)
	}
	focusAbsPaths := resolveFocusPaths(projectRoot, opts.focusPaths)
	mode := strings.ToLower(strings.TrimSpace(opts.mode))
	if mode == "" {
		mode = "full"
	}
	switch mode {
	case "full", "first":
		return buildFullFilesPreview(projectRoot, cont.ConfigRepo, focusAbsPaths)
	case "incremental", "current":
		changes, err := fileanalysis.PrepareCurrentChanges(ctx, cont.FileTracker, cont.ConfigRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, focusAbsPaths)
		if err != nil {
			return nil, err
		}
		preview := &filesPreview{
			Mode:     "incremental",
			Included: append([]string{}, changes.AddedOrModified...),
			Deleted:  append([]string{}, changes.Deleted...),
		}
		preview.SkippedDocuments = changes.SkippedCount(fileanalysis.SkipReasonDocument)
		preview.SkippedOther = len(changes.Skipped) - preview.SkippedDocuments
		sort.Strings(preview.Included)
		sort.Strings(preview.Deleted)
		return preview, nil
	default:
		return nil, fmt.Errorf("%s", i18n.GetWithParams("PreviewFilesUnsupportedMode", map[string]interface{}{"Mode": opts.mode}))
	}
}

func buildFullFilesPreview(projectRoot string, configRepo config.Reader, focusAbsPaths []string) (*filesPreview, error) {
	selection, err := fileanalysis.SelectFiles(fileanalysis.SelectOptions{
		Root:          projectRoot,
		Policy:        fileanalysis.NewConfiguredSelectionPolicy(configRepo, projectRoot),
		FocusAbsPaths: focusAbsPaths,
	})
	if err != nil {
		return nil, err
	}
	preview := &filesPreview{
		Mode:             "full",
		Included:         selection.Paths(),
		SkippedDocuments: selection.SkippedCount(fileanalysis.SkipReasonDocument),
		SkippedOther:     len(selection.Skipped) - selection.SkippedCount(fileanalysis.SkipReasonDocument),
	}
	sort.Strings(preview.Included)
	sort.Strings(preview.Deleted)
	return preview, nil
}

func writeFilesPreviewReport(seedPath string, preview *filesPreview) (string, error) {
	if preview == nil {
		return "", nil
	}
	runtimeDir := layout.New(seedPath).Runtime("preview", "files")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return "", err
	}
	reportName := runtimefiles.Name("preview-files", preview.Mode) + ".md"
	reportPath := filepath.Join(runtimeDir, reportName)
	file, err := os.Create(reportPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err := writeFilesPreview(file, preview); err != nil {
		return "", err
	}
	return reportPath, nil
}

func writeFilesPreviewPath(w io.Writer, dir, file string) error {
	if strings.TrimSpace(dir) == "" && strings.TrimSpace(file) == "" {
		return nil
	}
	_, err := fmt.Fprintln(w, i18n.GetWithParams("PreviewFilesOutputSavedReport", map[string]interface{}{"Dir": dir, "File": file}))
	return err
}

func previewReportDisplayPath(projectRoot, reportPath string) string {
	if strings.TrimSpace(reportPath) == "" {
		return ""
	}
	if strings.TrimSpace(projectRoot) != "" {
		if rel, err := filepath.Rel(projectRoot, reportPath); err == nil {
			return rel
		}
	}
	return reportPath
}

func writeFilesPreview(w io.Writer, preview *filesPreview) error {
	if preview == nil {
		return nil
	}
	if _, err := fmt.Fprintf(w, "# %s\n\n", i18n.Get("PreviewFilesReportTitle")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "> %s\n\n", i18n.Get("PreviewFilesReportDisposableNote")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "## "+i18n.Get("PreviewFilesReportSummary")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| %s | %s |\n", i18n.Get("PreviewFilesOutputMode"), preview.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| %s | %d |\n", i18n.Get("PreviewFilesOutputIncluded"), len(preview.Included)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| %s | %d |\n", i18n.Get("PreviewFilesOutputDeleted"), len(preview.Deleted)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| %s | %d |\n", i18n.Get("PreviewFilesOutputSkippedDocuments"), preview.SkippedDocuments); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| %s | %d |\n\n", i18n.Get("PreviewFilesOutputSkippedOther"), preview.SkippedOther); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "## "+i18n.Get("PreviewFilesReportIncludedTree")); err != nil {
		return err
	}
	if err := writeFilesPreviewTree(w, preview.Included); err != nil {
		return err
	}
	if len(preview.Deleted) > 0 {
		if _, err := fmt.Fprintf(w, "\n## %s\n\n", i18n.Get("PreviewFilesReportDeletedTree")); err != nil {
			return err
		}
		if err := writeFilesPreviewTree(w, preview.Deleted); err != nil {
			return err
		}
	}
	return nil
}

func writeFilesPreviewTree(w io.Writer, paths []string) error {
	if _, err := fmt.Fprintln(w, "```text"); err != nil {
		return err
	}
	defer func() {
		_, _ = fmt.Fprintln(w, "```")
	}()
	if len(paths) == 0 {
		_, err := fmt.Fprintln(w, i18n.Get("PreviewFilesOutputNone"))
		return err
	}
	tree := buildFilesPreviewTree(paths)
	return writeFilesPreviewTreeNode(w, tree, "", true)
}

type previewTreeNode struct {
	name     string
	dir      bool
	children map[string]*previewTreeNode
}

func buildFilesPreviewTree(paths []string) *previewTreeNode {
	root := &previewTreeNode{dir: true, children: map[string]*previewTreeNode{}}
	for _, raw := range paths {
		path := strings.TrimSpace(filepath.ToSlash(raw))
		if path == "" {
			continue
		}
		parts := strings.Split(path, "/")
		node := root
		for i, part := range parts {
			if part == "" {
				continue
			}
			child := node.children[part]
			if child == nil {
				child = &previewTreeNode{name: part, children: map[string]*previewTreeNode{}}
				node.children[part] = child
			}
			if i < len(parts)-1 {
				child.dir = true
			}
			node = child
		}
	}
	return root
}

func writeFilesPreviewTreeNode(w io.Writer, node *previewTreeNode, prefix string, root bool) error {
	if root {
		if _, err := fmt.Fprintln(w, "."); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := node.children[names[i]]
		right := node.children[names[j]]
		if left.dir != right.dir {
			return left.dir
		}
		return names[i] < names[j]
	})
	for i, name := range names {
		child := node.children[name]
		last := i == len(names)-1
		connector := "├── "
		nextPrefix := prefix + "│   "
		if last {
			connector = "└── "
			nextPrefix = prefix + "    "
		}
		label := child.name
		if child.dir {
			label += "/"
		}
		if _, err := fmt.Fprintf(w, "%s%s%s\n", prefix, connector, label); err != nil {
			return err
		}
		if len(child.children) > 0 {
			if err := writeFilesPreviewTreeNode(w, child, nextPrefix, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveFocusPaths(projectRoot string, paths []string) []string {
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if filepath.IsAbs(path) {
			resolved = append(resolved, path)
			continue
		}
		resolved = append(resolved, filepath.Join(projectRoot, filepath.FromSlash(path)))
	}
	return resolved
}
