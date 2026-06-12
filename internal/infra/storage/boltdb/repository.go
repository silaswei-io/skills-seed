package boltdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	bolt "go.etcd.io/bbolt"
)

// PatternRepository Pattern 仓储实现
type PatternRepository struct {
	db *bolt.DB
}

var (
	bucketPatterns       = []byte("patterns")
	bucketMetadata       = []byte("metadata")
	bucketAnalyzedFiles  = []byte("analyzed_files")
	bucketPatternHits    = []byte("pattern_hits")
	bucketReviewComments = []byte("review_comments")
	keyAnalyzedCommits   = []byte("analyzed_commits")
)

// NewPatternRepository 创建 Pattern 仓储
func NewPatternRepository(dbPath string) (*PatternRepository, error) {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// 创建主bucket和metadata bucket
	err = db.Update(func(tx *bolt.Tx) error {
		// 创建 patterns 主bucket
		if _, err := tx.CreateBucketIfNotExists(bucketPatterns); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketPatterns, err)
		}

		// 创建 metadata bucket（用于存储已分析的commit等）
		if _, err := tx.CreateBucketIfNotExists(bucketMetadata); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketMetadata, err)
		}

		// 创建 analyzed_files bucket（用于保存 learn current 文件指纹）
		if _, err := tx.CreateBucketIfNotExists(bucketAnalyzedFiles); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketAnalyzedFiles, err)
		}
		if _, err := tx.CreateBucketIfNotExists(bucketPatternHits); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketPatternHits, err)
		}
		if _, err := tx.CreateBucketIfNotExists(bucketReviewComments); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketReviewComments, err)
		}

		return nil
	})

	if err != nil {
		db.Close()
		return nil, err
	}

	return &PatternRepository{db: db}, nil
}

// Get 根据ID获取模式
func (r *PatternRepository) Get(ctx context.Context, id string) (*domain.Pattern, error) {
	var p *domain.Pattern

	err := r.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)

		// 遍历所有分类子bucket查找
		return mainBucket.ForEach(func(categoryKey, _ []byte) error {
			categoryBucket := mainBucket.Bucket(categoryKey)
			if categoryBucket == nil {
				return nil
			}

			data := categoryBucket.Get([]byte(id))
			if data != nil {
				var found domain.Pattern
				if err := json.Unmarshal(data, &found); err != nil {
					return err
				}
				found.NormalizeAfterLoad()
				p = &found
				return nil // 找到了
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	if p == nil {
		return nil, fmt.Errorf("pattern not found: %s", id)
	}

	return p, nil
}

// GetAll 获取所有模式
func (r *PatternRepository) GetAll(ctx context.Context) ([]domain.Pattern, error) {
	var patterns []domain.Pattern

	err := r.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)

		// 遍历所有分类子bucket
		return mainBucket.ForEach(func(categoryKey, _ []byte) error {
			categoryBucket := mainBucket.Bucket(categoryKey)
			if categoryBucket == nil {
				return nil
			}

			// 遍历该分类下的所有模式
			return categoryBucket.ForEach(func(k, v []byte) error {
				var p domain.Pattern
				if err := json.Unmarshal(v, &p); err != nil {
					return err
				}
				p.NormalizeAfterLoad()
				patterns = append(patterns, p)
				return nil
			})
		})
	})

	if err != nil {
		return nil, err
	}

	return patterns, nil
}

