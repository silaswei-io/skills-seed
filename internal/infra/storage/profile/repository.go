package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/jsonfile"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
)

// ErrProfileNotFound 表示项目画像不存在
var ErrProfileNotFound = errors.New("project profile not found")

// Repository 保存项目画像 JSON 文档。
type Repository struct {
	layout layout.Layout
}

// NewRepository 创建项目画像仓储
func NewRepository(seedPath string) *Repository {
	return &Repository{
		layout: layout.New(seedPath),
	}
}

// Path 返回项目画像文件路径
func (r *Repository) Path() string {
	return r.layout.ProjectProfile()
}

// Get 读取项目画像
func (r *Repository) Get(ctx context.Context) (*domain.ProjectProfile, error) {
	return profileStore(r.layout.ProjectProfile()).Get(ctx)
}

// Save 保存项目画像
func (r *Repository) Save(ctx context.Context, projectProfile *domain.ProjectProfile) error {
	projectProfile = domain.CleanProjectProfile(projectProfile)
	return profileStore(r.layout.ProjectProfile()).Save(ctx, projectProfile)
}

// GetForProject 读取工作区子项目画像
func (r *Repository) GetForProject(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
	return profileStore(r.projectPath(projectID)).Get(ctx)
}

// SaveForProject 保存工作区子项目画像
func (r *Repository) SaveForProject(ctx context.Context, projectID string, projectProfile *domain.ProjectProfile) error {
	projectProfile = domain.CleanProjectProfile(projectProfile)
	return profileStore(r.projectPath(projectID)).Save(ctx, projectProfile)
}

func (r *Repository) projectPath(projectID string) string {
	return r.layout.ProjectDocument(projectID, "project-profile.json")
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
