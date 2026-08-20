package clear

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/runtimeclean"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/spf13/cobra"
)

// Cmd 返回 clear 命令。
func Cmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clear",
		Short:   i18n.Get("ClearShort"),
		Long:    i18n.Get("ClearLongDesc"),
		Example: i18n.Get("ClearExample"),
	}
	cmd.AddCommand(runtimeCmd(cont))
	return cmd
}

func runtimeCmd(cont *container.Container) *cobra.Command {
	dryRun := false
	force := false
	cmd := &cobra.Command{
		Use:     "runtime",
		Short:   i18n.Get("ClearRuntimeShort"),
		Long:    i18n.Get("ClearRuntimeLongDesc"),
		Example: i18n.Get("ClearRuntimeExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			target := runtimeclean.Path(cont.SeedPath)
			if dryRun {
				return printRuntimeClearResult(cmd, "ClearRuntimeDryRun", target)
			}
			if !force && interactive.IsTerminal() {
				confirmed, err := interactive.Confirm(
					i18n.Get("ClearRuntimeConfirmTitle"),
					i18n.Get("InteractiveYes"),
					i18n.Get("InteractiveNo"),
					false,
				)
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
			}
			if err := runtimeclean.Clear(cont.SeedPath); err != nil {
				return err
			}
			return printRuntimeClearResult(cmd, "ClearRuntimeCompleted", target)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.Get("ClearRuntimeFlagDryRun"))
	cmd.Flags().BoolVar(&force, "force", false, i18n.Get("ClearRuntimeFlagForce"))
	return cmd
}

func printRuntimeClearResult(cmd *cobra.Command, key string, path string) error {
	displayPath := path
	if wd, err := os.Getwd(); err == nil {
		if rel, relErr := filepath.Rel(wd, path); relErr == nil && !strings.HasPrefix(rel, "..") {
			displayPath = rel
		}
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), i18n.GetWithParams(key, map[string]interface{}{
		"Path": displayPath,
	}))
	return err
}
