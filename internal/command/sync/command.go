package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	stdsync "sync"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/changelog"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/interactive"
	"github.com/silaswei-io/skills-seed/internal/service/syncflow"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
	"github.com/silaswei-io/skills-seed/internal/terminal/progress"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

type syncRunMode string

const (
	syncRunAuto    syncRunMode = "auto"
	syncRunResume  syncRunMode = "resume"
	syncRunRestart syncRunMode = "restart"
)

// Dependencies 描述 sync 命令需要调用的应用用例。
type Dependencies struct {
	LearnCurrent                func(cont *container.Container, req syncflow.LearnCurrentRequest, opts LearnCurrentOptions) (domain.LearnCurrentResult, error)
	Generate                    func(cont *container.Container) error
	GenerateChild               func(cont *container.Container) error
	LearnWorkspaceRelationships func(cont *container.Container, userContext string) (bool, error)
	GenerateWorkspaceRoot       func(cont *container.Container) error
}

// LearnCurrentOptions 描述 sync 命令层对子项目学习过程的展示控制。
type LearnCurrentOptions struct {
	Quiet          bool
	OnStepStart    func(label string)
	OnStepUpdate   func(label string)
	OnStepComplete func(label string)
}

const syncWorkspaceChildStepTotal = 8 // 子项目 learn current 7 步 + 子项目 skill 生成 1 步。

// Cmd 返回 sync 命令
func Cmd(cont *container.Container, deps ...Dependencies) *cobra.Command {
	dependencies := Dependencies{}
	if len(deps) > 0 {
		dependencies = deps[0]
	}
	userContext := ""
	contextPath := []string{}
	resume := false
	restart := false
	noInteractive := false

	cmd := &cobra.Command{
		Use:     "sync",
		Short:   i18n.Get("SyncShort"),
		Long:    i18n.Get("SyncLongDesc"),
		Example: i18n.Get("SyncExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			ctx := cmd.Context()
			stateScope := commandutil.CommandStateScopeForCobra(cmd)
			resolvedContext, err := commandutil.ResolveRuntimeContext(userContext, contextPath...)
			if err != nil {
				return err
			}
			inputs, err := normalizeSyncInputs(syncInputs{
				UserContext: resolvedContext,
			})
			if err != nil {
				return err
			}
			resolvedMode, err := syncModeFromFlags(resume, restart)
			if err != nil {
				return err
			}
			if shouldRunInteractiveSync(cmd, inputs.UserContext, noInteractive) {
				mode, err := resolveInteractiveSync(ctx, cmd, cont, stateScope)
				if err != nil {
					if errors.Is(err, interactive.ErrCanceled) {
						return nil
					}
					return err
				}
				resolvedMode = mode
			}
			if resolvedMode == syncRunRestart {
				if err := clearSyncCommandStates(cont, stateScope); err != nil {
					return err
				}
			}
			if resolvedMode == syncRunResume {
				resumable, err := hasResumableSyncCommandStateForTarget(ctx, cont, stateScope)
				if err != nil {
					return err
				}
				if !resumable {
					return fmt.Errorf("%s", i18n.Get("SyncResumeStateMissing"))
				}
			}
			change := changelog.Start(cont.SeedPath, "sync")
			if err := syncLearn(ctx, cont, stateScope, inputs.UserContext, resolvedMode, change, dependencies); err != nil {
				return err
			}
			return change.Save(i18n.Get("ChangeLogSummarySync"))
		},
	}

	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("SyncFlagContext"))
	cmd.Flags().StringArrayVar(&contextPath, "context-path", nil, i18n.Get("SyncFlagContextPath"))
	cmd.Flags().BoolVar(&resume, "resume", false, i18n.Get("SyncFlagResume"))
	cmd.Flags().BoolVar(&restart, "restart", false, i18n.Get("SyncFlagRestart"))
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, i18n.Get("InteractiveFlagNoInteractive"))

	return cmd
}

func syncModeFromFlags(resume, restart bool) (syncRunMode, error) {
	if resume && restart {
		return syncRunAuto, fmt.Errorf("%s", i18n.Get("SyncRunModeConflict"))
	}
	if resume {
		return syncRunResume, nil
	}
	if restart {
		return syncRunRestart, nil
	}
	return syncRunAuto, nil
}

type syncInputs struct {
	UserContext string
}

func normalizeSyncInputs(inputs syncInputs) (syncInputs, error) {
	inputs.UserContext = strings.TrimSpace(inputs.UserContext)
	return inputs, nil
}

