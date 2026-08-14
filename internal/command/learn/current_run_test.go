package learn

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	gitinfra "github.com/silaswei-io/skills-seed/internal/infra/git"
	"github.com/stretchr/testify/require"
)

func TestNewLearnCurrentProjectRunPreservesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	run := newLearnCurrentProjectRun(ctx, &container.Container{SeedPath: t.TempDir()}, learnCurrentProjectOptions{})
	cancel()

	require.ErrorIs(t, run.ctx.Err(), context.Canceled)
}

func TestPrepareProjectReportsBrokenAuthoritySourceWithoutMislabelingCurrentDirectory(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	missing := filepath.Join(t.TempDir(), "missing-authority.md")
	if err := os.Symlink(missing, filepath.Join(projectRoot, "CLAUDE.md")); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	run := newLearnCurrentProjectRun(context.Background(), &container.Container{
		SeedPath:   seedPath,
		ConfigRepo: configRepo,
		GitRepo:    gitinfra.NewRepository(projectRoot),
	}, learnCurrentProjectOptions{})

	err = run.prepareProject()

	require.ErrorContains(t, err, "读取权威项目知识版本失败")
	require.ErrorContains(t, err, `resolve authority source "CLAUDE.md"`)
	require.NotContains(t, err.Error(), "获取当前目录失败")
}
