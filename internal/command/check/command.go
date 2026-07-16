package check

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/autofix"
	"github.com/spf13/cobra"
)

type checkOptions struct {
	interactive bool
	checkAll    bool
}

// Cmd 返回 check 命令
func Cmd(cont *container.Container) *cobra.Command {
	opts := checkOptions{interactive: true}
	cmd := &cobra.Command{
		Use:     "check",
		Short:   i18n.Get("CheckShort"),
		Long:    i18n.Get("CheckLongDesc"),
		Example: i18n.Get("CheckExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 检查 container 是否初始化
			if cont == nil {
				logger.Error(i18n.Get("CheckNotInitialized"))
				logger.Debug(i18n.Get("CheckRunInitFirst") + "\n")
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return runCheck(cont, opts)
		},
	}

	// 添加 flags
	cmd.Flags().BoolVarP(&opts.interactive, "interactive", "i", opts.interactive, i18n.Get("CheckFlagInteractive"))
	cmd.Flags().BoolVarP(&opts.checkAll, "all", "a", opts.checkAll, i18n.Get("CheckFlagAll"))

	return cmd
}

func runCheck(cont *container.Container, opts checkOptions) error {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return err
	}

	ctx := runtimecontext.WithSeedPath(context.Background(), cont.SeedPath)

	logger.Info(i18n.Get("CheckStarting"))

	var issues []domain.Issue
	var err error
	tracker := progress.New(1)
	retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
	ctx = retryProgress.WithContext(ctx)
	label := i18n.Get("ProgressCheckAnalyzeAI")

	// 检查所有文件还是只检查暂存文件
	err = tracker.RunStep(label, func() error {
		retryProgress.StartStep(label)
		if opts.checkAll {
			logger.Debug(i18n.Get("CheckAllFiles") + "\n")
			issues, err = cont.CheckerSvc.CheckAll(ctx)
		} else {
			logger.Debug(i18n.Get("CheckStagedFiles") + "\n")
			issues, err = cont.CheckerSvc.Check(ctx)
		}
		retryProgress.FinishStep(label, err == nil)
		return err
	})

	if err != nil {
		logger.Error(i18n.GetWithParams("CheckFailed", map[string]interface{}{"Error": err.Error()}))
		return err
	}

	// 显示检查结果
	if len(issues) == 0 {
		logger.Info(i18n.Get("CheckNoIssues"))
		return nil
	}

	logger.Warn(i18n.GetWithParams("CheckFoundIssues", map[string]interface{}{"Count": len(issues)}) + "\n")
	for i, iss := range issues {
		severityLabel := fmt.Sprintf("[%s]", strings.ToUpper(string(iss.Severity)))
		logger.Info(i18n.GetWithParams("LoggerCheckIssueFormat", map[string]interface{}{
			"Index":    i + 1,
			"Severity": severityLabel,
			"File":     iss.File,
			"Line":     iss.Line,
		}))
		logger.Info(i18n.GetWithParams("LoggerCheckIssueMessage", map[string]interface{}{"Message": iss.Message}))

		if iss.Suggestion != "" {
			logger.Debug(i18n.GetWithParams("CheckSuggestion", map[string]interface{}{"Suggestion": iss.Suggestion}) + "\n")
		}
	}

	// 如果是交互模式，处理问题
	if opts.interactive {
		return handleIssuesInteractively(cont, issues, ctx)
	}

	return nil
}

