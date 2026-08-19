package update

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/service/selfupdate"
	"github.com/stretchr/testify/require"
)

type fakeUpdater struct {
	version string
	result  selfupdate.Result
	err     error
}

func (u *fakeUpdater) Update(_ context.Context, version string) (selfupdate.Result, error) {
	u.version = version
	return u.result, u.err
}

type fakeProgressUpdater struct {
	fakeUpdater
	progressVersion string
}

func (u *fakeProgressUpdater) UpdateWithProgress(_ context.Context, version string, report selfupdate.ProgressReporter) (selfupdate.Result, error) {
	u.progressVersion = version
	for _, stage := range []selfupdate.Stage{
		selfupdate.StageResolveRelease,
		selfupdate.StageDownloadAsset,
		selfupdate.StageVerifyAsset,
		selfupdate.StageInstallAsset,
	} {
		report(selfupdate.ProgressEvent{Stage: stage})
	}
	return u.result, u.err
}

func TestCmdUpdatesLatestByDefault(t *testing.T) {
	require.NoError(t, i18n.Init("en-US"))
	updater := &fakeUpdater{result: selfupdate.Result{Version: "v1.2.3", ExecutablePath: "/usr/local/bin/skills-seed"}}
	cmd := NewCmd(updater)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs(nil)

	require.NoError(t, cmd.Execute())
	require.Equal(t, "latest", updater.version)
	require.Contains(t, output.String(), "v1.2.3")
}

func TestCmdPassesRequestedVersion(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	updater := &fakeUpdater{result: selfupdate.Result{Version: "v1.2.3", ExecutablePath: "/usr/local/bin/skills-seed"}}
	cmd := NewCmd(updater)
	cmd.SetArgs([]string{"--version", "v1.2.3"})

	require.NoError(t, cmd.Execute())
	require.Equal(t, "v1.2.3", updater.version)
}

func TestCmdUsesProgressUpdaterWhenAvailable(t *testing.T) {
	require.NoError(t, i18n.Init("en-US"))
	updater := &fakeProgressUpdater{fakeUpdater: fakeUpdater{result: selfupdate.Result{Version: "v1.2.3"}}}
	cmd := NewCmd(updater)
	cmd.SetArgs([]string{"--version", "v1.2.3"})

	require.NoError(t, cmd.Execute())
	require.Equal(t, "v1.2.3", updater.progressVersion)
	require.Empty(t, updater.version)
}

func TestCmdReturnsLocalizedUpdateFailure(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	updater := &fakeUpdater{err: errors.New("network unavailable")}
	cmd := NewCmd(updater)

	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorContains(t, err, i18n.Get("UpdateFailed"))
}
