// Package update 提供 Skills Seed CLI 的自更新命令。
package update

import (
	"context"
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/service/selfupdate"
	"github.com/silaswei-io/skills-seed/internal/terminal/progress"
	"github.com/spf13/cobra"
)

// Updater 定义 CLI 自更新需要的最小能力。
type Updater interface {
	Update(ctx context.Context, version string) (selfupdate.Result, error)
}

// ProgressUpdater 是能报告更新阶段的更新器。
type ProgressUpdater interface {
	Updater
	UpdateWithProgress(ctx context.Context, version string, report selfupdate.ProgressReporter) (selfupdate.Result, error)
}

const updateStepTotal = 4

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
			result, err := runUpdate(cmd.Context(), updater, strings.TrimSpace(version))
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

func runUpdate(ctx context.Context, updater Updater, version string) (selfupdate.Result, error) {
	progressUpdater, ok := updater.(ProgressUpdater)
	if !ok {
		return updater.Update(ctx, version)
	}

	tracker := progress.New(updateStepTotal)
	currentLabel := ""
	currentStage := selfupdate.Stage("")
	started := false
	report := func(event selfupdate.ProgressEvent) {
		label := updateStageLabel(event)
		if started && event.Stage == currentStage {
			tracker.UpdateStep(label)
			return
		}
		if started {
			tracker.CompleteStep(currentLabel)
		}
		currentLabel = label
		currentStage = event.Stage
		started = true
		tracker.StartStep(label)
	}
	result, err := progressUpdater.UpdateWithProgress(ctx, version, report)
	if !started {
		return result, err
	}
	if err != nil {
		tracker.FailStep(currentLabel)
		return result, err
	}
	tracker.CompleteStep(currentLabel)
	return result, nil
}

func updateStageLabel(event selfupdate.ProgressEvent) string {
	switch event.Stage {
	case selfupdate.StageResolveRelease:
		return i18n.Get("UpdateProgressResolveRelease")
	case selfupdate.StageDownloadAsset:
		if event.Downloaded > 0 {
			params := map[string]interface{}{
				"Downloaded": formatDownloadSize(event.Downloaded),
			}
			if event.Total > 0 {
				params["Total"] = formatDownloadSize(event.Total)
				params["Percent"] = event.Downloaded * 100 / event.Total
				return i18n.GetWithParams("UpdateProgressDownloadAssetDetail", params)
			}
			return i18n.GetWithParams("UpdateProgressDownloadAssetBytes", params)
		}
		return i18n.Get("UpdateProgressDownloadAsset")
	case selfupdate.StageVerifyAsset:
		return i18n.Get("UpdateProgressVerifyAsset")
	case selfupdate.StageInstallAsset:
		return i18n.Get("UpdateProgressInstallAsset")
	default:
		return i18n.Get("UpdateProgressResolveRelease")
	}
}

func formatDownloadSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := "B"
	for _, candidate := range units {
		value /= 1024
		unit = candidate
		if value < 1024 {
			break
		}
	}
	return fmt.Sprintf("%.1f %s", value, unit)
}
