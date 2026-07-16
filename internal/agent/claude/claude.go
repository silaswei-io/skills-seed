package claude

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts/loader"
)

// ClaudeAgent 实现模型代理
type ClaudeAgent struct {
	commandPath      string
	timeout          time.Duration
	promptLoader     promptRenderer
	allowUserPlugins bool
	retryCfg         config.RetryConfig
}

// promptRenderer 是 Agent 依赖的最小提示词渲染能力，便于测试渲染错误链路
type promptRenderer interface {
	Render(name string, data interface{}) (string, error)
	RenderForRuntimeTask(name string, data interface{}, task promptloader.RuntimeTask) (string, error)
}

// New 创建代理
func New(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) *ClaudeAgent {
	if commandPath == "" {
		commandPath = "claude"
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &ClaudeAgent{
		commandPath:      commandPath,
		timeout:          timeout,
		promptLoader:     loader,
		allowUserPlugins: allowUserPlugins,
		retryCfg:         retryCfg,
	}
}

// Name 返回代理名称
func (c *ClaudeAgent) Name() string {
	return "claude"
}

// IsAvailable 检查代理是否可用
func (c *ClaudeAgent) IsAvailable() bool {
	_, err := exec.LookPath(c.commandPath)
	return err == nil
}

// AnalyzeCode 分析代码
func (c *ClaudeAgent) AnalyzeCode(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-check")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 构建提示词（从模板加载）
	data, err := agent.CheckPromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.Render("learn-analyze", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderAnalyzePromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, err := c.callClaude(ctx, "AnalyzeCode", prompt, aicontract.ContractAnalyzeCode)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeAnalyzeFailed"), err)
	}

	// 3. 解析结构化结果
	result, err := parser.ParseAnalyzeResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeCode",
		"issues_count", len(result.Issues),
		"suggestions_count", len(result.Suggestions),
		"confidence", result.Confidence,
	)

	result.AnalyzedAt = time.Now()
	return result, nil
}

// LearnFromCommit 从提交中学习
func (c *ClaudeAgent) LearnFromCommit(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-learn")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 包装为批量格式，复用批量学习模板
	data, err := agent.BatchLearnPromptData(
		session,
		[]domain.CommitInfo{req.Commit},
		[]agent.CommitFileChange{{Commit: req.Commit, Files: req.ChangedFiles}},
		req.KnownPatternsJSON,
		req.KnownPatternsPath,
		req.KnownPatternsCount,
	)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.Render("learn-batch", data)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentPromptRenderFailed"),
			"method", "LearnFromCommit",
			"template", "learn-batch",
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderBatchLearnPromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, err := c.callClaude(ctx, "LearnFromCommit", prompt, aicontract.ContractLearnPatterns)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "LearnFromCommit",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeLearnFailed"), err)
	}

	// 3. 解析结构化结果
	result, err := parser.ParseLearnResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "LearnFromCommit",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "LearnFromCommit",
		"patterns_count", len(result.Patterns),
	)

	result.LearnedAt = time.Now()
	return result, nil
}

// BatchLearnFromCommits 批量从多个提交中学习
func (c *ClaudeAgent) BatchLearnFromCommits(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-learn-batch")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 准备模板数据
	data, err := agent.BatchLearnPromptData(session, req.Commits, req.CommitFiles, req.KnownPatternsJSON, req.KnownPatternsPath, req.KnownPatternsCount)
	if err != nil {
		return nil, err
	}

	// 2. 渲染提示词
	prompt, err := c.promptLoader.Render("learn-batch", data)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentPromptRenderFailed"),
			"method", "BatchLearnFromCommits",
			"template", "learn-batch",
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderBatchLearnPromptFailed"))
	}

	// 3. 调用外部命令行程序
	output, err := c.callClaude(ctx, "BatchLearnFromCommits", prompt, aicontract.ContractLearnPatterns)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "BatchLearnFromCommits",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeBatchLearnFailed"), err)
	}

	// 4. 解析结构化结果
	result, err := parser.ParseBatchLearnResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "BatchLearnFromCommits",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "BatchLearnFromCommits",
		"patterns_count", len(result.Patterns),
	)

	result.LearnedAt = time.Now()
	return result, nil
}

// GenerateFixes 为给定的问题生成修复代码
func (c *ClaudeAgent) GenerateFixes(ctx context.Context, req *agent.GenerateFixesRequest) (*agent.GenerateFixesResult, error) {
	// 1. 构建提示词（从模板加载）
	prompt, err := c.promptLoader.Render("fix-generate", req)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentPromptRenderFailed"),
			"method", "GenerateFixes",
			"template", "fix-generate",
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderGenerateFixesPromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, err := c.callClaude(ctx, "GenerateFixes", prompt, aicontract.ContractGenerateFixes)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "GenerateFixes",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeGenerateFixesFailed"), err)
	}

	// 3. 解析结构化结果
	result, err := parser.ParseGenerateFixesResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "GenerateFixes",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "GenerateFixes",
		"fixes_count", len(result.Fixes),
		"confidence", result.Confidence,
	)

	result.GeneratedAt = time.Now()
	return result, nil
}

