package generate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	outputPath string
	merge      bool // 是否在生成前合并模式
)

// Cmd 返回 generate 命令
func Cmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-skills",
		Short: i18n.Get("GenerateShort"),
		Long:  i18n.Get("GenerateLongDesc"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 检查 container 是否初始化
			if cont == nil {
				logger.Error(i18n.Get("GenerateNotInitialized"))
				logger.Debug(i18n.Get("GenerateRunInitFirst"))
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return runGenerate(cont, cmd)
		},
	}

	// 添加 flags
	defaultOutputPath := ""
	if cont != nil {
		defaultOutputPath = outputPathForCurrentProvider(cont)
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", defaultOutputPath, i18n.Get("GenerateFlagOutput"))
	cmd.Flags().BoolVarP(&merge, "merge", "m", false, i18n.Get("GenerateFlagMerge"))

	return cmd
}

func runGenerate(cont *container.Container, cmd *cobra.Command) error {
	ctx := context.Background()
	tracker := progress.New(2)

	logger.Info(i18n.Get("GenerateStarting"))
	if err := commandutil.LockConfiguredMode(ctx, cont); err != nil {
		return err
	}

	// 获取模式数量
	var count int
	if err := tracker.RunStep(i18n.Get("ProgressGenerateCountPatterns"), func() error {
		var countErr error
		count, countErr = cont.PatternRepo.Count(ctx)
		return countErr
	}); err != nil {
		logger.Error(i18n.GetWithParams("GenerateCountFailed", map[string]interface{}{"Error": err.Error()}))
		return err
	}

	isWorkspaceMode := cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace
	if count == 0 && !isWorkspaceMode {
		logger.Warn(i18n.Get("GenerateNoPatterns"))
		return nil
	}
	if err := requireAgentForGenerate(cont, isWorkspaceMode); err != nil {
		return err
	}

	logger.Debug(i18n.GetWithParams("GenerateFoundPatterns", map[string]interface{}{"Count": count}) + "\n")

	effectiveOutputPath := outputPath
	if !cmd.Flags().Changed("output") {
		effectiveOutputPath = outputPathForCurrentProvider(cont)
	}

	// 如果指定了 --merge 标志，先合并模式
	if merge {
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
	if err := tracker.RunStep(i18n.Get("ProgressGenerateWriteSkills"), func() error {
		if isWorkspaceMode {
			if err := generateWorkspaceChildSkills(ctx, cont); err != nil {
				return err
			}
		}
		return cont.GeneratorSvc.GenerateSkills(ctx, effectiveOutputPath)
	}); err != nil {
		logger.Error(i18n.GetWithParams("GenerateFailed", map[string]interface{}{"Error": err.Error()}))
		return err
	}

	logger.Info(i18n.Get("GenerateSuccessMsg"))
	logger.Info(i18n.GetWithParams("GenerateOutputPath", map[string]interface{}{"Path": effectiveOutputPath}))
	if err := commandutil.MarkSkillsGenerated(ctx, cont); err != nil {
		return err
	}

	return nil
}

func generateWorkspaceChildSkills(ctx context.Context, cont *container.Container) error {
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
	return workspacediscovery.RunProjectTasks(ctx, workspaceConfig.Projects, parallelism, func(ctx context.Context, project config.WorkspaceProjectConfig) error {
		projectRootPath := workspacediscovery.ProjectRoot(projectRoot, project)
		logger.Info(i18n.GetWithParams("GenerateWorkspaceChildGenerating", map[string]interface{}{"ProjectName": project.ID}))
		childCont, err := openWorkspaceChildContainer(ctx, projectRootPath, project)
		if err != nil {
			return err
		}
		defer childCont.Close()

		if err := commandutil.LockConfiguredMode(ctx, childCont); err != nil {
			return err
		}
		if generationModeRequiresAgent(childCont) {
			if err := commandutil.RequireAgentAvailable(childCont); err != nil {
				return err
			}
		}
		childOutputPath := outputPathForCurrentProvider(childCont)
		if err := childCont.GeneratorSvc.GenerateSkills(ctx, childOutputPath); err != nil {
			var manualErr *generator.ManualSkillExistsError
			if errors.As(err, &manualErr) {
				logger.Warn(i18n.GetWithParams("GenerateWorkspaceChildManualSkillSkipped", map[string]interface{}{
					"ProjectName": project.ID,
					"Path":        manualErr.Path,
				}))
				return nil
			}
			return err
		}
		logger.Info(i18n.GetWithParams("GenerateWorkspaceChildGenerated", map[string]interface{}{"ProjectName": project.ID}))
		return nil
	})
}

func requireAgentForGenerate(cont *container.Container, isWorkspaceMode bool) error {
	if merge {
		return commandutil.RequireAgentAvailable(cont)
	}
	if isWorkspaceMode {
		return nil
	}
	if generationModeRequiresAgent(cont) {
		return commandutil.RequireAgentAvailable(cont)
	}
	return nil
}

func generationModeRequiresAgent(cont *container.Container) bool {
	if cont == nil || cont.ConfigRepo == nil {
		return false
	}
	return config.NormalizeGenerationMode(cont.ConfigRepo.GetGenerationConfig().Mode) == config.GenerationModeAI
}

func openWorkspaceChildContainer(ctx context.Context, projectRootPath string, project config.WorkspaceProjectConfig) (*container.Container, error) {
	childSeedPath := filepath.Join(projectRootPath, ".skills-seed")
	childConfigPath := filepath.Join(childSeedPath, "config.yaml")
	if _, err := os.Stat(childConfigPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("GenerateWorkspaceChildNotInitialized", map[string]interface{}{
				"ProjectName": project.ID,
				"Path":        projectRootPath,
			}))
		}
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(projectRootPath, ".git")); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s", i18n.GetWithParams("GenerateWorkspaceChildNotGitRepo", map[string]interface{}{
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
		return nil, fmt.Errorf("%s", i18n.GetWithParams("GenerateWorkspaceChildModeInvalid", map[string]interface{}{
			"ProjectName": project.ID,
			"Mode":        childCont.ConfigRepo.GetProjectConfig().Mode,
		}))
	}
	return childCont, nil
}

func outputPathForCurrentProvider(cont *container.Container) string {
	if cont == nil || cont.ConfigRepo == nil {
		return ""
	}
	return config.EffectiveSkillsPath(cont.ConfigRepo.GetAgentConfig().Provider, cont.ConfigRepo.GetOutputConfig())
}
