package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_SaveAndGet(t *testing.T) {
	repo := NewRepository(t.TempDir())
	ctx := context.Background()

	err := repo.Save(ctx, &domain.ProjectProfile{
		ProjectName: "test-project",
		Language:    "go",
		Summary:     "A test project",
		GeneratedAt: "2026-05-19 12:00:00",
	})
	require.NoError(t, err)

	got, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test-project", got.ProjectName)
	assert.Equal(t, "go", got.Language)
	assert.Equal(t, "A test project", got.Summary)
}

func TestRepository_GetMissing(t *testing.T) {
	repo := NewRepository(t.TempDir())

	_, err := repo.Get(context.Background())
	assert.True(t, errors.Is(err, ErrProfileNotFound))
}

func TestRepository_SaveAndGetForProject(t *testing.T) {
	seedPath := t.TempDir()
	repo := NewRepository(seedPath)
	ctx := context.Background()

	err := repo.SaveForProject(ctx, "backend", &domain.ProjectProfile{
		ProjectName: "backend",
		Language:    "go",
		Summary:     "Backend service",
	})
	require.NoError(t, err)

	got, err := repo.GetForProject(ctx, "backend")
	require.NoError(t, err)
	assert.Equal(t, "backend", got.ProjectName)
	assert.Equal(t, "go", got.Language)

	data, err := os.ReadFile(filepath.Join(seedPath, "memory", "projects", "backend", "project-profile.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "\n  \"project_name\": \"backend\"")
	assert.Equal(t, byte('\n'), data[len(data)-1])
}

func TestRepository_SaveAndGetSpec(t *testing.T) {
	seedPath := t.TempDir()
	repo := NewRepository(seedPath)
	ctx := context.Background()

	spec := &domain.ProjectSpec{
		ProjectName: "test-project",
		Language:    "go",
		Summary:     "Use existing patterns",
	}
	require.NoError(t, repo.SaveSpec(ctx, spec))

	got, err := repo.GetSpec(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test-project", got.ProjectName)
	assert.Equal(t, "Use existing patterns", got.Summary)
}

func TestRepository_SaveNilProfileReturnsError(t *testing.T) {
	repo := NewRepository(t.TempDir())

	err := repo.Save(context.Background(), nil)
	require.Error(t, err)
	assert.NotEmpty(t, err.Error())
}

func TestRepository_GetMissingSpec(t *testing.T) {
	repo := NewRepository(t.TempDir())

	_, err := repo.GetSpec(context.Background())
	assert.True(t, errors.Is(err, ErrSpecNotFound))
}
