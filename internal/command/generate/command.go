package generate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	statestore "github.com/silaswei-io/skills-seed/internal/infra/storage/state"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	ws "github.com/silaswei-io/skills-seed/internal/service/workspace"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

var sleepAfterGenerateChildStep = time.Sleep

type generateOptions struct {
	outputPath    string
	outputChanged bool
	noReferences  bool
	force         bool
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
	cmd.Flags().BoolVar(&opts.noReferences, "no-references", false, i18n.Get("GenerateFlagNoReferences"))
	cmd.Flags().BoolVar(&opts.force, "force", false, i18n.Get("GenerateFlagForce"))

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

	// 生成 Skills
	if isWorkspaceMode {
		if err := runGenerateWorkspace(ctx, cont, opts); err != nil {
			return err
		}
	} else {
		if shouldSkipProjectGenerate(ctx, cont, opts) {
			logger.Info(i18n.Get("GenerateSkillsSkipped"))
			logger.Info(i18n.Get("GenerateSuccessMsg"))
			logger.Info(i18n.GetWithParams("GenerateOutputPath", map[string]interface{}{"Path": effectiveOutputPath}))
			return nil
		}
		generateLabel := i18n.Get("ProgressGenerateWriteSkills")
		retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
		generateCtx := retryProgress.WithContext(ctx)
		if err := tracker.RunStep(generateLabel, func() error {
			retryProgress.StartStep(generateLabel)
			callErr := cont.GeneratorSvc.GenerateSkillsWithHooks(generateCtx, effectiveOutputPath, generator.GenerateProgressHooks{}, generator.GenerateOptions{SkipReferences: opts.noReferences})
			retryProgress.FinishStep(generateLabel, callErr == nil)
			return callErr
		}); err != nil {
			logger.Error(i18n.GetWithParams("GenerateFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
		if cont.StateRepo != nil {
			if err := cont.StateRepo.ClearSkillsDirty(ctx, domain.SkillsDirtyTarget{Project: true}); err != nil {
				return err
			}
		}
	}

	logger.Info(i18n.Get("GenerateSuccessMsg"))
	logger.Info(i18n.GetWithParams("GenerateOutputPath", map[string]interface{}{"Path": effectiveOutputPath}))
	if err := commandutil.MarkSkillsGenerated(ctx, cont); err != nil {
		return err
	}

	return nil
}

func runGenerateWorkspace(ctx context.Context, cont *container.Container, opts generateOptions) error {
	state, stateLoaded, err := loadRuntimeState(ctx, cont)
	if err != nil {
		return err
	}
	dirty := domain.SkillsDirtyState{}
	if stateLoaded {
		dirty = state.SkillsDirty
	}
	selection := workspaceGenerateSelectionForState(stateLoaded && state.SkillsGenerated, dirty, opts.force)
	if len(selection.projects) > 0 || selection.all {
		selectedProjects := selection.filter(cont.ConfigRepo.GetWorkspaceConfig().Projects)
		childProgress := progress.NewMulti(commandutil.WorkspaceProjectProgressNames(selectedProjects))
		childProgress.SetLabel(i18n.Get("ProgressGenerateWorkspaceProjects"))
		childProgress.SetTaskTotal(ws.GenerateProjectStepTotal)
		if err := generateWorkspaceChildSkillsSelected(ctx, cont, selection, childProgress); err != nil {
			logger.Error(i18n.GetWithParams("GenerateFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
	}
	shouldGenerateRoot := opts.force || dirty.Workspace || len(dirty.Projects) > 0 || !stateLoaded || !state.SkillsGenerated
	if shouldGenerateRoot {
		rootTracker := progress.New(1)
		rootLabel := i18n.Get("ProgressGenerateWriteRootSkills")
		retryProgress := agent.NewRetryProgressBinder(rootTracker.UpdateStep)
		rootCtx := retryProgress.WithContext(ctx)
		if err := rootTracker.RunStep(rootLabel, func() error {
			retryProgress.StartStep(rootLabel)
			callErr := cont.WorkspaceGeneratorSvc.GenerateWorkspaceSkills(rootCtx)
			retryProgress.FinishStep(rootLabel, callErr == nil)
			return callErr
		}); err != nil {
			logger.Error(i18n.GetWithParams("GenerateFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}
	}
	if cont.StateRepo != nil {
		if err := cont.StateRepo.ClearSkillsDirty(ctx, domain.SkillsDirtyTarget{Workspace: shouldGenerateRoot, Projects: dirty.Projects}); err != nil {
			return err
		}
	}
	if !shouldGenerateRoot && len(selection.projects) == 0 && !selection.all {
		logger.Info(i18n.Get("GenerateWorkspaceRootSkipped"))
	}
	return nil
}

type workspaceGenerateSelection struct {
	all      bool
	projects map[string]struct{}
}

func workspaceGenerateSelectionForState(skillsGenerated bool, dirty domain.SkillsDirtyState, force bool) workspaceGenerateSelection {
	if force || !skillsGenerated {
		return workspaceGenerateSelection{all: true}
	}
	selected := make(map[string]struct{}, len(dirty.Projects))
	for _, projectID := range dirty.Projects {
		projectID = strings.TrimSpace(projectID)
		if projectID != "" {
			selected[projectID] = struct{}{}
		}
	}
	return workspaceGenerateSelection{projects: selected}
}

func (s workspaceGenerateSelection) includes(project config.WorkspaceProjectConfig) bool {
	if s.all {
		return true
	}
	if len(s.projects) == 0 {
		return false
	}
	if _, ok := s.projects[project.ID]; ok && project.ID != "" {
		return true
	}
	if _, ok := s.projects[project.Path]; ok && project.Path != "" {
		return true
	}
	return false
}

func (s workspaceGenerateSelection) filter(projects []config.WorkspaceProjectConfig) []config.WorkspaceProjectConfig {
	selected := make([]config.WorkspaceProjectConfig, 0, len(projects))
	for _, project := range projects {
		if s.includes(project) {
			selected = append(selected, project)
		}
	}
	return selected
}

func shouldSkipProjectGenerate(ctx context.Context, cont *container.Container, opts generateOptions) bool {
	if opts.force || opts.outputChanged || opts.noReferences || cont == nil || cont.StateRepo == nil {
		return false
	}
	state, stateLoaded, err := loadRuntimeState(ctx, cont)
	if err != nil || !stateLoaded {
		return false
	}
	return state.SkillsGenerated && !state.SkillsDirty.Project
}

func loadRuntimeState(ctx context.Context, cont *container.Container) (*domain.RuntimeState, bool, error) {
	if cont == nil || cont.StateRepo == nil {
		return nil, false, nil
	}
	state, err := cont.StateRepo.Get(ctx)
	if err != nil {
		if errors.Is(err, statestore.ErrStateNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return state, true, nil
}

func generateWorkspaceChildSkills(ctx context.Context, cont *container.Container, trackers ...*progress.MultiTracker) error {
	return generateWorkspaceChildSkillsSelected(ctx, cont, workspaceGenerateSelection{all: true}, trackers...)
}

func generateWorkspaceChildSkillsSelected(ctx context.Context, cont *container.Container, selection workspaceGenerateSelection, trackers ...*progress.MultiTracker) error {
	workspaceConfig := cont.ConfigRepo.GetWorkspaceConfig()
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}
	projects := selection.filter(workspaceConfig.Projects)
	if len(projects) == 0 {
		return nil
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
	return workspacediscovery.RunProjectTasks(ctx, projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
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
		}, generator.GenerateOptions{}); err != nil {
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
	return cont.ConfigRepo.GetEffectiveSkillsPath()
}

func pauseAfterFastGenerateChildStep(startedAt time.Time) {
	progress.PauseAfterFastStep(startedAt, sleepAfterGenerateChildStep)
}