// GetByCategory 根据分类获取模式
func (r *PatternRepository) GetByCategory(ctx context.Context, category domain.Category) ([]domain.Pattern, error) {
	var patterns []domain.Pattern

	err := r.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)
		categoryBucket := mainBucket.Bucket([]byte(category))
		if categoryBucket == nil {
			// 该分类不存在，返回空列表
			return nil
		}

		return categoryBucket.ForEach(func(k, v []byte) error {
			var p domain.Pattern
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			p.NormalizeAfterLoad()
			patterns = append(patterns, p)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return patterns, nil
}

// GetHighConfidence 获取高置信度模式
func (r *PatternRepository) GetHighConfidence(ctx context.Context, threshold float64) ([]domain.Pattern, error) {
	all, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []domain.Pattern
	for _, p := range all {
		if p.Confidence >= threshold {
			filtered = append(filtered, p)
		}
	}

	return filtered, nil
}

// Save 保存模式
func (r *PatternRepository) Save(ctx context.Context, p *domain.Pattern) error {
	if !p.IsValid() {
		return fmt.Errorf("invalid pattern")
	}

	return r.db.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)

		// 获取或创建该分类的子bucket
		categoryKey := []byte(p.Category)
		categoryBucket, err := mainBucket.CreateBucketIfNotExists(categoryKey)
		if err != nil {
			return fmt.Errorf("failed to create category bucket %s: %w", p.Category, err)
		}

		previous, err := findPatternInTx(mainBucket, p.ID)
		if err != nil {
			return err
		}
		p.RefreshMetrics()
		p.NormalizeForSave(previous, time.Now())

		// 在该分类的子bucket中保存模式
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return categoryBucket.Put([]byte(p.ID), data)
	})
}

// RecordPatternHits 保存 check 命中记录。
func (r *PatternRepository) RecordPatternHits(ctx context.Context, hits []domain.PatternHit) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketPatternHits)
		for _, hit := range hits {
			if hit.PatternID == "" {
				continue
			}
			if hit.CreatedAt.IsZero() {
				hit.CreatedAt = time.Now()
			}
			if hit.UpdatedAt.IsZero() {
				hit.UpdatedAt = hit.CreatedAt
			}
			if hit.CheckRunID == "" {
				hit.CheckRunID = fmt.Sprintf("check-%d", hit.CreatedAt.UnixNano())
			}
			data, err := json.Marshal(hit)
			if err != nil {
				return err
			}
			key := []byte(fmt.Sprintf("%s/%020d/%s/%d", hit.PatternID, hit.CreatedAt.UnixNano(), filepath.ToSlash(hit.File), hit.Line))
			if err := bucket.Put(key, data); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetPatternHitStats 返回所有模式的质量指标和 check 命中统计。
func (r *PatternRepository) GetPatternHitStats(ctx context.Context) ([]domain.PatternHitStats, error) {
	patterns, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	statsByPattern := make(map[string]*domain.PatternHitStats, len(patterns))
	for _, p := range patterns {
		pattern := p
		statsByPattern[p.ID] = &domain.PatternHitStats{Pattern: pattern}
	}

	err = r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketPatternHits)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			var hit domain.PatternHit
			if err := json.Unmarshal(v, &hit); err != nil {
				return err
			}
			stat, ok := statsByPattern[hit.PatternID]
			if !ok {
				return nil
			}
			stat.HitCount++
			if stat.LastHitAt.IsZero() || hit.CreatedAt.After(stat.LastHitAt) {
				stat.LastHitAt = hit.CreatedAt
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	stats := make([]domain.PatternHitStats, 0, len(statsByPattern))
	for _, stat := range statsByPattern {
		stats = append(stats, *stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].HitCount != stats[j].HitCount {
			return stats[i].HitCount > stats[j].HitCount
		}
		if stats[i].Pattern.Metrics.EffectiveScore != stats[j].Pattern.Metrics.EffectiveScore {
			return stats[i].Pattern.Metrics.EffectiveScore > stats[j].Pattern.Metrics.EffectiveScore
		}
		return stats[i].Pattern.ID < stats[j].Pattern.ID
	})
	return stats, nil
}

