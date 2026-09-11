package hook

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestPreCommitHookDelegatesToHookRun(t *testing.T) {
	content, err := preCommitHookContent()
	require.NoError(t, err)

	require.Contains(t, content, "skills-seed hook run")
	require.NotContains(t, content, "skills-seed check")
}

func TestHookCommandMetadataAndActionCommand(t *testing.T) {
	cmd := Cmd()
	require.Equal(t, "hook", cmd.Use)
	require.Len(t, cmd.Commands(), 3)

	var output bytes.Buffer
	action := hookActionCmd("test", "short", "long", "example", func(*cobra.Command) error {
		return nil
	}, "complete")
	action.SetOut(&output)
	require.NoError(t, action.Execute())
	require.Contains(t, output.String(), "complete")

	wantErr := errors.New("action failed")
	action = hookActionCmd("test", "", "", "", func(*cobra.Command) error { return wantErr }, "")
	require.ErrorIs(t, action.Execute(), wantErr)
}

func TestInstallAndUninstallHook(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	require.Error(t, installHook())
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.Error(t, installHook())
	require.NoError(t, os.Mkdir(filepath.Join(root, ".skills-seed"), 0o755))
	require.NoError(t, installHook())

	hookPath := filepath.Join(root, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "skills-seed hook run")
	info, err := os.Stat(hookPath)
	require.NoError(t, err)
	require.NotZero(t, info.Mode().Perm()&0o100)

	require.NoError(t, uninstallHook())
	require.NoFileExists(t, hookPath)
	require.Error(t, uninstallHook())
}

func TestRunPreCommitHookSkipsInNonInteractiveEnvironment(t *testing.T) {
	if interactive.IsTerminal() {
		t.Skip("测试需要非交互式标准输入输出")
	}
	var output bytes.Buffer
	require.NoError(t, runPreCommitHook(&output, &output))
	require.NotEmpty(t, output.String())
}

func TestRunHookAction(t *testing.T) {
	require.NoError(t, runHookAction(hookActionSkip, &bytes.Buffer{}, &bytes.Buffer{}))
	t.Setenv("PATH", t.TempDir())
	require.Error(t, runHookAction(hookActionSync, &bytes.Buffer{}, &bytes.Buffer{}))
}

func TestHookActionArgs(t *testing.T) {
	require.Equal(t, []string{"sync"}, hookActionArgs(hookActionSync))
	require.Equal(t, []string{"learn", "current"}, hookActionArgs(hookActionLearn))
	require.Nil(t, hookActionArgs(hookActionSkip))
}
