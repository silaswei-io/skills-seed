package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

const defaultWorkspaceParallelismCap = 6

// EffectiveParallelism 计算实际并发数
func EffectiveParallelism(mode string, configured, projectCount int) int {
	if configured > 0 {
		return configured
	}
	if mode != domain.ModeWorkspace {
		return 1
	}
	if projectCount <= 0 {
		return 1
	}
	if projectCount > defaultWorkspaceParallelismCap {
		return defaultWorkspaceParallelismCap
	}
	return projectCount
}

// ProjectTask 是一个子项目并发任务
type ProjectTask func(ctx context.Context, project config.WorkspaceProjectConfig) error

// RunProjectTasks 按并发上限执行子项目任务
func RunProjectTasks(ctx context.Context, projects []config.WorkspaceProjectConfig, parallelism int, task ProjectTask) error {
	if len(projects) == 0 {
		return nil
	}
	if parallelism <= 0 {
		parallelism = 1
	}
	if parallelism > len(projects) {
		parallelism = len(projects)
	}

	ordered := append([]config.WorkspaceProjectConfig(nil), projects...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Path < ordered[j].Path
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan config.WorkspaceProjectConfig)
	errs := make(chan error, len(ordered))
	var wg sync.WaitGroup

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for project := range jobs {
				if err := task(ctx, project); err != nil {
					errs <- fmt.Errorf("%s: %w", project.ID, err)
					cancel()
					return
				}
			}
		}()
	}

	for _, project := range ordered {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					return err
				}
			}
			return ctx.Err()
		case jobs <- project:
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// ProjectRoot 返回子项目绝对路径
func ProjectRoot(workspaceRoot string, project config.WorkspaceProjectConfig) string {
	return filepath.Join(workspaceRoot, filepath.FromSlash(project.Path))
}

// ResolveProjectRoot 返回子项目绝对路径，并确保配置路径没有逃逸工作区根目录。
func ResolveProjectRoot(workspaceRoot string, project config.WorkspaceProjectConfig) (string, error) {
	rootAbs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", err
	}
	projectAbs, err := filepath.Abs(ProjectRoot(rootAbs, project))
	if err != nil {
		return "", err
	}
	rootAbs = filepath.Clean(rootAbs)
	projectAbs = filepath.Clean(projectAbs)

	relPath, err := filepath.Rel(rootAbs, projectAbs)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%s", i18n.GetWithParams("WorkspaceProjectPathOutsideRoot", map[string]interface{}{
			"Path": project.Path,
			"Root": workspaceRoot,
		}))
	}
	return projectAbs, nil
}
