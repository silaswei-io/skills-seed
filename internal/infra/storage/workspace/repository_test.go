package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/stretchr/testify/require"
)

func TestProfileRepositoryLifecycle(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := NewProfileRepository(seedPath)

	_, err := repo.Get(context.Background())
	require.ErrorIs(t, err, ErrProfileNotFound)
	require.Error(t, repo.Save(context.Background(), nil))

	want := &domain.WorkspaceProfile{Name: "workspace", RootPath: "/repo", GeneratedAt: "2026-09-11"}
	require.NoError(t, repo.Save(context.Background(), want))
	got, err := repo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.FileExists(t, layout.New(seedPath).WorkspaceProfile())
}

func TestSpecRepositoryLifecycle(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	repo := NewSpecRepository(seedPath)

	_, err := repo.Get(context.Background())
	require.ErrorIs(t, err, ErrSpecNotFound)
	require.Error(t, repo.Save(context.Background(), nil))

	want := &domain.WorkspaceSpec{GeneratedAt: "2026-09-11", ChangeOrder: []string{"api", "web"}}
	require.NoError(t, repo.Save(context.Background(), want))
	got, err := repo.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.FileExists(t, layout.New(seedPath).WorkspaceSpec())
}

func TestRepositoriesPropagateContextAndParseErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	profileRepo := NewProfileRepository(filepath.Join(t.TempDir(), ".skills-seed"))
	_, err := profileRepo.Get(ctx)
	require.ErrorIs(t, err, context.Canceled)

	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	specRepo := NewSpecRepository(seedPath)
	path := layout.New(seedPath).WorkspaceSpec()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o644))
	_, err = specRepo.Get(context.Background())
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrSpecNotFound))
}
