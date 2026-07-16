package boltdb

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
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
	p.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/example.go", Line: 1, Kind: "file"}}
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
	original.Description = "Use ProjectConfig.Name from internal/infra/config/repository.go when building config defaults."
	original.Rule = "Config defaults must preserve project provider mappings."
	err := repo.Save(ctx, original)
	require.NoError(t, err)

	got, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "p-001", got.ID)
	assert.Equal(t, "camelCase naming", got.Name)
	assert.Equal(t, domain.CategoryNaming, got.Category)
	assert.Equal(t, "Use ProjectConfig.Name from internal/infra/config/repository.go when building config defaults.", got.Description)
	assert.Equal(t, "good example", got.GoodExample)
	assert.Equal(t, "bad example", got.BadExample)
	assert.Equal(t, "Config defaults must preserve project provider mappings.", got.Rule)
	assert.Equal(t, 0.9, got.Confidence)
	assert.Greater(t, got.Metrics.SpecificityScore, 0.0)
	assert.Greater(t, got.Metrics.EffectiveScore, 0.0)
	assert.WithinDuration(t, original.CreatedAt, got.CreatedAt, time.Second)
}

func TestPatternRepository_SaveNormalizesCategoryBeforeBucketWrite(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	pattern := newTestPattern("p-001", "error category normalization", domain.Category(" Error "), 0.9)
	require.NoError(t, repo.Save(ctx, pattern))

	got, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryError, got.Category)

	errorPatterns, err := repo.GetByCategory(ctx, domain.CategoryError)
	require.NoError(t, err)
	require.Len(t, errorPatterns, 1)
	require.Equal(t, "p-001", errorPatterns[0].ID)

	rawPatterns, err := repo.GetByCategory(ctx, domain.Category(" Error "))
	require.NoError(t, err)
	require.Len(t, rawPatterns, 1)
	require.Equal(t, domain.CategoryError, rawPatterns[0].Category)
}

func TestPatternRepository_SaveRejectsNilPattern(t *testing.T) {
	repo := setupTestDB(t)
	err := repo.Save(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid pattern")
}

func TestPatternRepository_ApplyPatternMutationValidatesBeforeDeleting(t *testing.T) {
	repo := setupTestDB(t)
	existing := newTestPattern("existing", "Existing", domain.CategoryError, 0.9)
	require.NoError(t, repo.Save(context.Background(), existing))

	err := repo.ApplyPatternMutation(context.Background(), domain.PatternMutation{
		DeleteIDs: []string{existing.ID},
		Save:      []*domain.Pattern{nil},
	})

	require.Error(t, err)
	got, getErr := repo.Get(context.Background(), existing.ID)
	require.NoError(t, getErr)
	require.Equal(t, existing.ID, got.ID)
}

func TestPatternRepository_SaveRemovesLegacyCategoryCopies(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	legacy := newTestPattern("p-001", "legacy security copy", domain.Category("security"), 0.8)
	require.NoError(t, repo.db.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)
		categoryBucket, err := mainBucket.CreateBucketIfNotExists([]byte("security"))
		if err != nil {
			return err
		}
		data, err := json.Marshal(legacy)
		if err != nil {
			return err
		}
		return categoryBucket.Put([]byte(legacy.ID), data)
	}))

	updated := newTestPattern("p-001", "canonical utils copy", domain.Category(" Security "), 0.9)
	require.NoError(t, repo.Save(ctx, updated))

	all, err := repo.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, domain.CategoryUtils, all[0].Category)

	utilsPatterns, err := repo.GetByCategory(ctx, domain.CategoryUtils)
	require.NoError(t, err)
	require.Len(t, utilsPatterns, 1)

	require.NoError(t, repo.db.View(func(tx *bolt.Tx) error {
		legacyBucket := tx.Bucket(bucketPatterns).Bucket([]byte("security"))
		require.NotNil(t, legacyBucket)
		require.Nil(t, legacyBucket.Get([]byte("p-001")))
		return nil
	}))
}

