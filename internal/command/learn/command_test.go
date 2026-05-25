package learn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestCmd_HistoryDefaultsUseLearningConfig(t *testing.T) {
	repo, err := config.NewRepository(t.TempDir(), "zh-CN")
	require.NoError(t, err)

	cfg := repo.Get()
	cfg.Learning.MaxCommits = 7
	cfg.Learning.BatchSize = 3
	require.NoError(t, repo.Update(cfg))

	cmd := Cmd(&container.Container{ConfigRepo: repo})
	historyCmd, _, err := cmd.Find([]string{"history"})
	require.NoError(t, err)

	limitFlag := historyCmd.Flags().Lookup("limit")
	require.NotNil(t, limitFlag)
	require.Equal(t, "7", limitFlag.DefValue)

	batchFlag := historyCmd.Flags().Lookup("batch-size")
	require.NotNil(t, batchFlag)
	require.Equal(t, "3", batchFlag.DefValue)
}

func TestCmd_CurrentIncludesFocusAndProfileFlags(t *testing.T) {
	cmd := Cmd(&container.Container{})
	currentCmd, _, err := cmd.Find([]string{"current"})
	require.NoError(t, err)

	focusFlag := currentCmd.Flags().Lookup("focus")
	require.NotNil(t, focusFlag)
	require.Equal(t, "f", focusFlag.Shorthand)

	profileFlag := currentCmd.Flags().Lookup("profile")
	require.NotNil(t, profileFlag)
	require.Equal(t, learnCurrentProfileAuto, profileFlag.DefValue)
}

func TestResolveFocusPaths(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "internal", "agent"), 0755))

	paths, err := resolveFocusPaths(projectRoot, []string{"internal/agent"})
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(projectRoot, "internal", "agent")}, paths)

	_, err = resolveFocusPaths(projectRoot, []string{"../outside"})
	require.Error(t, err)
}

func TestShouldRefreshProfile(t *testing.T) {
	projectRoot := t.TempDir()

	tests := []struct {
		name          string
		focusPaths    []string
		mode          string
		profileExists bool
		want          bool
		wantErr       bool
	}{
		{name: "full scan refreshes existing profile", mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "missing profile refreshes scoped scan", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileAuto, profileExists: false, want: true},
		{name: "narrow focus skips existing profile", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileAuto, profileExists: true, want: false},
		{name: "root focus refreshes", focusPaths: []string{projectRoot}, mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "critical focus refreshes", focusPaths: []string{filepath.Join(projectRoot, "internal", "domain")}, mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "multiple focus modules refresh", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent"), filepath.Join(projectRoot, "internal", "prompts")}, mode: learnCurrentProfileAuto, profileExists: true, want: true},
		{name: "skip mode skips", mode: learnCurrentProfileSkip, profileExists: false, want: false},
		{name: "refresh mode refreshes", focusPaths: []string{filepath.Join(projectRoot, "internal", "agent")}, mode: learnCurrentProfileRefresh, profileExists: true, want: true},
		{name: "invalid mode fails", mode: "later", profileExists: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shouldRefreshProfile(projectRoot, tt.focusPaths, tt.mode, tt.profileExists)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
