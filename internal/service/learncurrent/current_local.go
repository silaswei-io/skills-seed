package learncurrent

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/service/navigation"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

func runLearnCurrentProjectWithOptions(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions) (*learnCurrentProjectResult, error) {
	if opts.semantic && len(opts.focusPaths) == 0 {
		return nil, fmt.Errorf("%s", i18n.Get("LearnSemanticFocusRequired"))
	}
	started := time.Now()
	project := cont.ConfigRepo.GetProjectConfig()
	root := project.RootPath
	if root == "" {
		root = filepath.Dir(cont.SeedPath)
	}
	totalSteps := 1
	if cont.FileTracker != nil && cont.PatternReader != nil {
		totalSteps = learnCurrentProjectStepTotal
	}
	steps := commandutil.NewConsoleStepRunner(commandutil.ConsoleStepRunnerOptions{TotalSteps: totalSteps, ShowProgress: opts.showProgress, OnStepStart: opts.onStepStart, OnStepComplete: opts.onStepComplete, OnStepUpdate: opts.onStepUpdate})
	result := &learnCurrentProjectResult{projectName: project.Name}
	var index *codegraph.Index
	err := steps.Run(i18n.Get("LearnLocalIndex"), func() error {
		var changed bool
		var err error
		index, changed, err = navigation.Refresh(ctx, cont.ConfigRepo, root)
		if err != nil {
			return err
		}
		result.navigationUpdated = changed || opts.force
		result.skipped = !result.navigationUpdated
		result.skippedCount = len(index.Unindexed)
		if opts.showDetailedLogs {
			logger.Info(i18n.GetWithParams("LearnLocalComplete", map[string]interface{}{"Files": len(index.Files), "Symbols": len(index.Nodes), "Unindexed": len(index.Unindexed)}))
		}
		return commandutil.LockConfiguredMode(ctx, cont)
	})
	if err == nil {
		err = supplementLocalProjectWithSteps(ctx, cont, opts, root, index, result, steps)
	}
	result.duration = time.Since(started)
	return result, err
}