func TestPatternRepository_PreservesPatternCreatedAtOnUpdate(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	original := newTestPattern("p-001", "business service", domain.CategoryBusiness, 0.82)
	original.CreatedAt = time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	original.UpdatedAt = original.CreatedAt
	original.SetBusinessMethod(&domain.BusinessMethod{
		Name:         "CreateOrder",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/service/order.go:42"},
		Type:         "domain",
		Function:     "func (s *Service) CreateOrder(ctx context.Context, req CreateOrderReq) error",
	})
	require.NoError(t, repo.Save(ctx, original))

	saved, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	createdAt := saved.CreatedAt
	updatedAt := saved.UpdatedAt
	require.False(t, createdAt.IsZero())
	require.False(t, updatedAt.IsZero())
	require.Equal(t, "internal/service/order.go:42", saved.BusinessMethod.CodeLocation.HistoricalLocation)
	require.Equal(t, "internal/service/order.go:42", saved.BusinessMethod.CodeLocation.CurrentLocation)
	require.Equal(t, domain.CodeLocationStatusValid, saved.BusinessMethod.CodeLocation.Status)
	require.False(t, saved.BusinessMethod.CodeLocation.CreatedAt.IsZero())
	require.False(t, saved.BusinessMethod.CodeLocation.UpdatedAt.IsZero())
	locationCreatedAt := saved.BusinessMethod.CodeLocation.CreatedAt
	locationUpdatedAt := saved.BusinessMethod.CodeLocation.UpdatedAt

	time.Sleep(2 * time.Millisecond)
	saved.Description = "Updated description with more evidence in internal/service/order.go."
	require.NoError(t, repo.Save(ctx, saved))

	updated, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	require.True(t, updated.CreatedAt.Equal(createdAt), "CreatedAt should be stable across updates")
	require.True(t, updated.UpdatedAt.After(updatedAt), "UpdatedAt should move forward on update")
	require.True(t, updated.BusinessMethod.CodeLocation.CreatedAt.Equal(locationCreatedAt))
	require.True(t, updated.BusinessMethod.CodeLocation.UpdatedAt.After(locationUpdatedAt))

	replacement := newTestPattern("p-001", "business service", domain.CategoryBusiness, 0.88)
	replacement.SetBusinessMethod(&domain.BusinessMethod{
		Name:         "CreateOrder",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/service/order.go:84"},
		Type:         "domain",
	})
	require.NoError(t, repo.Save(ctx, replacement))

	replaced, err := repo.Get(ctx, "p-001")
	require.NoError(t, err)
	require.True(t, replaced.CreatedAt.Equal(createdAt), "existing DB CreatedAt should win over NewPattern defaults")
	require.True(t, replaced.BusinessMethod.CodeLocation.CreatedAt.Equal(locationCreatedAt))
	require.Equal(t, "internal/service/order.go:42", replaced.BusinessMethod.CodeLocation.HistoricalLocation)
	require.Equal(t, "internal/service/order.go:84", replaced.BusinessMethod.CodeLocation.CurrentLocation)
}

func TestPatternRepository_RecordPatternHitsAndStats(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p1 := newTestPattern("p-001", "specific check", domain.CategoryError, 0.9)
	p1.Description = "Wrap errors from internal/service/checker/service.go with domain.NewDomainError."
	p2 := newTestPattern("p-002", "unused check", domain.CategoryTesting, 0.7)
	require.NoError(t, repo.Save(ctx, p1))
	require.NoError(t, repo.Save(ctx, p2))

	now := time.Now().UTC()
	require.NoError(t, repo.RecordPatternHits(ctx, []domain.PatternHit{
		{
			PatternID:  "p-001",
			File:       "internal/service/checker/service.go",
			Line:       81,
			Severity:   domain.SeverityWarning,
			Confidence: 0.82,
			CheckRunID: "run-1",
			CreatedAt:  now,
		},
		{
			PatternID:  "p-001",
			File:       "internal/service/checker/service.go",
			Line:       83,
			Severity:   domain.SeverityError,
			Confidence: 0.9,
			CheckRunID: "run-2",
			CreatedAt:  now.Add(time.Minute),
		},
		{
			PatternID: "",
			File:      "ignored.go",
		},
	}))

	stats, err := repo.GetPatternHitStats(ctx)
	require.NoError(t, err)
	require.Len(t, stats, 2)

	assert.Equal(t, "p-001", stats[0].Pattern.ID)
	assert.Equal(t, 2, stats[0].HitCount)
	assert.WithinDuration(t, now.Add(time.Minute), stats[0].LastHitAt, time.Second)
	assert.Equal(t, "p-002", stats[1].Pattern.ID)
	assert.Equal(t, 0, stats[1].HitCount)
	assert.True(t, stats[1].LastHitAt.IsZero())
}

