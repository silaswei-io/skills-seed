package boltdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	bolt "go.etcd.io/bbolt"
)

// PatternRepository Pattern 仓储实现
type PatternRepository struct {
	db *bolt.DB
}

var (
	bucketPatterns     = []byte("patterns")
	bucketMetadata     = []byte("metadata")
	keyAnalyzedCommits = []byte("analyzed_commits")
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

		// 在该分类的子bucket中保存模式
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return categoryBucket.Put([]byte(p.ID), data)
	})
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
		var commits []string
		data := metaBucket.Get(keyAnalyzedCommits)
		if data != nil {
			if err := json.Unmarshal(data, &commits); err != nil {
				return err
			}
		}

		// 添加新的commit（如果不存在）
		for _, c := range commits {
			if c == commitHash {
				return nil // 已存在
			}
		}

		commits = append(commits, commitHash)

		// 保存更新后的列表
		updated, err := json.Marshal(commits)
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

		var commits []string
		if err := json.Unmarshal(data, &commits); err != nil {
			return err
		}

		for _, c := range commits {
			if c == commitHash {
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

		return json.Unmarshal(data, &commits)
	})

	return commits, err
}

// Close 关闭数据库
func (r *PatternRepository) Close() error {
	return r.db.Close()
}
