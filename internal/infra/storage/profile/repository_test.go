package profile

import (
	"context"
	"errors"
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