func TestPatternRepository_RecordPatternHitsMaintainsTimestamps(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, repo.RecordPatternHits(ctx, []domain.PatternHit{
		{
			PatternID:  "p-001",
			File:       "internal/service/checker/service.go",
			Line:       81,
			Severity:   domain.SeverityWarning,
			Confidence: 0.82,
		},
	}))

	var hits []domain.PatternHit
	require.NoError(t, repo.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketPatternHits)
		return bucket.ForEach(func(k, v []byte) error {
			var hit domain.PatternHit
			if err := json.Unmarshal(v, &hit); err != nil {
				return err
			}
			hits = append(hits, hit)
			return nil
		})
	}))
	require.Len(t, hits, 1)
	require.False(t, hits[0].CreatedAt.IsZero())
	require.False(t, hits[0].UpdatedAt.IsZero())
	require.True(t, hits[0].UpdatedAt.Equal(hits[0].CreatedAt))
}

func TestPatternRepository_ImportReviewCommentsAndStats(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 28, 9, 0, 0, 0, time.UTC)
	require.NoError(t, repo.RecordPatternHits(ctx, []domain.PatternHit{
		{
			PatternID:  "p-error-wrap",
			File:       "internal/service/checker/service.go",
			Line:       82,
			Severity:   domain.SeverityWarning,
			Confidence: 0.86,
			CheckRunID: "run-1",
			CreatedAt:  now,
		},
		{
			PatternID:  "p-naming",
			File:       "internal/domain/models.go",
			Line:       44,
			Severity:   domain.SeverityInfo,
			Confidence: 0.71,
			CheckRunID: "run-2",
			CreatedAt:  now.Add(time.Minute),
		},
	}))

	comments := []domain.ReviewComment{
		{
			ID:        "c-1",
			Provider:  "local",
			ReviewID:  "review-1",
			Commit:    "abc123",
			File:      "internal/service/checker/service.go",
			Line:      84,
			Author:    "reviewer",
			Body:      "wrap checker errors",
			Resolved:  true,
			CreatedAt: now.Add(2 * time.Minute),
		},
		{
			ID:        "c-2",
			Provider:  "local",
			ReviewID:  "review-1",
			Commit:    "abc123",
			File:      "internal/service/generator/service.go",
			Line:      20,
			Author:    "reviewer",
			Body:      "new uncovered feedback",
			Resolved:  false,
			CreatedAt: now.Add(3 * time.Minute),
		},
	}

	require.NoError(t, repo.ImportReviewComments(ctx, comments))

	stats, err := repo.GetReviewStats(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalComments)
	assert.Equal(t, 1, stats.PreventedComments)
	assert.Equal(t, 1, stats.MissedComments)
	require.Len(t, stats.MatchedPatterns, 1)
	assert.Equal(t, "p-error-wrap", stats.MatchedPatterns[0].PatternID)
	assert.Equal(t, 1, stats.MatchedPatterns[0].CommentCount)
}

func TestPatternRepository_ImportReviewCommentsMaintainsTimestamps(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, repo.ImportReviewComments(ctx, []domain.ReviewComment{
		{
			ID:       "c-1",
			Provider: "local",
			File:     "internal/service/checker/service.go",
			Line:     84,
			Body:     "wrap checker errors",
		},
	}))

	first := readReviewCommentForTest(t, repo, "c-1")
	require.False(t, first.CreatedAt.IsZero())
	require.False(t, first.UpdatedAt.IsZero())

	time.Sleep(2 * time.Millisecond)
	first.Body = "wrap checker errors with domain error"
	require.NoError(t, repo.ImportReviewComments(ctx, []domain.ReviewComment{first}))

	updated := readReviewCommentForTest(t, repo, "c-1")
	require.True(t, updated.CreatedAt.Equal(first.CreatedAt))
	require.True(t, updated.UpdatedAt.After(first.UpdatedAt))
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

func TestPatternRepository_FindSimilarNormalizesCategory(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	original := newTestPattern("p-001", "path guard", domain.CategoryUtils, 0.9)
	require.NoError(t, repo.Save(ctx, original))

	search := &domain.Pattern{
		Name:     "path guard",
		Category: domain.Category(" Security "),
	}
	found, err := repo.FindSimilar(ctx, search)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "p-001", found.ID)
	require.Equal(t, domain.Category(" Security "), search.Category)
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

