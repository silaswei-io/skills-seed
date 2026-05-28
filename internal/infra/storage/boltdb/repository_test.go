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

// setupTestDB 创建测试用的全新 PatternRepository。
// 数据库保存在测试结束后会被清理的临时目录中。
func setupTestDB(t *testing.T) *PatternRepository {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	patternRepo, err := NewPatternRepository(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() { patternRepo.Close() })
	return patternRepo
}

// newTestPattern 创建测试用的有效 Pattern。
func newTestPattern(id, name string, category domain.Category, confidence float64) *domain.Pattern {
	p := domain.NewPattern(id, name, category)
	p.Confidence = confidence
	p.Description = "Test pattern: " + name
	p.Rule = "Follow the " + name + " convention"
	p.SetExamples("good example", "bad example")
	return p
}

// ==================== PatternRepository 测试 ====================

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

	// 确认所有返回的模式都属于请求的分类。
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

	// 查找名称和分类都相同的模式。
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

	// 查找名称不同的模式。
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

	// 确认模式存在。
	got, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	require.NotNil(t, got)

	// 删除模式。
	err = repo.Delete(ctx, "p-001")
	require.NoError(t, err)

	// 确认模式已不存在。
	_, err = repo.Get(ctx, "p-001")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pattern not found")
}

func TestPatternRepository_Count(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	// 初始数量为 0。
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

	// 空 ID 的 Pattern 无效。
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

	// 初始状态下没有已分析提交。
	analyzed, err := repo.IsCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)
	assert.False(t, analyzed)

	commits, err := repo.GetAnalyzedCommits(ctx)
	require.NoError(t, err)
	assert.Empty(t, commits)

	// 标记一个提交为已分析。
	err = repo.MarkCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)

	// 此时该提交应为已分析。
	analyzed, err = repo.IsCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)
	assert.True(t, analyzed)

	// 标记另一个提交。
	err = repo.MarkCommitAnalyzed(ctx, "def456")
	require.NoError(t, err)

	commits, err = repo.GetAnalyzedCommits(ctx)
	require.NoError(t, err)
	assert.Len(t, commits, 2)
	assert.Contains(t, commits, "abc123")
	assert.Contains(t, commits, "def456")

	// 重复标记同一提交应保持幂等。
	err = repo.MarkCommitAnalyzed(ctx, "abc123")
	require.NoError(t, err)

	commits, err = repo.GetAnalyzedCommits(ctx)
	require.NoError(t, err)
	assert.Len(t, commits, 2) // 仍为 2，不应变成 3
}

func TestPatternRepository_FileAnalysisTracking(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	scope := domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}

	records := []domain.FileAnalysisRecord{
		{
			ProjectID:      "backend",
			ScopePath:      "backend",
			Path:           "internal/app.go",
			Hash:           "abc",
			HashAlgorithm:  domain.FileAnalysisHashMD5,
			Size:           12,
			ModTime:        "2026-05-26T00:00:00Z",
			Source:         domain.FileAnalysisSourceCurrentCode,
			LastAnalyzedAt: "2026-05-26T00:00:01Z",
		},
	}

	require.NoError(t, repo.SaveAnalyzedFiles(ctx, records))

	got, err := repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "abc", got.Hash)

	list, err := repo.ListAnalyzedFiles(ctx, scope)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, repo.DeleteAnalyzedFiles(ctx, scope, []string{"internal/app.go"}))
	got, err = repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPatternRepository_FileAnalysisTrackingScopesRecords(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, repo.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{
		{ProjectID: "backend", ScopePath: "backend", Path: "main.go", Hash: "backend", HashAlgorithm: domain.FileAnalysisHashMD5},
		{ProjectID: "frontend", ScopePath: "frontend", Path: "main.go", Hash: "frontend", HashAlgorithm: domain.FileAnalysisHashMD5},
	}))

	backend, err := repo.GetAnalyzedFile(ctx, domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}, "main.go")
	require.NoError(t, err)
	require.NotNil(t, backend)
	assert.Equal(t, "backend", backend.Hash)

	frontend, err := repo.GetAnalyzedFile(ctx, domain.FileAnalysisScope{ProjectID: "frontend", ScopePath: "frontend"}, "main.go")
	require.NoError(t, err)
	require.NotNil(t, frontend)
	assert.Equal(t, "frontend", frontend.Hash)
}
