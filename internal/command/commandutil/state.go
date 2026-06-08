package commandutil

import (
	"context"
	"errors"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
)

// LockConfiguredMode 锁定当前配置中的单项目或工作区模式。
func LockConfiguredMode(ctx context.Context, cont *container.Container) error {
	if cont == nil || cont.StateRepo == nil || cont.ConfigRepo == nil {
		return nil
	}
	mode := cont.ConfigRepo.GetProjectConfig().Mode
	_, err := cont.StateRepo.LockMode(ctx, mode)
	if err == nil {
		return nil
	}

	var modeErr *statestore.ModeLockedError
	if errors.As(err, &modeErr) {
		return errors.New(i18n.GetWithParams("ProjectModeLocked", map[string]interface{}{
			"LockedMode": modeErr.LockedMode,
			"ConfigMode": modeErr.ConfigMode,
		}))
	}
	return err
}

// MarkLearned 锁定当前模式并标记已学习
func MarkLearned(ctx context.Context, cont *container.Container) error {
	if cont == nil || cont.StateRepo == nil || cont.ConfigRepo == nil {
		return nil
	}
	if err := LockConfiguredMode(ctx, cont); err != nil {
		return err
	}
	return cont.StateRepo.MarkLearned(ctx, cont.ConfigRepo.GetProjectConfig().Mode)
}

// MarkSkillsGenerated 锁定当前模式并标记已生成 Skills。
func MarkSkillsGenerated(ctx context.Context, cont *container.Container) error {
	if cont == nil || cont.StateRepo == nil || cont.ConfigRepo == nil {
		return nil
	}
	if err := LockConfiguredMode(ctx, cont); err != nil {
		return err
	}
	return cont.StateRepo.MarkSkillsGenerated(ctx, cont.ConfigRepo.GetProjectConfig().Mode)
}
