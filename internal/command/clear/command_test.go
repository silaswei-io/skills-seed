package clear

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestRuntimeClearDryRunDoesNotDeleteRuntime(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	seedPath := t.TempDir()
	repo := createClearTestConfig(t, seedPath)
	prepareClearRuntimeFixture(t, seedPath)
	cont := &container.Container{SeedPath: seedPath, ConfigRepo: repo}

	cmd := Cmd(cont)
	cmd.SetArgs([]string{"runtime", "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "将要清理 runtime 目录")
	require.DirExists(t, filepath.Join(seedPath, "runtime"))
	require.FileExists(t, filepath.Join(seedPath, "runtime", "logs", "runtime.log"))
	require.FileExists(t, filepath.Join(seedPath, "store", "keep.txt"))
}

func TestRuntimeClearDeletesOnlyRuntime(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))

	seedPath := t.TempDir()
	repo := createClearTestConfig(t, seedPath)
	prepareClearRuntimeFixture(t, seedPath)
	cont := &container.Container{SeedPath: seedPath, ConfigRepo: repo}

	cmd := Cmd(cont)
	cmd.SetArgs([]string{"runtime", "--force"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "runtime 目录已清理")
	_, err := os.Stat(filepath.Join(seedPath, "runtime", "logs", "runtime.log"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err) || strings.Contains(err.Error(), "no such file or directory"))
	require.DirExists(t, filepath.Join(seedPath, "store"))
	require.FileExists(t, filepath.Join(seedPath, "store", "keep.txt"))
}

func createClearTestConfig(t *testing.T, seedPath string) *config.Repository {
	t.Helper()

	repo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	cfg := repo.Get()
	cfg.Project.RootPath = filepath.Dir(seedPath)
	cfg.Project.Name = "demo"
	cfg.Project.Language = "go"
	require.NoError(t, repo.Update(cfg))
	return repo
}

func prepareClearRuntimeFixture(t *testing.T, seedPath string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "runtime", "logs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "runtime", "logs", "runtime.log"), []byte("runtime"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "store"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "store", "keep.txt"), []byte("keep"), 0o644))
}
