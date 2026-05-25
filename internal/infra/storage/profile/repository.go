package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// ErrProfileNotFound 表示项目画像不存在
var ErrProfileNotFound = errors.New("project profile not found")

// Repository stores the project profile as a readable JSON document under .skills-seed
type Repository struct {
	path string
}

// NewRepository 创建项目画像仓储
func NewRepository(seedPath string) *Repository {
	return &Repository{
		path: filepath.Join(seedPath, "memory", "project-profile.json"),
	}
}

// Path 返回项目画像文件路径
func (r *Repository) Path() string {
	return r.path
}

// Get 读取项目画像
func (r *Repository) Get(ctx context.Context) (*domain.ProjectProfile, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("read project profile: %w", err)
	}

	var profile domain.ProjectProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse project profile: %w", err)
	}
	return &profile, nil
}

// Save 保存项目画像
func (r *Repository) Save(ctx context.Context, projectProfile *domain.ProjectProfile) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if projectProfile == nil {
		return fmt.Errorf("project profile is nil")
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return fmt.Errorf("create project profile directory: %w", err)
	}

	data, err := json.MarshalIndent(projectProfile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project profile: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(r.path, data, 0644); err != nil {
		return fmt.Errorf("write project profile: %w", err)
	}
	return nil
}