// ImportReviewComments 保存导入的代码评审评论。
func (r *PatternRepository) ImportReviewComments(ctx context.Context, comments []domain.ReviewComment) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketReviewComments)
		for _, comment := range comments {
			if comment.ID == "" {
				continue
			}
			previous := domain.ReviewComment{}
			if data := bucket.Get([]byte(comment.ID)); data != nil {
				if err := json.Unmarshal(data, &previous); err != nil {
					return err
				}
			}
			now := time.Now()
			if comment.CreatedAt.IsZero() {
				if !previous.CreatedAt.IsZero() {
					comment.CreatedAt = previous.CreatedAt
				} else {
					comment.CreatedAt = now
				}
			}
			comment.UpdatedAt = now
			data, err := json.Marshal(comment)
			if err != nil {
				return err
			}
			if err := bucket.Put([]byte(comment.ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetReviewStats 返回评审评论与已有 check 命中的匹配统计。
func (r *PatternRepository) GetReviewStats(ctx context.Context, lineWindow int) (domain.ReviewStats, error) {
	if lineWindow < 0 {
		lineWindow = 0
	}

	comments, err := r.listReviewComments(ctx)
	if err != nil {
		return domain.ReviewStats{}, err
	}
	hits, err := r.listPatternHits(ctx)
	if err != nil {
		return domain.ReviewStats{}, err
	}

	stats := domain.ReviewStats{TotalComments: len(comments)}
	matchedByPattern := make(map[string]int)
	for _, comment := range comments {
		match := findReviewCommentHit(comment, hits, lineWindow)
		if match == nil {
			stats.MissedComments++
			continue
		}
		stats.PreventedComments++
		matchedByPattern[match.PatternID]++
	}

	stats.MatchedPatterns = make([]domain.ReviewMatchedPatternStats, 0, len(matchedByPattern))
	for patternID, count := range matchedByPattern {
		stats.MatchedPatterns = append(stats.MatchedPatterns, domain.ReviewMatchedPatternStats{
			PatternID:    patternID,
			CommentCount: count,
		})
	}
	sort.Slice(stats.MatchedPatterns, func(i, j int) bool {
		if stats.MatchedPatterns[i].CommentCount != stats.MatchedPatterns[j].CommentCount {
			return stats.MatchedPatterns[i].CommentCount > stats.MatchedPatterns[j].CommentCount
		}
		return stats.MatchedPatterns[i].PatternID < stats.MatchedPatterns[j].PatternID
	})

	return stats, nil
}

func (r *PatternRepository) listReviewComments(ctx context.Context) ([]domain.ReviewComment, error) {
	var comments []domain.ReviewComment
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketReviewComments)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			var comment domain.ReviewComment
			if err := json.Unmarshal(v, &comment); err != nil {
				return err
			}
			comments = append(comments, comment)
			return nil
		})
	})
	return comments, err
}

func (r *PatternRepository) listPatternHits(ctx context.Context) ([]domain.PatternHit, error) {
	var hits []domain.PatternHit
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketPatternHits)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			var hit domain.PatternHit
			if err := json.Unmarshal(v, &hit); err != nil {
				return err
			}
			if hit.PatternID != "" {
				hits = append(hits, hit)
			}
			return nil
		})
	})
	return hits, err
}

func findReviewCommentHit(comment domain.ReviewComment, hits []domain.PatternHit, lineWindow int) *domain.PatternHit {
	commentFile := normalizeReviewPath(comment.File)
	for i := range hits {
		hit := &hits[i]
		if normalizeReviewPath(hit.File) != commentFile {
			continue
		}
		if absInt(hit.Line-comment.Line) <= lineWindow {
			return hit
		}
	}
	return nil
}