// SelectFiles 基于候选文件树选择当前代码学习范围。
func (c *ClaudeAgent) SelectFiles(ctx context.Context, req *agent.SelectFilesRequest) (*agent.SelectFilesResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("file-select", ""))
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-file-select")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	data, err := agent.SelectFilesPromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.RenderForRuntimeTask("file-select", data, promptRuntimeTask(task))
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderAnalyzePromptFailed"))
	}

	output, err := c.callClaude(ctx, "SelectFiles", prompt, aicontract.ContractSelectFiles, task)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeAnalyzeFailed"), err)
	}
	return parser.ParseSelectFilesResult(output)
}

// CuratePatterns 策展候选模式并输出规范模式。
func (c *ClaudeAgent) CuratePatterns(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
	data := map[string]interface{}{
		"Operation":           req.Operation,
		"CandidatePatterns":   req.CandidatePatterns,
		"ExistingPatterns":    req.ExistingPatterns,
		"AllExisting":         req.AllExisting,
		"ExistingByCandidate": req.ExistingByCandidate,
		"AllowedCategories":   domain.AllowedPatternCategoriesText(),
	}

	prompt, err := c.promptLoader.Render("pattern-curate", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderCuratePatternsPromptFailed"))
	}

	output, err := c.callClaude(ctx, "CuratePatterns", prompt, aicontract.ContractCuratePatterns)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentCuratePatternsCallFailed"),
			"error", err,
			"operation", req.Operation,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeCuratePatternsFailed"), err)
	}

	result, err := parser.ParseCuratePatternsResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "CuratePatterns",
			"error", err,
			"operation", req.Operation,
		)
		return nil, err
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "CuratePatterns",
		"written_count", len(result.Patterns),
		"dropped_count", len(result.Dropped),
		"total_candidates", result.Summary.TotalCandidates,
	)

	return result, nil
}

// UserDefinePattern 根据用户自然语言描述生成模式
func (c *ClaudeAgent) UserDefinePattern(ctx context.Context, req *agent.UserDefinePatternRequest) (*agent.UserDefinePatternResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-user-pattern")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	data, err := agent.UserDefinePatternPromptData(session, req)
	if err != nil {
		return nil, err
	}

	prompt, err := c.promptLoader.Render("user-define-pattern", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderUserDefinePatternPromptFailed"))
	}

	output, err := c.callClaude(ctx, "UserDefinePattern", prompt, aicontract.ContractUserDefinePattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentUserDefinePatternFailed"), err)
	}

	result, err := parser.ParseUserDefinePatternResult(output)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", "UserDefinePattern",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "UserDefinePattern",
		"pattern_id", result.Pattern.ID,
		"pattern_name", result.Pattern.Name,
		"category", result.Pattern.Category,
	)

	return result, nil
}

// OptimizeWorkflow 将用户工作流说明整理为标准工作流。
func (c *ClaudeAgent) OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
	prompt, err := c.promptLoader.Render("workflow-optimize", req)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderOptimizeWorkflowPromptFailed"))
	}

	output, err := c.callClaude(ctx, "OptimizeWorkflow", prompt, aicontract.ContractOptimizeWorkflow)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentOptimizeWorkflowFailed"), err)
	}

	result, err := parser.ParseOptimizeWorkflowResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}
	return result, nil
}

// AnalyzeProject 分析项目结构
func (c *ClaudeAgent) AnalyzeProject(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-project-profile")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 准备模板数据
	data, err := agent.AnalyzeProjectPromptData(session, req)
	if err != nil {
		return nil, err
	}

	// 2. 渲染提示词
	prompt, err := c.promptLoader.Render("project-profile", data)
	if err != nil || prompt == "" {
		logger.Error(i18n.Get("LoggerAgentProjectPromptRenderFailed"),
			"project", req.ProjectName,
			"error", err,
			"prompt_empty", prompt == "",
		)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.Get("AgentRenderProjectAnalysisPromptFailed"), err)
		}
		return nil, fmt.Errorf("%s: prompt is empty", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	// 3. 调用外部命令行程序
	output, err := c.callClaude(ctx, "AnalyzeProject", prompt, aicontract.ContractProjectProfile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	// 4. 解析结果
	result, err := parser.ParseAnalyzeProjectResult(output)
	if err != nil {
		logger.Error(i18n.Get("AgentParseProjectAnalysisFailed"),
			"error", err,
			"project", req.ProjectName)
		logger.Error(i18n.Get("AgentRawOutputLength"), "output_length", len(output))
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeProject",
		"frameworks_count", len(result.Frameworks),
		"dependencies_count", len(result.Dependencies),
		"key_modules_count", len(result.KeyModules),
	)

	return result, nil
}

// AnalyzeCurrentCodebase 分析当前代码库，提取初始模式
func (c *ClaudeAgent) AnalyzeCurrentCodebase(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
	operation := agent.AnalyzeCurrentCodebaseOperation(req)
	task := agent.NewRuntimeTask(agent.RuntimeSlug("pattern-learn-current", req.RuntimeLabel))
	session, err := agent.NewPromptInputSessionForContext(ctx, agent.RuntimePromptInputPrefix("skills-seed-pattern-learn-current", req.RuntimeLabel))
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	// 1. 构建提示词（从模板加载）
	data, err := agent.AnalyzeCurrentCodebasePromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.RenderForRuntimeTask("pattern-learn-current", data, promptRuntimeTask(task))
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderInitSkillsPromptFailed"))
	}

	// 2. 调用外部命令行程序
	output, archive, err := c.callClaudeWithArchive(ctx, operation, prompt, aicontract.ContractAnalyzeCurrentCodebase, task)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", operation,
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	// 3. 解析结果
	result, err := parseClaudeResult(c.Name(), operation, output, archive, parser.ParseAnalyzeCurrentCodebaseResult)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", operation,
			"error", err,
			"output_length", len(output),
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", operation,
		"patterns_count", len(result.Patterns),
		"profile_refresh_recommended", result.ProfileRefreshRecommended.Needed,
	)

	return result, nil
}

