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
	"github.com/silaswei-io/skills-seed/internal/utils/jsonx"
	bolt "go.etcd.io/bbolt"
)

// PatternRepository Pattern 仓储实现
type PatternRepository struct {
	db *bolt.DB
}

var (
	bucketPatterns      = []byte("patterns")
	bucketMetadata      = []byte("metadata")
	bucketAnalyzedFiles = []byte("analyzed_files")
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
				if err := jsonx.Unmarshal(data, &found); err != nil {
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
				if err := jsonx.Unmarshal(v, &p); err != nil {
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
	category = domain.NormalizePatternCategory(category)

	err := r.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)
		categoryBucket := mainBucket.Bucket([]byte(category))
		if categoryBucket == nil {
			// 该分类不存在，返回空列表
			return nil
		}

		return categoryBucket.ForEach(func(k, v []byte) error {
			var p domain.Pattern
			if err := jsonx.Unmarshal(v, &p); err != nil {
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
	if err := validatePatternForSave(p); err != nil {
		return err
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		return savePatternInTx(tx.Bucket(bucketPatterns), p)
	})
}

// ApplyPatternMutation 在一个 Bolt 写事务中应用模式删除和保存。
func (r *PatternRepository) ApplyPatternMutation(ctx context.Context, mutation domain.PatternMutation) error {
	for _, pattern := range mutation.Save {
		if err := validatePatternForSave(pattern); err != nil {
			return err
		}
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)
		for _, id := range mutation.DeleteIDs {
			if err := deletePatternInTx(mainBucket, id); err != nil {
				return err
			}
		}
		for _, pattern := range mutation.Save {
			if err := savePatternInTx(mainBucket, pattern); err != nil {
				return err
			}
		}
		return nil
	})
}

func validatePatternForSave(pattern *domain.Pattern) error {
	if pattern == nil {
		return fmt.Errorf("invalid pattern")
	}
	pattern.Category = domain.NormalizePatternCategory(pattern.Category)
	if !pattern.IsValid() {
		return fmt.Errorf("invalid pattern")
	}
	return nil
}

func savePatternInTx(mainBucket *bolt.Bucket, pattern *domain.Pattern) error {
	categoryKey := []byte(pattern.Category)
	categoryBucket, err := mainBucket.CreateBucketIfNotExists(categoryKey)
	if err != nil {
		return fmt.Errorf("failed to create category bucket %s: %w", pattern.Category, err)
	}
	previous, err := findPatternInTx(mainBucket, pattern.ID)
	if err != nil {
		return err
	}
	pattern.RefreshMetrics()
	pattern.NormalizeForSave(previous, time.Now())
	data, err := json.Marshal(pattern)
	if err != nil {
		return err
	}
	if err := categoryBucket.Put([]byte(pattern.ID), data); err != nil {
		return err
	}
	return deletePatternCopiesOutsideCategory(mainBucket, pattern.ID, pattern.Category)
}

// GetPatternStats 返回所有模式的质量指标。
func (r *PatternRepository) GetPatternStats(ctx context.Context) ([]domain.PatternStats, error) {
	patterns, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	stats := make([]domain.PatternStats, 0, len(patterns))
	for _, pattern := range patterns {
		stats = append(stats, domain.PatternStats{Pattern: pattern})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Pattern.Metrics.EffectiveScore != stats[j].Pattern.Metrics.EffectiveScore {
			return stats[i].Pattern.Metrics.EffectiveScore > stats[j].Pattern.Metrics.EffectiveScore
		}
		if stats[i].Pattern.Confidence != stats[j].Pattern.Confidence {
			return stats[i].Pattern.Confidence > stats[j].Pattern.Confidence
		}
		return stats[i].Pattern.ID < stats[j].Pattern.ID
	})
	return stats, nil
}

// FindSimilar 查找相似的模式
func (r *PatternRepository) FindSimilar(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error) {
	var found *domain.Pattern
	if pattern == nil {
		return nil, nil
	}
	searchPattern := *pattern
	searchPattern.Category = domain.NormalizePatternCategory(searchPattern.Category)

	err := r.db.View(func(tx *bolt.Tx) error {
		mainBucket := tx.Bucket(bucketPatterns)

		// 只在相同分类中查找相似模式
		categoryKey := []byte(searchPattern.Category)
		categoryBucket := mainBucket.Bucket(categoryKey)
		if categoryBucket == nil {
			return nil // 该分类不存在
		}

		return categoryBucket.ForEach(func(k, v []byte) error {
			var p domain.Pattern
			if err := jsonx.Unmarshal(v, &p); err != nil {
				return err
			}

			// 检查是否相似
			if p.IsSimilar(&searchPattern) {
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
		return deletePatternInTx(tx.Bucket(bucketPatterns), id)
	})
}

func deletePatternInTx(mainBucket *bolt.Bucket, id string) error {
	return mainBucket.ForEach(func(categoryKey, _ []byte) error {
		categoryBucket := mainBucket.Bucket(categoryKey)
		if categoryBucket == nil {
			return nil
		}
		return categoryBucket.Delete([]byte(id))
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
		if err := jsonx.Unmarshal(data, &found); err != nil {
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
			if err := jsonx.Unmarshal(v, &record); err != nil {
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
				if err := jsonx.Unmarshal(data, &previous); err != nil {
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
			path = filepath.ToSlash(filepath.Clean(path))
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
		if err := jsonx.Unmarshal(data, &pattern); err != nil {
			return err
		}
		pattern.NormalizeAfterLoad()
		found = &pattern
		return nil
	})
	return found, err
}

func deletePatternCopiesOutsideCategory(mainBucket *bolt.Bucket, id string, keepCategory domain.Category) error {
	keepKey := []byte(keepCategory)
	return mainBucket.ForEach(func(categoryKey, _ []byte) error {
		if bytes.Equal(categoryKey, keepKey) {
			return nil
		}
		categoryBucket := mainBucket.Bucket(categoryKey)
		if categoryBucket == nil {
			return nil
		}
		return categoryBucket.Delete([]byte(id))
	})
}
