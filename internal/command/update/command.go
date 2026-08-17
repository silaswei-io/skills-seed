// Package update 提供 Skills Seed CLI 的自更新命令。
package update

import (
	"context"
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/service/selfupdate"
	"github.com/spf13/cobra"
)

// Updater 定义 CLI 自更新需要的最小能力。
type Updater interface {
	Update(ctx context.Context, version string) (selfupdate.Result, error)
}

// Cmd 创建自更新命令。
func Cmd() *cobra.Command {
	return NewCmd(selfupdate.New())
}

// NewCmd 使用指定更新器创建命令，便于隔离网络与文件系统测试。
func NewCmd(updater Updater) *cobra.Command {
	version := "latest"
	cmd := &cobra.Command{
		Use:     "update",
		Short:   i18n.Get("UpdateShort"),
		Long:    i18n.Get("UpdateLongDesc"),
		Example: i18n.Get("UpdateExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if updater == nil {
				return fmt.Errorf("%s", i18n.Get("UpdateUnavailable"))
			}
			result, err := updater.Update(cmd.Context(), strings.TrimSpace(version))
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.Get("UpdateFailed"), err)
			}
			messageKey := "UpdateCompleted"
			if result.RestartRequired {
				messageKey = "UpdateRestartRequired"
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), i18n.GetWithParams(messageKey, map[string]interface{}{
				"Version": result.Version,
				"Path":    result.ExecutablePath,
			}))
			return err
		},
	}
	cmd.Flags().StringVar(&version, "version", version, i18n.Get("UpdateFlagVersion"))
	return cmd
}