// AnalyzeCurrentCodebaseBatch 批量分析当前代码库，按分析单元返回候选模式。
func (c *ClaudeAgent) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	operation := agent.AnalyzeCurrentCodebaseBatchOperation(req)
	task := agent.NewRuntimeTask(agent.RuntimeSlug("pattern-learn-current-batch", req.RuntimeLabel))
	session, err := agent.NewPromptInputSessionForContext(ctx, agent.RuntimePromptInputPrefix("skills-seed-pattern-learn-current-batch", req.RuntimeLabel))
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	data, err := agent.AnalyzeCurrentCodebaseBatchPromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.RenderForRuntimeTask("pattern-learn-current-batch", data, promptRuntimeTask(task))
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderInitSkillsPromptFailed"))
	}

	output, archive, err := c.callClaudeWithArchive(ctx, operation, prompt, aicontract.ContractAnalyzeCurrentCodebaseBatch, task)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", operation,
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	result, err := parseClaudeResult(c.Name(), operation, output, archive, parser.ParseAnalyzeCurrentCodebaseBatchResult)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
			"method", operation,
			"error", err,
			"output_length", len(output),
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", operation,
		"units_count", len(result.Units),
	)
	return result, nil
}

// PlanAnalysisUnits 将当前待学习文件拆成可续跑的业务分析单元。
func (c *ClaudeAgent) PlanAnalysisUnits(ctx context.Context, req *agent.PlanAnalysisUnitsRequest) (*agent.PlanAnalysisUnitsResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("analysis-plan", ""))
	session, err := agent.NewPromptInputSessionForContext(ctx, "skills-seed-analysis-plan")
	if err != nil {
		return nil, err
	}
	defer session.Cleanup()

	data, err := agent.PlanAnalysisUnitsPromptData(session, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.RenderForRuntimeTask("analysis-plan", data, promptRuntimeTask(task))
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderAnalysisPlanPromptFailed"))
	}

	output, err := c.callClaude(ctx, "PlanAnalysisUnits", prompt, aicontract.ContractPlanAnalysisUnits, task)
	if err != nil {
		logger.Error(i18n.Get("LoggerAgentClaudeCallFailedNonFallback"),
			"method", "PlanAnalysisUnits",
			"error", err,
		)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	result, err := parser.ParsePlanAnalysisUnitsResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "PlanAnalysisUnits",
		"units_count", len(result.Units),
	)
	return result, nil
}

// AnalyzeWorkspaceProfile 分析工作区结构和跨项目关系
func (c *ClaudeAgent) AnalyzeWorkspaceProfile(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
	prompt, err := c.promptLoader.Render("skill-workspace-profile", agent.WorkspacePromptData(req.WorkspaceName, req.WorkspaceRoot, req.WorkspaceInputPath, "", req.UserContextPath))
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callClaude(ctx, "AnalyzeWorkspaceProfile", prompt, aicontract.ContractWorkspaceProfile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	result, err := parser.ParseWorkspaceProfile(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeWorkspaceProfile",
		"projects_count", len(result.Projects),
		"impact_routes_count", len(result.ImpactRoutes),
	)
	return result, nil
}

// AnalyzeWorkspaceSpec 生成工作区级开发规范
func (c *ClaudeAgent) AnalyzeWorkspaceSpec(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
	data := agent.WorkspacePromptData(req.WorkspaceName, req.WorkspaceRoot, req.WorkspaceInputPath, req.WorkspaceProfilePath, req.UserContextPath)
	prompt, err := c.promptLoader.Render("skill-workspace-spec", data)
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callClaude(ctx, "AnalyzeWorkspaceSpec", prompt, aicontract.ContractWorkspaceSpec)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentClaudeProjectAnalysisFailed"), err)
	}

	result, err := parser.ParseWorkspaceSpec(output)
	if err != nil {
		return nil, err
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "AnalyzeWorkspaceSpec",
		"routing_count", len(result.Routing),
		"rules_count", len(result.Rules),
	)
	return result, nil
}
