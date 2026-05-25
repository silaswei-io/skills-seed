package boltdb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a fresh PatternRepository for testing
// The database is stored in a temporary directory that is cleaned up after the test
func setupTestDB(t *testing.T) *PatternRepository {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	patternRepo, err := NewPatternRepository(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() { patternRepo.Close() })
	return patternRepo
}

// newTestPattern creates a valid Pattern for testing
func newTestPattern(id, name string, category domain.Category, confidence float64) *domain.Pattern {
	p := domain.NewPattern(id, name, category)
	p.Confidence = confidence
	p.Description = "Test pattern: " + name
	p.Rule = "Follow the " + name + " convention"
	p.SetExamples("good example", "bad example")
	return p
}

// ==================== PatternRepository tests ====================

func TestNewPatternRepository(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := NewPatternRepository(dbPath)
	require.NoError(t, err)
	require.NotNil(t, repo)
	t.Cleanup(func() { repo.Close() })
}

func TestPatternRepository_SaveAndGet(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	original := newTestPattern("p-001", "camelCase naming", domain.CategoryNaming, 0.9)
	err := repo.Save(ctx, original)
	require.NoError(t, err)

	got, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "p-001", got.ID)
	assert.Equal(t, "camelCase naming", got.Name)
	assert.Equal(t, domain.CategoryNaming, got.Category)
	assert.Equal(t, "Test pattern: camelCase naming", got.Description)
	assert.Equal(t, "good example", got.GoodExample)
	assert.Equal(t, "bad example", got.BadExample)
	assert.Equal(t, "Follow the camelCase naming convention", got.Rule)
	assert.Equal(t, 0.9, got.Confidence)
	assert.WithinDuration(t, original.CreatedAt, got.CreatedAt, time.Second)
}

func TestPatternRepository_Get_NotFound(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pattern not found")
}

func TestPatternRepository_GetAll(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p1 := newTestPattern("p-001", "pattern-1", domain.CategoryNaming, 0.9)
	p2 := newTestPattern("p-002", "pattern-2", domain.CategoryError, 0.8)
	p3 := newTestPattern("p-003", "pattern-3", domain.CategoryTesting, 0.7)

	require.NoError(t, repo.Save(ctx, p1))
	require.NoError(t, repo.Save(ctx, p2))
	require.NoError(t, repo.Save(ctx, p3))

	patterns, err := repo.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, patterns, 3)
}

func TestPatternRepository_GetAll_Empty(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	patterns, err := repo.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, patterns)
}

func TestPatternRepository_GetByCategory(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p1 := newTestPattern("p-001", "pattern-1", domain.CategoryNaming, 0.9)
	p2 := newTestPattern("p-002", "pattern-2", domain.CategoryNaming, 0.8)
	p3 := newTestPattern("p-003", "pattern-3", domain.CategoryError, 0.7)

	require.NoError(t, repo.Save(ctx, p1))
	require.NoError(t, repo.Save(ctx, p2))
	require.NoError(t, repo.Save(ctx, p3))

	patterns, err := repo.GetByCategory(ctx, domain.CategoryNaming)
	require.NoError(t, err)
	assert.Len(t, patterns, 2)

	// Verify all returned patterns belong to the requested category
	for _, p := range patterns {
		assert.Equal(t, domain.CategoryNaming, p.Category)
	}
}

func TestPatternRepository_GetByCategory_NoMatch(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p1 := newTestPattern("p-001", "pattern-1", domain.CategoryNaming, 0.9)
	require.NoError(t, repo.Save(ctx, p1))

	patterns, err := repo.GetByCategory(ctx, domain.CategoryDatabase)
	require.NoError(t, err)
	assert.Empty(t, patterns)
}

