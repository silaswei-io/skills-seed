package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// ErrProfileNotFound 表示项目画像不存在
var ErrProfileNotFound = errors.New("project profile not found")

// ErrSpecNotFound 表示项目开发规范不存在
var ErrSpecNotFound = errors.New("project spec not found")

// Repository 将项目画像以可读 JSON 文档保存到 .skills-seed 下
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
		return nil, fmt.Errorf("%s: %w", i18n.Get("ProjectProfileReadFailed"), err)
	}

	var profile domain.ProjectProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ProjectProfileParseFailed"), err)
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
		return fmt.Errorf("%s", i18n.Get("ProjectProfileNil"))
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectProfileCreateDirFailed"), err)
	}

	data, err := json.MarshalIndent(projectProfile, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectProfileMarshalFailed"), err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(r.path, data, 0644); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectProfileWriteFailed"), err)
	}
	return nil
}

// GetForProject 读取 workspace 子项目画像
func (r *Repository) GetForProject(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	path := r.projectPath(projectID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("%s: %w", i18n.Get("ProjectProfileReadFailed"), err)
	}

	var profile domain.ProjectProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ProjectProfileParseFailed"), err)
	}
	return &profile, nil
}

// SaveForProject 保存 workspace 子项目画像
func (r *Repository) SaveForProject(ctx context.Context, projectID string, projectProfile *domain.ProjectProfile) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if projectProfile == nil {
		return fmt.Errorf("%s", i18n.Get("ProjectProfileNil"))
	}
	path := r.projectPath(projectID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectProfileCreateDirFailed"), err)
	}

	data, err := json.MarshalIndent(projectProfile, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectProfileMarshalFailed"), err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectProfileWriteFailed"), err)
	}
	return nil
}

func (r *Repository) projectPath(projectID string) string {
	return filepath.Join(filepath.Dir(r.path), "projects", projectID, "project-profile.json")
}

// GetSpec 读取单项目开发规范
func (r *Repository) GetSpec(ctx context.Context) (*domain.ProjectSpec, error) {
	return r.readSpec(ctx, filepath.Join(filepath.Dir(r.path), "project-spec.json"))
}

// SaveSpec 保存单项目开发规范
func (r *Repository) SaveSpec(ctx context.Context, spec *domain.ProjectSpec) error {
	return r.writeSpec(ctx, filepath.Join(filepath.Dir(r.path), "project-spec.json"), spec)
}

// GetSpecForProject 读取 workspace 子项目开发规范
func (r *Repository) GetSpecForProject(ctx context.Context, projectID string) (*domain.ProjectSpec, error) {
	return r.readSpec(ctx, r.projectSpecPath(projectID))
}

// SaveSpecForProject 保存 workspace 子项目开发规范
func (r *Repository) SaveSpecForProject(ctx context.Context, projectID string, spec *domain.ProjectSpec) error {
	return r.writeSpec(ctx, r.projectSpecPath(projectID), spec)
}

func (r *Repository) readSpec(ctx context.Context, path string) (*domain.ProjectSpec, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSpecNotFound
		}
		return nil, fmt.Errorf("%s: %w", i18n.Get("ProjectSpecReadFailed"), err)
	}

	var spec domain.ProjectSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("ProjectSpecParseFailed"), err)
	}
	return &spec, nil
}

func (r *Repository) writeSpec(ctx context.Context, path string, spec *domain.ProjectSpec) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if spec == nil {
		return fmt.Errorf("%s", i18n.Get("ProjectSpecNil"))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectSpecCreateDirFailed"), err)
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectSpecMarshalFailed"), err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("ProjectSpecWriteFailed"), err)
	}
	return nil
}

func (r *Repository) projectSpecPath(projectID string) string {
	return filepath.Join(filepath.Dir(r.path), "projects", projectID, "project-spec.json")
}
