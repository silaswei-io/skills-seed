package preview

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/spf13/cobra"
)

type filesOptions struct {
	mode       string
	focusPaths []string
	limit      int
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
		Short:   "Preview analysis inputs",
		Long:    "Preview files and prompt inputs that skills-seed would analyze without calling an AI agent.",
		Example: "skills-seed preview files\nskills-seed preview files --mode incremental --focus internal/service",
	}
	previewCmd.AddCommand(filesCmd(cont))
	return previewCmd
}

func filesCmd(cont *container.Container) *cobra.Command {
	opts := filesOptions{mode: "full", limit: 200}
	cmd := &cobra.Command{
		Use:     "files",
		Short:   "Preview files selected for analysis",
		Long:    "Preview source files selected for full or incremental analysis. Document files are skipped by default; source files under docs/ are kept.",
		Example: "skills-seed preview files --mode full\nskills-seed preview files --mode incremental --focus internal/service",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("skills-seed project is not initialized")
			}
			preview, err := buildFilesPreview(cmd.Context(), cont, opts)
			if err != nil {
				return err
			}
			return writeFilesPreview(cmd.OutOrStdout(), preview, opts.limit)
		},
	}
	cmd.Flags().StringVar(&opts.mode, "mode", opts.mode, "analysis mode: full or incremental")
	cmd.Flags().StringArrayVarP(&opts.focusPaths, "focus", "f", nil, "only preview files under these paths")
	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, "maximum included files to print")
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
		return preview, nil
	default:
		return nil, fmt.Errorf("unsupported preview mode %q", opts.mode)
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
	return preview, nil
}

func writeFilesPreview(w io.Writer, preview *filesPreview, limit int) error {
	if preview == nil {
		return nil
	}
	if limit <= 0 {
		limit = len(preview.Included)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "mode\t%s\n", preview.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "included\t%d\n", len(preview.Included)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "deleted\t%d\n", len(preview.Deleted)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "skipped_documents\t%d\n", preview.SkippedDocuments); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "skipped_other\t%d\n\n", preview.SkippedOther); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(tw, "INCLUDED_FILES"); err != nil {
		return err
	}
	for i, path := range preview.Included {
		if i >= limit {
			if _, err := fmt.Fprintf(tw, "... %d more\n", len(preview.Included)-limit); err != nil {
				return err
			}
			break
		}
		if _, err := fmt.Fprintln(tw, path); err != nil {
			return err
		}
	}
	if len(preview.Deleted) > 0 {
		if _, err := fmt.Fprintln(tw, "\nDELETED_FILES"); err != nil {
			return err
		}
		for _, path := range preview.Deleted {
			if _, err := fmt.Fprintln(tw, path); err != nil {
				return err
			}
		}
	}
	return tw.Flush()
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
