package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/jsonfile"
)

// ErrProfileNotFound 表示工作区画像文件不存在。
var ErrProfileNotFound = errors.New("workspace profile not found")

// ErrSpecNotFound 表示工作区规范文件不存在。
var ErrSpecNotFound = errors.New("workspace spec not found")

// ProfileRepository 保存工作区画像文件。
type ProfileRepository struct {
	path string
}

// NewProfileRepository 创建工作区画像仓储
func NewProfileRepository(seedPath string) *ProfileRepository {
	return &ProfileRepository{path: filepath.Join(seedPath, "memory", "workspace-profile.json")}
}

// Get 读取工作区画像
func (r *ProfileRepository) Get(ctx context.Context) (*domain.WorkspaceProfile, error) {
	return workspaceProfileStore(r.path).Get(ctx)
}

// Save 写入工作区画像
func (r *ProfileRepository) Save(ctx context.Context, profile *domain.WorkspaceProfile) error {
	return workspaceProfileStore(r.path).Save(ctx, profile)
}

// SpecRepository 保存工作区规范文件。
type SpecRepository struct {
	path string
}

// NewSpecRepository 创建工作区规范仓储
func NewSpecRepository(seedPath string) *SpecRepository {
	return &SpecRepository{path: filepath.Join(seedPath, "memory", "workspace-spec.json")}
}

// Get 读取工作区规范
func (r *SpecRepository) Get(ctx context.Context) (*domain.WorkspaceSpec, error) {
	return workspaceSpecStore(r.path).Get(ctx)
}

// Save 写入工作区规范
func (r *SpecRepository) Save(ctx context.Context, spec *domain.WorkspaceSpec) error {
	return workspaceSpecStore(r.path).Save(ctx, spec)
}

func workspaceProfileStore(path string) jsonfile.Store[domain.WorkspaceProfile] {
	return jsonfile.Store[domain.WorkspaceProfile]{
		Path:     path,
		NotFound: ErrProfileNotFound,
		NilValue: fmt.Errorf("%s", i18n.Get("WorkspaceProfileNil")),
		Labels: jsonfile.Labels{
			Read:      i18n.Get("WorkspaceProfileReadFailed"),
			Parse:     i18n.Get("WorkspaceProfileParseFailed"),
			CreateDir: i18n.Get("WorkspaceProfileCreateDirFailed"),
			Marshal:   i18n.Get("WorkspaceProfileMarshalFailed"),
			Write:     i18n.Get("WorkspaceProfileWriteFailed"),
		},
	}
}

func workspaceSpecStore(path string) jsonfile.Store[domain.WorkspaceSpec] {
	return jsonfile.Store[domain.WorkspaceSpec]{
		Path:     path,
		NotFound: ErrSpecNotFound,
		NilValue: fmt.Errorf("%s", i18n.Get("WorkspaceSpecNil")),
		Labels: jsonfile.Labels{
			Read:      i18n.Get("WorkspaceSpecReadFailed"),
			Parse:     i18n.Get("WorkspaceSpecParseFailed"),
			CreateDir: i18n.Get("WorkspaceSpecCreateDirFailed"),
			Marshal:   i18n.Get("WorkspaceSpecMarshalFailed"),
			Write:     i18n.Get("WorkspaceSpecWriteFailed"),
		},
	}
}