func normalizeReviewPath(path string) string {
	return filepath.ToSlash(path)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// FindSimilar 查找相似的模式
func (r *PatternRepository) FindSimilar(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error) {
	var found *domain.Pattern

	err := r.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)

		// 只在相同分类中查找相似模式
		categoryKey := []byte(pattern.Category)
		categoryBucket := mainBucket.Bucket(categoryKey)
		if categoryBucket == nil {
			return nil // 该分类不存在
		}

		return categoryBucket.ForEach(func(k, v []byte) error {
			var p domain.Pattern
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}

			// 检查是否相似
			if p.IsSimilar(pattern) {
				found = &p
				return nil // 找到了
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	if found == nil {
		return nil, nil
	}

	return found, nil
}

// Delete 删除模式
func (r *PatternRepository) Delete(ctx context.Context, id string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)

		// 在所有分类中查找并删除
		return mainBucket.ForEach(func(categoryKey, _ []byte) error {
			categoryBucket := mainBucket.Bucket(categoryKey)
			if categoryBucket == nil {
				return nil
			}

			if categoryBucket.Get([]byte(id)) != nil {
				return categoryBucket.Delete([]byte(id))
			}
			return nil
		})
	})
}

// Count 统计模式数量
func (r *PatternRepository) Count(ctx context.Context) (int, error) {
	count := 0

	err := r.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)

		return mainBucket.ForEach(func(categoryKey, _ []byte) error {
			categoryBucket := mainBucket.Bucket(categoryKey)
			if categoryBucket == nil {
				return nil
			}

			return categoryBucket.ForEach(func(k, v []byte) error {
				count++
				return nil
			})
		})
	})

	return count, err
}

// MarkCommitAnalyzed 标记commit已被分析
func (r *PatternRepository) MarkCommitAnalyzed(ctx context.Context, commitHash string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		metaBucket := tx.Bucket(bucketMetadata)

		// 获取已存在的列表
		records, err := readAnalyzedCommitRecords(metaBucket.Get(keyAnalyzedCommits))
		if err != nil {
			return err
		}
		// 添加新的commit（如果不存在）
		for _, record := range records {
			if record.Hash == commitHash {
				return nil // 已存在
			}
		}

		now := time.Now()
		records = append(records, domain.AnalyzedCommitRecord{
			Hash:      commitHash,
			CreatedAt: now,
			UpdatedAt: now,
		})

		// 保存更新后的列表
		updated, err := json.Marshal(records)
		if err != nil {
			return err
		}
		return metaBucket.Put(keyAnalyzedCommits, updated)
	})
}

// IsCommitAnalyzed 检查commit是否已被分析
func (r *PatternRepository) IsCommitAnalyzed(ctx context.Context, commitHash string) (bool, error) {
	var analyzed bool

	err := r.db.View(func(tx *bolt.Tx) error {
		metaBucket := tx.Bucket(bucketMetadata)
		data := metaBucket.Get(keyAnalyzedCommits)
		if data == nil {
			return nil // 没有任何记录
		}

		records, err := readAnalyzedCommitRecords(data)
		if err != nil {
			return err
		}

		for _, record := range records {
			if record.Hash == commitHash {
				analyzed = true
				break
			}
		}
		return nil
	})

	return analyzed, err
}

// GetAnalyzedCommits 获取所有已分析的commit列表
func (r *PatternRepository) GetAnalyzedCommits(ctx context.Context) ([]string, error) {
	var commits []string

	err := r.db.View(func(tx *bolt.Tx) error {
		metaBucket := tx.Bucket(bucketMetadata)
		data := metaBucket.Get(keyAnalyzedCommits)
		if data == nil {
			return nil
		}

		records, err := readAnalyzedCommitRecords(data)
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.Hash != "" {
				commits = append(commits, record.Hash)
			}
		}
		return nil
	})

	return commits, err
}

// GetAnalyzedFile 获取指定文件最近一次成功分析记录
func (r *PatternRepository) GetAnalyzedFile(ctx context.Context, scope domain.FileAnalysisScope, path string) (*domain.FileAnalysisRecord, error) {
	var record *domain.FileAnalysisRecord

	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		data := bucket.Get([]byte(scope.KeyForPath(path)))
		if data == nil {
			return nil
		}

		var found domain.FileAnalysisRecord
		if err := json.Unmarshal(data, &found); err != nil {
			return err
		}
		normalizeFileAnalysisRecordDefaults(&found)
		record = &found
		return nil
	})

	return record, err
}