func syncCommandStateSeedPaths(cont *container.Container) ([]string, error) {
	if cont == nil {
		return nil, nil
	}
	seedPaths := []string{cont.SeedPath}
	if cont.ConfigRepo == nil || cont.ConfigRepo.GetProjectConfig().Mode != domain.ModeWorkspace {
		return seedPaths, nil
	}
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	if strings.TrimSpace(projectRoot) == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
		if err != nil {
			return nil, err
		}
		seedPaths = append(seedPaths, filepath.Join(projectRootPath, ".skills-seed"))
	}
	return seedPaths, nil
}

func clearSyncCommandStates(cont *container.Container, stateScope string) error {
	seedPaths, err := syncCommandStateSeedPaths(cont)
	if err != nil {
		return err
	}
	for _, seedPath := range seedPaths {
		if err := commandstate.NewRepository(seedPath, stateScope).Clear(); err != nil {
			return err
		}
	}
	return nil
}

// syncLearn 路径 A：学习当前代码 → 生成 Skills。
func syncLearn(ctx context.Context, cont *container.Container, stateScope string, userContext string, mode syncRunMode, change *changelog.Builder, deps ...Dependencies) error {
	dependencies := Dependencies{}
	if len(deps) > 0 {
		dependencies = deps[0]
	}
	if cont != nil && cont.ConfigRepo != nil && cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return syncWorkspaceLearn(ctx, cont, stateScope, userContext, mode, change, dependencies)
	}
	var learnCurrent syncflow.LearnCurrentFunc
	if dependencies.LearnCurrent != nil {
		learnCurrent = func(ctx context.Context, req syncflow.LearnCurrentRequest) (domain.LearnCurrentResult, error) {
			return dependencies.LearnCurrent(cont, req, LearnCurrentOptions{})
		}
	}
	var generate syncflow.GenerateFunc
	if dependencies.Generate != nil {
		generate = func(ctx context.Context) error {
			return dependencies.Generate(cont)
		}
	}
	service := syncflow.Service{
		LearnCurrent: learnCurrent,
		Generate:     generate,
		OutputMissing: func() bool {
			return syncGeneratedSkillMissing(cont)
		},
	}
	return service.Run(ctx, syncflow.Request{
		Learn: syncflow.LearnCurrentRequest{
			StateScope:  stateScope,
			UserContext: userContext,
			Force:       mode == syncRunRestart,
		},
		Change: change,
	})
}

