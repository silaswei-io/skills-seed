package generate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

var sleepAfterGenerateChildStep = time.Sleep

type generateOptions struct {
	outputPath    string
	outputChanged bool
	merge         bool
}

// Cmd 返回 generate 命令
func Cmd(cont *container.Container) *cobra.Command {
	generateCmd := &cobra.Command{
		Use:     "generate",
		Short:   i18n.Get("GenerateShort"),
		Long:    i18n.Get("GenerateLongDesc"),
		Example: i18n.Get("GenerateExample"),
	}
	generateCmd.AddCommand(skillsCmd(cont))
	return generateCmd
}

func skillsCmd(cont *container.Container) *cobra.Command {
	opts := generateOptions{}
	cmd := &cobra.Command{
		Use:     "skills",
		Short:   i18n.Get("GenerateSkillsShort"),
		Long:    i18n.Get("GenerateSkillsLongDesc"),
		Example: i18n.Get("GenerateSkillsExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 检查 container 是否初始化
			if cont == nil {
				logger.Error(i18n.Get("GenerateNotInitialized"))
				logger.Debug(i18n.Get("GenerateRunInitFirst"))
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			opts.outputChanged = cmd.Flags().Changed("output")
			return runGenerate(cont, opts)
		},
	}

	// 添加 flags
	defaultOutputPath := ""
	if cont != nil {
		defaultOutputPath = outputPathForCurrentTarget(cont)
	}
	opts.outputPath = defaultOutputPath
	cmd.Flags().StringVarP(&opts.outputPath, "output", "o", defaultOutputPath, i18n.Get("GenerateFlagOutput"))
	cmd.Flags().BoolVarP(&opts.merge, "merge", "m", false, i18n.Get("GenerateFlagMerge"))

	return cmd
}

// RunGenerate 导出：生成 skills（供 sync 调用）
func RunGenerate(cont *container.Container) error {
	return runGenerate(cont, generateOptions{})
}

func runGenerate(cont *container.Container, opts generateOptions) error {
	ctx := runtimecontext.WithSeedPath(context.Background(), cont.SeedPath)

	logger.Info(i18n.Get("GenerateStarting"))
	if err := commandutil.LockConfiguredMode(ctx, cont); err != nil {
		return err
	}

	// 获取模式数量
	var count int
	isWorkspaceMode := cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace
	var tracker *progress.Tracker
	if !isWorkspaceMode {
		tracker = progress.New(2)
		if err := tracker.RunStep(i18n.Get("ProgressGenerateCountPatterns"), func() error {
			var countErr error
			count, countErr = cont.PatternRepo.Count(ctx)
			return countErr
		}); err != nil {
			logger.Error(i18n.GetWithParams("GenerateCountFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
	} else {
		var countErr error
		count, countErr = cont.PatternRepo.Count(ctx)
		if countErr != nil {
			logger.Error(i18n.GetWithParams("GenerateCountFailed", map[string]interface{}{"Error": countErr.Error()}))
			return countErr
		}
	}

	if count == 0 && !isWorkspaceMode {
		logger.Warn(i18n.Get("GenerateNoPatterns"))
		return nil
	}
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return err
	}

	logger.Debug(i18n.GetWithParams("GenerateFoundPatterns", map[string]interface{}{"Count": count}) + "\n")

	effectiveOutputPath := opts.outputPath
	if !opts.outputChanged {
		effectiveOutputPath = outputPathForCurrentTarget(cont)
	}

	// 如果指定了 --merge 标志，先合并模式
	if opts.merge {
		logger.Warn(i18n.Get("GenerateMergeDeprecated"))
		logger.Info(i18n.Get("GenerateMergeStarting"))
		if _, err := cont.MergerSvc.MergePatterns(ctx, &merger.MergePatternsRequest{}); err != nil {
			logger.Error(i18n.GetWithParams("GenerateMergeFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
		logger.Info(i18n.Get("GenerateMergeCompleted"))

		// 重新获取合并后的模式数量
		var err error
		count, err = cont.PatternRepo.Count(ctx)
		if err != nil {
			logger.Error(i18n.GetWithParams("GenerateCountFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
		logger.Info(i18n.GetWithParams("GenerateMergedCount", map[string]interface{}{"Count": count}))
	}

	// 生成 Skills
	if isWorkspaceMode {
		childProgress := progress.NewMulti(commandutil.WorkspaceProjectProgressNames(cont.ConfigRepo.GetWorkspaceConfig().Projects))
		childProgress.SetLabel(i18n.Get("ProgressGenerateWorkspaceProjects"))
		childProgress.SetTaskTotal(generator.GenerateProjectStepTotal)
		if err := generateWorkspaceChildSkills(ctx, cont, childProgress); err != nil {
			logger.Error(i18n.GetWithParams("GenerateFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
		rootTracker := progress.New(1)
		rootLabel := i18n.Get("ProgressGenerateWriteRootSkills")
		retryProgress := agent.NewRetryProgressBinder(rootTracker.UpdateStep)
		rootCtx := retryProgress.WithContext(ctx)
		if err := rootTracker.RunStep(rootLabel, func() error {
			retryProgress.StartStep(rootLabel)
			callErr := cont.GeneratorSvc.GenerateSkills(rootCtx, effectiveOutputPath)
			retryProgress.FinishStep(rootLabel, callErr == nil)
			return callErr
		}); err != nil {
			logger.Error(i18n.GetWithParams("GenerateFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
	} else {
		generateLabel := i18n.Get("ProgressGenerateWriteSkills")
		retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
		generateCtx := retryProgress.WithContext(ctx)
		if err := tracker.RunStep(generateLabel, func() error {
			retryProgress.StartStep(generateLabel)
			callErr := cont.GeneratorSvc.GenerateSkills(generateCtx, effectiveOutputPath)
			retryProgress.FinishStep(generateLabel, callErr == nil)
			return callErr
		}); err != nil {
			logger.Error(i18n.GetWithParams("GenerateFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
	}

	logger.Info(i18n.Get("GenerateSuccessMsg"))
	logger.Info(i18n.GetWithParams("GenerateOutputPath", map[string]interface{}{"Path": effectiveOutputPath}))
	if err := commandutil.MarkSkillsGenerated(ctx, cont); err != nil {
		return err
	}

	return nil
}

func generateWorkspaceChildSkills(ctx context.Context, cont *container.Container, trackers ...*progress.MultiTracker) error {
	workspaceConfig := cont.ConfigRepo.GetWorkspaceConfig()
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}

	projectRoot := projectConfig.RootPath
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	parallelism := workspacediscovery.EffectiveParallelism(domain.ModeWorkspace, cont.ConfigRepo.GetAgentConfig().Parallelism, len(workspaceConfig.Projects))
	var multiTracker *progress.MultiTracker
	if len(trackers) > 0 {
		multiTracker = trackers[0]
	}
	return workspacediscovery.RunProjectTasks(ctx, workspaceConfig.Projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
		projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
		if err != nil {
			return err
		}
		progressName := commandutil.WorkspaceProjectProgressName(project)
		stepStartedAt := time.Now()
		startStep := func(label string) {
			if multiTracker != nil {
				stepStartedAt = time.Now()
				multiTracker.Start(progressName, label)
			}
		}
		updateStep := func(label string) {
			if multiTracker != nil {
				multiTracker.Update(progressName, label)
			}
		}
		completeStep := func(label string) {
			if multiTracker != nil {
				multiTracker.CompleteStep(progressName, label)
				pauseAfterFastGenerateChildStep(stepStartedAt)
			}
		}
		failProgress := true
		defer func() {
			if failProgress && multiTracker != nil {
				multiTracker.Fail(progressName, i18n.Get("GenerateWorkspaceProjectProgressFailed"))
			}
		}()
		startStep(i18n.Get("ProgressGenerateResolveOutput"))
		logger.Info(i18n.GetWithParams("GenerateWorkspaceChildGenerating", map[string]interface{}{"ProjectName": project.ID}))
		childCont, err := commandutil.OpenWorkspaceChildContainer(ctx, projectRootPath, project, commandutil.WorkspaceChildErrorKeys{
			NotInitialized: "GenerateWorkspaceChildNotInitialized",
			NotGitRepo:     "GenerateWorkspaceChildNotGitRepo",
			ModeInvalid:    "GenerateWorkspaceChildModeInvalid",
		})
		if err != nil {
			return err
		}
		defer childCont.Close()

		if err := commandutil.LockConfiguredMode(ctx, childCont); err != nil {
			return err
		}
		scope := project.ID
		if scope == "" {
			scope = project.Path
		}
		childCtx := agent.WithTokenUsageScope(runtimecontext.WithoutUserContext(ctx), scope)
		childCtx = runtimecontext.WithSeedPath(childCtx, childCont.SeedPath)
		if err := commandutil.RequireAgentAvailable(childCont); err != nil {
			return err
		}
		childOutputPath := outputPathForCurrentTarget(childCont)
		if err := childCont.GeneratorSvc.GenerateSkillsWithHooks(childCtx, childOutputPath, generator.GenerateProgressHooks{
			OnStepStart:    startStep,
			OnStepUpdate:   updateStep,
			OnStepComplete: completeStep,
		}); err != nil {
			var manualErr *generator.ManualSkillExistsError
			if errors.As(err, &manualErr) {
				logger.Warn(i18n.GetWithParams("GenerateWorkspaceChildManualSkillSkipped", map[string]interface{}{
					"ProjectName": project.ID,
					"Path":        manualErr.Path,
				}))
				if multiTracker != nil {
					multiTracker.Complete(progressName, i18n.Get("GenerateWorkspaceProjectProgressComplete"))
				}
				failProgress = false
				return nil
			}
			return err
		}
		logger.Info(i18n.GetWithParams("GenerateWorkspaceChildGenerated", map[string]interface{}{"ProjectName": project.ID}))
		if multiTracker != nil {
			multiTracker.Complete(progressName, i18n.Get("GenerateWorkspaceProjectProgressComplete"))
		}
		failProgress = false
		agent.FlushTokenUsageScope(childCtx)
		return nil
	})
}

func outputPathForCurrentTarget(cont *container.Container) string {
	if cont == nil || cont.ConfigRepo == nil {
		return ""
	}
	skillsConfig := cont.ConfigRepo.GetSkillsConfig()
	target := config.EffectiveSkillsTarget(cont.ConfigRepo.GetAgentConfig(), skillsConfig)
	return config.EffectiveSkillsPath(target, skillsConfig)
}

func pauseAfterFastGenerateChildStep(startedAt time.Time) {
	progress.PauseAfterFastStep(startedAt, sleepAfterGenerateChildStep)
}