// handleIssuesInteractively 交互式处理问题
func handleIssuesInteractively(cont *container.Container, issues []domain.Issue, ctx context.Context) error {
	// 第一阶段：选择主要操作
	action, err := SelectAction(len(issues))
	if err != nil {
		logger.Error(i18n.GetWithParams("LoggerCheckSelectorError", map[string]interface{}{"Error": err.Error()}))
		return err
	}

	switch action {
	case ActionAutoFix:
		// 第二阶段：选择修复策略
		defaultStrategy := cont.ConfigRepo.GetAutoFixConfig().Strategy
		strategy, err := SelectStrategy(defaultStrategy)
		if err != nil {
			logger.Error(i18n.GetWithParams("LoggerCheckStrategyError", map[string]interface{}{"Error": err.Error()}))
			return err
		}

		logger.Info(i18n.Get("LoggerCheckAutoFixApplying"))
		if err := applyAutoFixWithStrategy(cont, ctx, issues, strategy); err != nil {
			logger.Error(i18n.GetWithParams("LoggerCheckAutoFixFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}

		return nil

	case ActionManualFix:
		logger.Info(i18n.Get("InteractiveManualFixHint"))
		return fmt.Errorf("%s", i18n.Get("ErrManualFixRequired"))

	case ActionLearnAndCommit:
		logger.Info(i18n.Get("InteractiveLearning"))

		// 从暂存区学习
		if err := learnFromStagedFiles(cont, ctx); err != nil {
			logger.Error(i18n.GetWithParams("InteractiveLearningFailed", map[string]interface{}{"Error": err.Error()}))
			return err
		}

		logger.Info(i18n.Get("InteractiveLearningSuccess"))
		return nil

	default:
		return fmt.Errorf("%s", i18n.Get("ErrInvalidAction"))
	}
}

// applyAutoFixWithStrategy 使用指定策略应用自动修复
func applyAutoFixWithStrategy(cont *container.Container, ctx context.Context, issues []domain.Issue, strategy string) error {
	logger.Info(i18n.GetWithParams("LoggerCheckUsingStrategy", map[string]interface{}{"Strategy": strategy}))

	// 1. 生成 AI 修复
	fixes, err := generateFixes(cont, ctx, issues)
	if err != nil {
		return fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGenerateFixes", map[string]interface{}{"Error": err.Error()}))
	}

	if len(fixes) == 0 {
		logger.Warn(i18n.Get("LoggerCheckNoFixGenerated"))
		return nil
	}

	logger.Info(i18n.GetWithParams("LoggerCheckFixCount", map[string]interface{}{"Count": len(fixes)}))

	// 获取备份路径（相对于 .skills-seed 目录）
	backupRelPath := cont.ConfigRepo.GetAutoFixConfig().BackupPath
	if backupRelPath == "" {
		backupRelPath = "backups"
	}
	// 转换为完整路径（.skills-seed 目录下的相对路径）
	backupDir := filepath.Join(cont.SeedPath, backupRelPath)

	// 2. 创建 autofix 服务并应用修复
	autoFixSvc := autofix.NewAutofixService(strategy, backupDir, cont.GitRepo)

	// 应用修复
	result, err := autoFixSvc.FixIssues(ctx, issues, fixes)
	if err != nil {
		return err
	}

	// 显示结果
	logger.Info(result.Message)
	if result.OutputPath != "" {
		logger.Debug(i18n.GetWithParams("LoggerCheckOutputPath", map[string]interface{}{"Path": result.OutputPath}))

		// 根据策略提供不同的提示
		switch strategy {
		case "patch":
			logger.Info(i18n.Get("LoggerCheckReviewPatch"))
			logger.Info(i18n.GetWithParams("LoggerCheckCatPatch", map[string]interface{}{"Path": result.OutputPath}))
			logger.Info(i18n.Get("LoggerCheckApplyPatch"))
			logger.Info(i18n.GetWithParams("LoggerCheckGitApply", map[string]interface{}{"Path": result.OutputPath}))
			logger.Info(i18n.Get("LoggerCheckReversePatch"))
			logger.Info(i18n.GetWithParams("LoggerCheckGitApplyReverse", map[string]interface{}{"Path": result.OutputPath}))
		case "backup":
			logger.Info(i18n.Get("LoggerCheckRestoreNeeded"))
			logger.Info(i18n.GetWithParams("LoggerCheckCpBackup", map[string]interface{}{"Path": result.OutputPath}))
		case "stash":
			logger.Info(i18n.Get("LoggerCheckRestoreFix"))
			logger.Info(i18n.Get("LoggerCheckGitStashPop"))
		case "branch":
			logger.Info(i18n.Get("LoggerCheckViewFix"))
			logger.Info(i18n.Get("LoggerCheckGitDiffMain"))
			logger.Info(i18n.Get("LoggerCheckCreatePR"))
			logger.Info(i18n.GetWithParams("LoggerCheckGhPRCreate", map[string]interface{}{"Branch": result.OutputPath}))
		}
	}

	return nil
}

// generateFixes 生成修复代码
func generateFixes(cont *container.Container, ctx context.Context, issues []domain.Issue) (map[string]string, error) {
	if len(issues) == 0 {
		return make(map[string]string), nil
	}

	// 1. 收集需要修复的文件
	fileMap := make(map[string]domain.FileInfo)
	for _, issue := range issues {
		if _, exists := fileMap[issue.File]; !exists {
			fileMap[issue.File] = domain.FileInfo{
				Path:     issue.File,
				Language: domain.NewFileInfo(issue.File, "").Language,
				Status:   domain.StatusModified,
			}
		}
	}

	// 转换为切片
	files := make([]domain.FileInfo, 0, len(fileMap))
	for _, file := range fileMap {
		files = append(files, file)
	}

	// 2. 获取项目上下文
	projectConfig := cont.ConfigRepo.GetProjectConfig()
	context := agent.ProjectContext{
		Name:     projectConfig.Name,
		Language: projectConfig.Language,
	}

	// 3. 调用 Agent 生成修复（直接传递 domain 类型）
	req := &agent.GenerateFixesRequest{
		Issues:  issues,
		Files:   files,
		Context: context,
	}

	tracker := progress.New(1)
	retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
	ctx = retryProgress.WithContext(ctx)
	label := i18n.Get("ProgressGenerateFixesAI")
	var result *agent.GenerateFixesResult
	err := tracker.RunStep(label, func() error {
		retryProgress.StartStep(label)
		var callErr error
		result, callErr = cont.Agent.GenerateFixes(ctx, req)
		retryProgress.FinishStep(label, callErr == nil)
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGenerateFixes", map[string]interface{}{"Error": err.Error()}))
	}
	if err := agent.RequireResult(result, "GenerateFixes"); err != nil {
		return nil, fmt.Errorf("%s", i18n.GetWithParams("ErrFailedToGenerateFixes", map[string]interface{}{"Error": err.Error()}))
	}

	logger.Info(i18n.GetWithParams("LoggerCheckConfidence", map[string]interface{}{"Confidence": fmt.Sprintf("%.0f", result.Confidence*100)}))
	if strings.TrimSpace(result.Summary) != "" {
		logger.Info(result.Summary)
	}
	for _, warning := range result.Warnings {
		if strings.TrimSpace(warning) != "" {
			logger.Warn(warning)
		}
	}

	return result.Fixes, nil
}

// learnFromStagedFiles 从暂存文件学习
func learnFromStagedFiles(cont *container.Container, ctx context.Context) error {
	stagedFiles, err := cont.GitRepo.GetStagedFiles(ctx)
	if err != nil {
		return err
	}

	if len(stagedFiles) == 0 {
		return nil
	}

	// 构建提交信息
	commitInfo := domain.CommitInfo{
		Hash:    "staged",
		Author:  "current-user",
		Message: "staged changes",
	}

	// 调用学习服务
	return cont.LearnerSvc.LearnFromStaged(ctx, commitInfo)
}