func syncWorkspaceLearn(ctx context.Context, cont *container.Container, stateScope string, userContext string, mode syncRunMode, change *changelog.Builder, dependencies Dependencies) error {
	if dependencies.LearnCurrent == nil {
		return fmt.Errorf("sync learn dependency is not configured")
	}
	if dependencies.GenerateChild == nil {
		return fmt.Errorf("sync child generate dependency is not configured")
	}
	if dependencies.LearnWorkspaceRelationships == nil {
		return fmt.Errorf("sync workspace relationships dependency is not configured")
	}
	if dependencies.GenerateWorkspaceRoot == nil {
		return fmt.Errorf("sync workspace root generate dependency is not configured")
	}

	workspaceConfig := cont.ConfigRepo.GetWorkspaceConfig()
	if len(workspaceConfig.Projects) == 0 {
		return fmt.Errorf("%s", i18n.Get("WorkspaceProjectsMissing"))
	}
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	projectRoot := projectConfig.RootPath
	if strings.TrimSpace(projectRoot) == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	parallelism := workspacediscovery.EffectiveParallelism(domain.ModeWorkspace, cont.ConfigRepo.GetAgentConfig().Parallelism, len(workspaceConfig.Projects))
	learnReq := syncflow.LearnCurrentRequest{
		StateScope:  stateScope,
		UserContext: userContext,
		Force:       mode == syncRunRestart,
	}
	var mu stdsync.Mutex
	changedProjects := map[string]bool{}
	childGenerated := false
	childProgress := progress.NewMulti(commandutil.WorkspaceProjectProgressNames(workspaceConfig.Projects))
	defer childProgress.Stop()
	childProgress.SetLabel(i18n.Get("ProgressLearnWorkspaceProjects"))
	childProgress.SetTaskTotal(syncWorkspaceChildStepTotal)

	logger.Info(i18n.Get("SyncStepLearn"))
	if err := workspacediscovery.RunProjectTasks(ctx, workspaceConfig.Projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
		childCont, err := syncOpenWorkspaceChild(ctx, projectRoot, project)
		if err != nil {
			return err
		}
		defer childCont.Close()

		progressName := commandutil.WorkspaceProjectProgressName(project)
		result, err := dependencies.LearnCurrent(childCont, learnReq, LearnCurrentOptions{
			Quiet: true,
			OnStepStart: func(label string) {
				childProgress.Start(progressName, label)
			},
			OnStepUpdate: func(label string) {
				childProgress.Update(progressName, label)
			},
			OnStepComplete: func(label string) {
				childProgress.CompleteStep(progressName, label)
			},
		})
		if err != nil {
			childProgress.Fail(progressName, i18n.Get("LearnWorkspaceProjectProgressFailed"))
			return fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
		}
		shouldGenerate := syncflow.ShouldGenerateAfterLearn(result) || syncGeneratedSkillMissing(childCont)
		if shouldGenerate {
			generateLabel := i18n.Get("ProgressGenerateWriteSkills")
			childProgress.Start(progressName, generateLabel)
			if err := dependencies.GenerateChild(childCont); err != nil {
				childProgress.Fail(progressName, i18n.Get("GenerateWorkspaceProjectProgressFailed"))
				return fmt.Errorf("%s: %w", i18n.Get("SyncGenerateFailed"), err)
			}
			childProgress.CompleteStep(progressName, generateLabel)
			childProgress.Complete(progressName, i18n.Get("GenerateWorkspaceProjectProgressComplete"))
		} else {
			childProgress.Complete(progressName, i18n.Get("SyncGenerateSkippedNoChanges"))
		}

		mu.Lock()
		if syncflow.ShouldGenerateAfterLearn(result) {
			changedProjects[workspaceProjectScope(project)] = true
		}
		if shouldGenerate {
			childGenerated = true
		}
		mu.Unlock()
		return nil
	}); err != nil {
		return err
	}

	relationshipsChanged, err := dependencies.LearnWorkspaceRelationships(cont, userContext)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("SyncLearnFailed"), err)
	}
	result := domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{
		Projects:         len(workspaceConfig.Projects),
		ChangedProjects:  len(changedProjects),
		WorkspaceChanged: relationshipsChanged,
		NoFileChanges:    !relationshipsChanged && len(changedProjects) == 0,
	}}
	syncflow.RecordLearnSummary(change, result)

	rootMissing := syncSkillOutputMissing(projectConfig.RootPath, cont.ConfigRepo.GetEffectiveSkillsPath())
	return syncflow.RunAfterLearn(result, childGenerated || rootMissing, func() error {
		return dependencies.GenerateWorkspaceRoot(cont)
	}, change)
}

func syncOpenWorkspaceChild(ctx context.Context, projectRoot string, project config.WorkspaceProjectConfig) (*container.Container, error) {
	projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
	if err != nil {
		return nil, err
	}
	return commandutil.OpenWorkspaceChildContainer(ctx, projectRootPath, project, commandutil.WorkspaceChildErrorKeys{
		NotInitialized: "LearnWorkspaceChildNotInitialized",
		NotGitRepo:     "LearnWorkspaceChildNotGitRepo",
		ModeInvalid:    "LearnWorkspaceChildModeInvalid",
	})
}

func workspaceProjectScope(project config.WorkspaceProjectConfig) string {
	if strings.TrimSpace(project.ID) != "" {
		return strings.TrimSpace(project.ID)
	}
	return strings.TrimSpace(project.Path)
}

func syncGeneratedSkillMissing(cont *container.Container) bool {
	if cont == nil || cont.ConfigRepo == nil {
		return false
	}
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if projectConfig.Mode == domain.ModeWorkspace {
		return syncWorkspaceGeneratedSkillMissing(cont)
	}
	outputPath := strings.TrimSpace(cont.ConfigRepo.GetEffectiveSkillsPath())
	return syncSkillOutputMissing(projectConfig.RootPath, outputPath)
}

func syncWorkspaceGeneratedSkillMissing(cont *container.Container) bool {
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	if syncSkillOutputMissing(projectConfig.RootPath, cont.ConfigRepo.GetEffectiveSkillsPath()) {
		return true
	}
	for _, project := range cont.ConfigRepo.GetWorkspaceConfig().Projects {
		target, err := workspacediscovery.ResolveChildSkillTarget(projectConfig.RootPath, project, cont.ConfigRepo)
		if err != nil {
			continue
		}
		if syncSkillFileMissing(target.OutputPath) {
			return true
		}
	}
	return false
}

func syncSkillOutputMissing(projectRoot, outputPath string) bool {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return false
	}
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(projectRoot, filepath.FromSlash(outputPath))
	}
	return syncSkillFileMissing(outputPath)
}

func syncSkillFileMissing(outputPath string) bool {
	_, err := os.Stat(filepath.Join(outputPath, "SKILL.md"))
	return errors.Is(err, os.ErrNotExist)
}