// ListAnalyzedFiles 获取指定范围内的全部文件分析记录
func (r *PatternRepository) ListAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
	records := make([]domain.FileAnalysisRecord, 0)
	prefix := []byte(scope.KeyPrefix())

	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		return bucket.ForEach(func(k, v []byte) error {
			if !bytes.HasPrefix(k, prefix) {
				return nil
			}

			var record domain.FileAnalysisRecord
			if err := json.Unmarshal(v, &record); err != nil {
				return err
			}
			normalizeFileAnalysisRecordDefaults(&record)
			records = append(records, record)
			return nil
		})
	})

	return records, err
}

// SaveAnalyzedFiles 保存一批文件分析记录
func (r *PatternRepository) SaveAnalyzedFiles(ctx context.Context, records []domain.FileAnalysisRecord) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		for _, record := range records {
			scope := domain.FileAnalysisScope{ProjectID: record.ProjectID, ScopePath: record.ScopePath}
			record.Path = filepath.ToSlash(filepath.Clean(record.Path))
			normalizeFileAnalysisRecordDefaults(&record)
			key := []byte(scope.KeyForPath(record.Path))
			previous := domain.FileAnalysisRecord{}
			if data := bucket.Get(key); data != nil {
				if err := json.Unmarshal(data, &previous); err != nil {
					return err
				}
			}
			now := time.Now()
			if record.CreatedAt.IsZero() {
				if !previous.CreatedAt.IsZero() {
					record.CreatedAt = previous.CreatedAt
				} else {
					record.CreatedAt = now
				}
			}
			record.UpdatedAt = now
			data, err := json.Marshal(record)
			if err != nil {
				return err
			}
			if err := bucket.Put(key, data); err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeFileAnalysisRecordDefaults(record *domain.FileAnalysisRecord) {
	if record == nil {
		return
	}
	if record.AnalysisStatus != "" {
		return
	}
	switch record.Source {
	case domain.FileAnalysisSourceInputDigest:
		record.AnalysisStatus = domain.FileAnalysisStatusInputDigest
	default:
		record.AnalysisStatus = domain.FileAnalysisStatusAnalyzed
	}
}

// DeleteAnalyzedFiles 删除指定范围内的文件分析记录
func (r *PatternRepository) DeleteAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAnalyzedFiles)
		for _, path := range paths {
			if err := bucket.Delete([]byte(scope.KeyForPath(path))); err != nil {
				return err
			}
		}
		return nil
	})
}

// Close 关闭数据库
func (r *PatternRepository) Close() error {
	return r.db.Close()
}

func findPatternInTx(mainBucket *bolt.Bucket, id string) (*domain.Pattern, error) {
	var found *domain.Pattern
	err := mainBucket.ForEach(func(categoryKey, _ []byte) error {
		categoryBucket := mainBucket.Bucket(categoryKey)
		if categoryBucket == nil {
			return nil
		}
		data := categoryBucket.Get([]byte(id))
		if data == nil {
			return nil
		}
		var pattern domain.Pattern
		if err := json.Unmarshal(data, &pattern); err != nil {
			return err
		}
		pattern.NormalizeAfterLoad()
		found = &pattern
		return nil
	})
	return found, err
}

func readAnalyzedCommitRecords(data []byte) ([]domain.AnalyzedCommitRecord, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var records []domain.AnalyzedCommitRecord
	if err := json.Unmarshal(data, &records); err == nil && records != nil {
		return records, nil
	}

	var hashes []string
	if err := json.Unmarshal(data, &hashes); err != nil {
		return nil, err
	}
	now := time.Now()
	records = make([]domain.AnalyzedCommitRecord, 0, len(hashes))
	for _, hash := range hashes {
		if hash == "" {
			continue
		}
		records = append(records, domain.AnalyzedCommitRecord{
			Hash:      hash,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return records, nil
}