func TestPatternRepository_GetHighConfidence(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p1 := newTestPattern("p-001", "high-1", domain.CategoryNaming, 0.95)
	p2 := newTestPattern("p-002", "high-2", domain.CategoryError, 0.85)
	p3 := newTestPattern("p-003", "low-1", domain.CategoryTesting, 0.5)
	p4 := newTestPattern("p-004", "borderline", domain.CategoryStructure, 0.8)

	require.NoError(t, repo.Save(ctx, p1))
	require.NoError(t, repo.Save(ctx, p2))
	require.NoError(t, repo.Save(ctx, p3))
	require.NoError(t, repo.Save(ctx, p4))

	patterns, err := repo.GetHighConfidence(ctx, 0.8)
	require.NoError(t, err)
	assert.Len(t, patterns, 3)

	for _, p := range patterns {
		assert.GreaterOrEqual(t, p.Confidence, 0.8)
	}
}

func TestPatternRepository_FindSimilar(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	original := newTestPattern("p-001", "error-wrap", domain.CategoryError, 0.9)
	require.NoError(t, repo.Save(ctx, original))

	// Search for a pattern with the same name and category
	search := &domain.Pattern{
		Name:     "error-wrap",
		Category: domain.CategoryError,
	}

	found, err := repo.FindSimilar(ctx, search)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "p-001", found.ID)
	assert.Equal(t, "error-wrap", found.Name)
}

func TestPatternRepository_FindSimilar_NotFound(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p1 := newTestPattern("p-001", "error-wrap", domain.CategoryError, 0.9)
	require.NoError(t, repo.Save(ctx, p1))

	// Search for a pattern with a different name
	search := &domain.Pattern{
		Name:     "non-matching-name",
		Category: domain.CategoryError,
	}

	found, err := repo.FindSimilar(ctx, search)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPatternRepository_Delete(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p := newTestPattern("p-001", "to-delete", domain.CategoryNaming, 0.9)
	require.NoError(t, repo.Save(ctx, p))

	// Verify it exists
	got, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Delete it
	err = repo.Delete(ctx, "p-001")
	require.NoError(t, err)

	// Verify it no longer exists
	_, err = repo.Get(ctx, "p-001")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pattern not found")
}

func TestPatternRepository_Count(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	// Initially zero
	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	p1 := newTestPattern("p-001", "pattern-1", domain.CategoryNaming, 0.9)
	p2 := newTestPattern("p-002", "pattern-2", domain.CategoryError, 0.8)
	p3 := newTestPattern("p-003", "pattern-3", domain.CategoryTesting, 0.7)

	require.NoError(t, repo.Save(ctx, p1))
	require.NoError(t, repo.Save(ctx, p2))
	require.NoError(t, repo.Save(ctx, p3))

	count, err = repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestPatternRepository_Save_Invalid(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	// Pattern with empty ID is invalid
	invalidPattern := &domain.Pattern{
		ID:         "",
		Name:       "some-name",
		Category:   domain.CategoryNaming,
		Confidence: 0.5,
	}

	err := repo.Save(ctx, invalidPattern)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestPatternRepository_CommitTracking(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	// Initially no commits are analyzed
	analyzed, err := repo.IsCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)
	assert.False(t, analyzed)

	commits, err := repo.GetAnalyzedCommits(ctx)
	require.NoError(t, err)
	assert.Empty(t, commits)

	// Mark a commit as analyzed
	err = repo.MarkCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)

	// Now it should be analyzed
	analyzed, err = repo.IsCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)
	assert.True(t, analyzed)

	// Mark another commit
	err = repo.MarkCommitAnalyzed(ctx, "def456")
	require.NoError(t, err)

	commits, err = repo.GetAnalyzedCommits(ctx)
	require.NoError(t, err)
	assert.Len(t, commits, 2)
	assert.Contains(t, commits, "abc123")
	assert.Contains(t, commits, "def456")

	// Marking the same commit again should be idempotent
	err = repo.MarkCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)

	commits, err = repo.GetAnalyzedCommits(ctx)
	require.NoError(t, err)
	assert.Len(t, commits, 2) // Still 2, not 3
}
