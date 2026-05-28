package commandutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

// WorkspaceChildErrorKeys 携带子项目校验所需的命令专属消息键。
type WorkspaceChildErrorKeys struct {
	NotInitialized string
	NotGitRepo     string
	ModeInvalid    string
}

// OpenWorkspaceChildContainer 打开已独立初始化的 workspace 子项目容器。
func OpenWorkspaceChildContainer(ctx context.Context, projectRootPath string, project config.WorkspaceProjectConfig, keys WorkspaceChildErrorKeys) (*container.Container, error) {
	childSeedPath := filepath.Join(projectRootPath, ".skills-seed")
	childConfigPath := filepath.Join(childSeedPath, "config.yaml")
	if _, err := os.Stat(childConfigPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s", i18n.GetWithParams(keys.NotInitialized, map[string]interface{}{
				"ProjectName": project.ID,
				"Path":        projectRootPath,
			}))
		}
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(projectRootPath, ".git")); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s", i18n.GetWithParams(keys.NotGitRepo, map[string]interface{}{
				"ProjectName": project.ID,
				"Path":        projectRootPath,
			}))
		}
		return nil, err
	}

	childCont, err := container.NewContainer(ctx, childSeedPath)
	if err != nil {
		return nil, err
	}
	if childCont.ConfigRepo.GetProjectConfig().Mode != domain.ModeProject {
		_ = childCont.Close()
		return nil, fmt.Errorf("%s", i18n.GetWithParams(keys.ModeInvalid, map[string]interface{}{
			"ProjectName": project.ID,
			"Mode":        childCont.ConfigRepo.GetProjectConfig().Mode,
		}))
	}
	return childCont, nil
}
