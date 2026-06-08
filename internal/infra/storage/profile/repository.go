package profile

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/jsonfile"
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
	return profileStore(r.path).Get(ctx)
}

// Save 保存项目画像
func (r *Repository) Save(ctx context.Context, projectProfile *domain.ProjectProfile) error {
	return profileStore(r.path).Save(ctx, projectProfile)
}

// GetForProject 读取工作区子项目画像
func (r *Repository) GetForProject(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
	return profileStore(r.projectPath(projectID)).Get(ctx)
}

// SaveForProject 保存工作区子项目画像
func (r *Repository) SaveForProject(ctx context.Context, projectID string, projectProfile *domain.ProjectProfile) error {
	return profileStore(r.projectPath(projectID)).Save(ctx, projectProfile)
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

// GetSpecForProject 读取工作区子项目开发规范
func (r *Repository) GetSpecForProject(ctx context.Context, projectID string) (*domain.ProjectSpec, error) {
	return r.readSpec(ctx, r.projectSpecPath(projectID))
}

// SaveSpecForProject 保存工作区子项目开发规范
func (r *Repository) SaveSpecForProject(ctx context.Context, projectID string, spec *domain.ProjectSpec) error {
	return r.writeSpec(ctx, r.projectSpecPath(projectID), spec)
}

func (r *Repository) readSpec(ctx context.Context, path string) (*domain.ProjectSpec, error) {
	return specStore(path).Get(ctx)
}

func (r *Repository) writeSpec(ctx context.Context, path string, spec *domain.ProjectSpec) error {
	return specStore(path).Save(ctx, spec)
}

func (r *Repository) projectSpecPath(projectID string) string {
	return filepath.Join(filepath.Dir(r.path), "projects", projectID, "project-spec.json")
}

func profileStore(path string) jsonfile.Store[domain.ProjectProfile] {
	return jsonfile.Store[domain.ProjectProfile]{
		Path:     path,
		NotFound: ErrProfileNotFound,
		NilValue: fmt.Errorf("%s", i18n.Get("ProjectProfileNil")),
		Labels: jsonfile.Labels{
			Read:      i18n.Get("ProjectProfileReadFailed"),
			Parse:     i18n.Get("ProjectProfileParseFailed"),
			CreateDir: i18n.Get("ProjectProfileCreateDirFailed"),
			Marshal:   i18n.Get("ProjectProfileMarshalFailed"),
			Write:     i18n.Get("ProjectProfileWriteFailed"),
		},
	}
}

func specStore(path string) jsonfile.Store[domain.ProjectSpec] {
	return jsonfile.Store[domain.ProjectSpec]{
		Path:     path,
		NotFound: ErrSpecNotFound,
		NilValue: fmt.Errorf("%s", i18n.Get("ProjectSpecNil")),
		Labels: jsonfile.Labels{
			Read:      i18n.Get("ProjectSpecReadFailed"),
			Parse:     i18n.Get("ProjectSpecParseFailed"),
			CreateDir: i18n.Get("ProjectSpecCreateDirFailed"),
			Marshal:   i18n.Get("ProjectSpecMarshalFailed"),
			Write:     i18n.Get("ProjectSpecWriteFailed"),
		},
	}
}
