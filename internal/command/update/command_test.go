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

func TestCmdReturnsLocalizedUpdateFailure(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	updater := &fakeUpdater{err: errors.New("network unavailable")}
	cmd := NewCmd(updater)

	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorContains(t, err, i18n.Get("UpdateFailed"))
}
