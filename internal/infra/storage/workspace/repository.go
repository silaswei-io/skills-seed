package workspace

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

// ErrProfileNotFound 表示 workspace-profile.json 不存在
var ErrProfileNotFound = errors.New("workspace profile not found")

// ErrSpecNotFound 表示 workspace-spec.json 不存在
var ErrSpecNotFound = errors.New("workspace spec not found")

// ProfileRepository 保存 workspace-profile.json
type ProfileRepository struct {
	path string
}

// NewProfileRepository 创建工作区画像仓储
func NewProfileRepository(seedPath string) *ProfileRepository {
	return &ProfileRepository{path: filepath.Join(seedPath, "memory", "workspace-profile.json")}
}

// Get 读取工作区画像
func (r *ProfileRepository) Get(ctx context.Context) (*domain.WorkspaceProfile, error) {
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
		return nil, fmt.Errorf("%s: %w", i18n.Get("WorkspaceProfileReadFailed"), err)
	}

	var profile domain.WorkspaceProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("WorkspaceProfileParseFailed"), err)
	}
	return &profile, nil
}

// Save 写入工作区画像
func (r *ProfileRepository) Save(ctx context.Context, profile *domain.WorkspaceProfile) error {
	if profile == nil {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProfileNil"))
	}
	return writeJSON(ctx, r.path, profile, workspaceJSONLabels{
		CreateDir: i18n.Get("WorkspaceProfileCreateDirFailed"),
		Marshal:   i18n.Get("WorkspaceProfileMarshalFailed"),
		Write:     i18n.Get("WorkspaceProfileWriteFailed"),
	})
}

// SpecRepository 保存 workspace-spec.json
type SpecRepository struct {
	path string
}

// NewSpecRepository 创建工作区规范仓储
func NewSpecRepository(seedPath string) *SpecRepository {
	return &SpecRepository{path: filepath.Join(seedPath, "memory", "workspace-spec.json")}
}

// Get 读取工作区规范
func (r *SpecRepository) Get(ctx context.Context) (*domain.WorkspaceSpec, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSpecNotFound
		}
		return nil, fmt.Errorf("%s: %w", i18n.Get("WorkspaceSpecReadFailed"), err)
	}

	var spec domain.WorkspaceSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("WorkspaceSpecParseFailed"), err)
	}
	return &spec, nil
}

// Save 写入工作区规范
func (r *SpecRepository) Save(ctx context.Context, spec *domain.WorkspaceSpec) error {
	if spec == nil {
		return fmt.Errorf("%s", i18n.Get("WorkspaceSpecNil"))
	}
	return writeJSON(ctx, r.path, spec, workspaceJSONLabels{
		CreateDir: i18n.Get("WorkspaceSpecCreateDirFailed"),
		Marshal:   i18n.Get("WorkspaceSpecMarshalFailed"),
		Write:     i18n.Get("WorkspaceSpecWriteFailed"),
	})
}

type workspaceJSONLabels struct {
	CreateDir string
	Marshal   string
	Write     string
}

func writeJSON(ctx context.Context, path string, value interface{}, labels workspaceJSONLabels) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("%s: %w", labels.CreateDir, err)
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", labels.Marshal, err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("%s: %w", labels.Write, err)
	}
	return nil
}
