package generate

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/generator"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	"github.com/spf13/cobra"
)

var (
	outputPath string
	merge      bool // 是否在生成前合并模式
	overwrite  bool // 是否覆盖已有子项目 skills
	rootOnly   bool // 是否只生成 workspace 根 skill
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
		defaultOutputPath = config.EffectiveSkillsPath(
			cont.ConfigRepo.GetAgentConfig().Provider,
			cont.ConfigRepo.GetOutputConfig(),
		)
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", defaultOutputPath, i18n.Get("GenerateFlagOutput"))
	cmd.Flags().BoolVarP(&merge, "merge", "m", false, i18n.Get("GenerateFlagMerge"))
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, i18n.Get("GenerateFlagOverwrite"))
	cmd.Flags().BoolVar(&rootOnly, "root-only", false, i18n.Get("GenerateFlagRootOnly"))

	return cmd
}

func runGenerate(cont *container.Container, cmd *cobra.Command) error {
	ctx := context.Background()
	tracker := progress.New(2)

	if overwrite && rootOnly {
		return fmt.Errorf("%s", i18n.Get("GenerateWorkspacePolicyFlagsConflict"))
	}

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
	if !isWorkspaceMode || merge {
		if err := commandutil.RequireAgentAvailable(cont); err != nil {
			return err
		}
	}

	logger.Debug(i18n.GetWithParams("GenerateFoundPatterns", map[string]interface{}{"Count": count}) + "\n")

	effectiveOutputPath := outputPath
	if !cmd.Flags().Changed("output") {
		effectiveOutputPath = config.EffectiveSkillsPath(
			cont.ConfigRepo.GetAgentConfig().Provider,
			cont.ConfigRepo.GetOutputConfig(),
		)
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

	generateOptions := []generator.GenerateOption{}
	if overwrite {
		generateOptions = append(generateOptions, generator.WithWorkspaceChildSkillPolicy(config.WorkspaceChildSkillPolicyOverwrite))
	}
	if rootOnly {
		generateOptions = append(generateOptions, generator.WithWorkspaceChildSkillPolicy(config.WorkspaceChildSkillPolicyRootOnly))
	}

	// 生成 Skills
	if err := tracker.RunStep(i18n.Get("ProgressGenerateWriteSkills"), func() error {
		if len(generateOptions) == 0 {
			return cont.GeneratorSvc.GenerateSkills(ctx, effectiveOutputPath)
		}
		return cont.GeneratorSvc.GenerateSkills(ctx, effectiveOutputPath, generateOptions...)
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
