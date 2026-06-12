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
	"github.com/silaswei-io/skills-seed/internal/i18n"
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
		Short:   i18n.Get("PreviewShort"),
		Long:    i18n.Get("PreviewLongDesc"),
		Example: i18n.Get("PreviewExample"),
	}
	previewCmd.AddCommand(filesCmd(cont))
	return previewCmd
}

func filesCmd(cont *container.Container) *cobra.Command {
	opts := filesOptions{mode: "full", limit: 200}
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
			return writeFilesPreview(cmd.OutOrStdout(), preview, opts.limit)
		},
	}
	cmd.Flags().StringVar(&opts.mode, "mode", opts.mode, i18n.Get("PreviewFilesFlagMode"))
	cmd.Flags().StringArrayVarP(&opts.focusPaths, "focus", "f", nil, i18n.Get("PreviewFilesFlagFocus"))
	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, i18n.Get("PreviewFilesFlagLimit"))
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
	if _, err := fmt.Fprintf(tw, "%s\t%s\n", i18n.Get("PreviewFilesOutputMode"), preview.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "%s\t%d\n", i18n.Get("PreviewFilesOutputIncluded"), len(preview.Included)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "%s\t%d\n", i18n.Get("PreviewFilesOutputDeleted"), len(preview.Deleted)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "%s\t%d\n", i18n.Get("PreviewFilesOutputSkippedDocuments"), preview.SkippedDocuments); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "%s\t%d\n\n", i18n.Get("PreviewFilesOutputSkippedOther"), preview.SkippedOther); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(tw, i18n.Get("PreviewFilesOutputIncludedFiles")); err != nil {
		return err
	}
	for i, path := range preview.Included {
		if i >= limit {
			if _, err := fmt.Fprintln(tw, i18n.GetWithParams("PreviewFilesOutputMore", map[string]interface{}{"Count": len(preview.Included) - limit})); err != nil {
				return err
			}
			break
		}
		if _, err := fmt.Fprintln(tw, path); err != nil {
			return err
		}
	}
	if len(preview.Deleted) > 0 {
		if _, err := fmt.Fprintf(tw, "\n%s\n", i18n.Get("PreviewFilesOutputDeletedFiles")); err != nil {
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
