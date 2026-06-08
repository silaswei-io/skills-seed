package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
)

// ErrStateNotFound 表示运行状态文件尚不存在
var ErrStateNotFound = errors.New("runtime state not found")

// ErrModeLocked 表示配置中的模式与已锁定模式不一致
var ErrModeLocked = errors.New("project mode is locked")

// Repository 把运行状态保存到 .skills-seed/memory
type Repository struct {
	path string
}

// NewRepository 创建运行状态仓储
func NewRepository(seedPath string) *Repository {
	return &Repository{path: filepath.Join(seedPath, "memory", "state.json")}
}

// Path 返回状态文件路径
func (r *Repository) Path() string {
	return r.path
}

// Get 读取运行状态
func (r *Repository) Get(ctx context.Context) (*domain.RuntimeState, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrStateNotFound
		}
		return nil, fmt.Errorf("read runtime state: %w", err)
	}

	var state domain.RuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse runtime state: %w", err)
	}
	return &state, nil
}

// Save 写入运行状态
func (r *Repository) Save(ctx context.Context, state *domain.RuntimeState) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if state == nil {
		return fmt.Errorf("runtime state is nil")
	}
	if state.Mode == "" {
		state.Mode = domain.ModeProject
	}
	state.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return fmt.Errorf("create runtime state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime state: %w", err)
	}
	data = append(data, '\n')

	if err := fileio.WriteFileAtomic(r.path, data, 0644); err != nil {
		return fmt.Errorf("write runtime state: %w", err)
	}
	return nil
}

// LockMode 记录当前模式，并禁止后续在单项目和工作区模式之间切换。
func (r *Repository) LockMode(ctx context.Context, mode string) (*domain.RuntimeState, error) {
	if mode == "" {
		mode = domain.ModeProject
	}

	current, err := r.Get(ctx)
	if err != nil {
		if !errors.Is(err, ErrStateNotFound) {
			return nil, err
		}
		current = &domain.RuntimeState{Mode: mode}
	}

	if current.Mode == "" {
		current.Mode = mode
	}
	if current.ModeLocked && current.Mode != mode {
		return nil, &ModeLockedError{LockedMode: current.Mode, ConfigMode: mode}
	}

	current.Mode = mode
	current.ModeLocked = true
	if err := r.Save(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

// ModeLockedError 携带已锁定模式和配置模式
type ModeLockedError struct {
	LockedMode string
	ConfigMode string
}

func (e *ModeLockedError) Error() string {
	return fmt.Sprintf("%v: initialized as %q but config requests %q", ErrModeLocked, e.LockedMode, e.ConfigMode)
}

func (e *ModeLockedError) Unwrap() error {
	return ErrModeLocked
}

// MarkLearned 锁定模式，并标记已经产生学习或画像数据
func (r *Repository) MarkLearned(ctx context.Context, mode string) error {
	state, err := r.LockMode(ctx, mode)
	if err != nil {
		return err
	}
	state.Learned = true
	return r.Save(ctx, state)
}

// MarkSkillsGenerated 锁定模式，并标记已经生成 skills
func (r *Repository) MarkSkillsGenerated(ctx context.Context, mode string) error {
	state, err := r.LockMode(ctx, mode)
	if err != nil {
		return err
	}
	state.SkillsGenerated = true
	return r.Save(ctx, state)
}
