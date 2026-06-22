package log

import (
	"fmt"
	"io"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/changelog"
	"github.com/silaswei-io/skills-seed/internal/utils"
	"github.com/spf13/cobra"
)

// Cmd 返回 log 命令。
func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "log",
		Short:   i18n.Get("LogShort"),
		Long:    i18n.Get("LogLongDesc"),
		Example: i18n.Get("LogExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd)
		},
	}
}

func run(cmd *cobra.Command) error {
	seedPath, err := utils.GetSeedPath()
	if err != nil {
		return err
	}
	entries, err := changelog.Recent(seedPath, 0)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), i18n.GetWithParams("LogNoChanges", map[string]interface{}{"Path": changelog.Path(seedPath)}))
		return err
	}
	return printEntries(cmd.OutOrStdout(), entries)
}

func printEntries(out io.Writer, entries []changelog.Entry) error {
	for i, entry := range entries {
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "change %s\n", entry.ID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Date:    %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05")); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Command: %s\n\n", entry.Command); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "    %s\n", entry.Summary); err != nil {
			return err
		}
		for _, detail := range entry.Details {
			if detail == entry.Summary {
				continue
			}
			if _, err := fmt.Fprintf(out, "    - %s\n", detail); err != nil {
				return err
			}
		}
	}
	return nil
}
