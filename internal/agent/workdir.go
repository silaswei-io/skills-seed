package agent

import (
	"context"
	"os"

	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
)

// WorkDirForContext 返回绑定在 ctx 上的项目根目录；未绑定时返回当前目录。
func WorkDirForContext(ctx context.Context) (string, error) {
	if projectRoot := runtimecontext.ProjectRoot(ctx); projectRoot != "" {
		return projectRoot, nil
	}
	return os.Getwd()
}