func TestPatternRepository_DeleteRemovesAllCategoryCopies(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	p := newTestPattern("p-001", "duplicated", domain.CategoryUtils, 0.9)
	require.NoError(t, repo.Save(ctx, p))
	legacy := newTestPattern("p-001", "duplicated legacy", domain.Category("security"), 0.8)
	require.NoError(t, repo.db.Update(func(tx *bolt.Tx) error {
		categoryBucket, err := tx.Bucket(bucketPatterns).CreateBucketIfNotExists([]byte("security"))
		if err != nil {
			return err
		}
		data, err := json.Marshal(legacy)
		if err != nil {
			return err
		}
		return categoryBucket.Put([]byte(legacy.ID), data)
	}))

	require.NoError(t, repo.Delete(ctx, "p-001"))

	all, err := repo.GetAll(ctx)
	require.NoError(t, err)
	require.Empty(t, all)
	_, err = repo.Get(ctx, "p-001")
	require.Error(t, err)
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

func TestPatternRepository_MarkCommitAnalyzedStoresTimestampedRecords(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, repo.MarkCommitAnalyzed(ctx, "abc123"))

	var records []domain.AnalyzedCommitRecord
	require.NoError(t, repo.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketMetadata).Get(keyAnalyzedCommits)
		return json.Unmarshal(data, &records)
	}))
	require.Len(t, records, 1)
	require.Equal(t, "abc123", records[0].Hash)
	require.False(t, records[0].CreatedAt.IsZero())
	require.False(t, records[0].UpdatedAt.IsZero())
}

func TestPatternRepository_MarkCommitsAnalyzedStoresBatch(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, repo.MarkCommitsAnalyzed(ctx, []string{"abc123", "def456", "abc123"}))

	commits, err := repo.GetAnalyzedCommits(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"abc123", "def456"}, commits)
}

func TestPatternRepository_FileAnalysisTracking(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	scope := domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}

	records := []domain.FileAnalysisRecord{
		{
			ProjectID:       "backend",
			ScopePath:       "backend",
			Path:            "internal/app.go",
			Hash:            "abc",
			HashAlgorithm:   domain.FileAnalysisHashMD5,
			Size:            12,
			ModTime:         "2026-05-26T00:00:00Z",
			Source:          domain.FileAnalysisSourceCurrentCode,
			AnalysisStatus:  domain.FileAnalysisStatusAISkipped,
			SelectionReason: "generated file",
			LastAnalyzedAt:  "2026-05-26T00:00:01Z",
		},
	}

	require.NoError(t, repo.SaveAnalyzedFiles(ctx, records))

	got, err := repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "abc", got.Hash)
	assert.Equal(t, domain.FileAnalysisStatusAISkipped, got.AnalysisStatus)
	assert.Equal(t, "generated file", got.SelectionReason)

	list, err := repo.ListAnalyzedFiles(ctx, scope)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, repo.DeleteAnalyzedFiles(ctx, scope, []string{"internal/app.go"}))
	got, err = repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPatternRepository_SaveAnalyzedFilesDefaultsAnalysisStatus(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	scope := domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}

	require.NoError(t, repo.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{
		{
			ProjectID:     "backend",
			ScopePath:     "backend",
			Path:          "internal/app.go",
			Hash:          "abc",
			HashAlgorithm: domain.FileAnalysisHashMD5,
			Source:        domain.FileAnalysisSourceCurrentCode,
		},
	}))

	got, err := repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.FileAnalysisStatusAnalyzed, got.AnalysisStatus)
}

func TestPatternRepository_SaveAnalyzedFilesMaintainsTimestamps(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()
	scope := domain.FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}

	require.NoError(t, repo.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{
		{
			ProjectID:     "backend",
			ScopePath:     "backend",
			Path:          "internal/app.go",
			Hash:          "abc",
			HashAlgorithm: domain.FileAnalysisHashMD5,
			Source:        domain.FileAnalysisSourceCurrentCode,
		},
	}))

	first, err := repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.False(t, first.CreatedAt.IsZero())
	require.False(t, first.UpdatedAt.IsZero())

	time.Sleep(2 * time.Millisecond)
	require.NoError(t, repo.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{
		{
			ProjectID:     "backend",
			ScopePath:     "backend",
			Path:          "internal/app.go",
			Hash:          "def",
			HashAlgorithm: domain.FileAnalysisHashMD5,
			Source:        domain.FileAnalysisSourceCurrentCode,
		},
	}))

	updated, err := repo.GetAnalyzedFile(ctx, scope, "internal/app.go")
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.True(t, updated.CreatedAt.Equal(first.CreatedAt))
	require.True(t, updated.UpdatedAt.After(first.UpdatedAt))
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

func readReviewCommentForTest(t *testing.T, repo *PatternRepository, id string) domain.ReviewComment {
	t.Helper()

	var comment domain.ReviewComment
	require.NoError(t, repo.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(bucketReviewComments).Get([]byte(id))
		return json.Unmarshal(data, &comment)
	}))
	return comment
}
