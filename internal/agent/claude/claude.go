package claude

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

// ClaudeAgent 实现模型代理
type ClaudeAgent struct {
	commandPath      string
	timeout          time.Duration
	promptLoader     promptloader.Renderer
	allowUserPlugins bool
	retryCfg         config.RetryConfig
	runtime          config.AgentRuntimeOptions
}

// New 创建代理
func New(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig, runtime config.AgentRuntimeOptions) *ClaudeAgent {
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
		runtime:          runtime,
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

	prompt, err := c.promptLoader.Render("core-user-pattern", data)
	if err != nil || prompt == "" {
		return nil, errors.New(i18n.Get("AgentRenderUserDefinePatternPromptFailed"))
	}

	output, err := c.callClaude(ctx, "UserDefinePattern", prompt, aicontract.ContractUserDefinePattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentUserDefinePatternFailed"), err)
	}

	result, err := parser.ParseUserDefinePatternResult(output)
	if err != nil {
		logger.DiagnosticError(i18n.Get("LoggerAgentParseResultFailedNonFallback"),
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

// OptimizeWorkflow 整理用户提供的工作流内容。
func (c *ClaudeAgent) OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeContentResult, error) {
	return c.optimizeContent(ctx, "OptimizeWorkflow", "core-workflow-optimize", req, aicontract.ContractOptimizeWorkflow,
		"AgentRenderOptimizeWorkflowPromptFailed", "AgentOptimizeWorkflowFailed")
}

// OptimizeRule 在当前项目边界内润色用户权威规则。
func (c *ClaudeAgent) OptimizeRule(ctx context.Context, req *agent.OptimizeRuleRequest) (*agent.OptimizeContentResult, error) {
	return c.optimizeContent(ctx, "OptimizeRule", "core-rule-optimize", req, aicontract.ContractOptimizeRule,
		"AgentRenderOptimizeRulePromptFailed", "AgentOptimizeRuleFailed")
}

func (c *ClaudeAgent) optimizeContent(ctx context.Context, operation, promptName string, data interface{}, contract, renderError, callError string) (*agent.OptimizeContentResult, error) {
	prompt, err := c.promptLoader.Render(promptName, data)
	if err != nil || prompt == "" {
		return nil, errors.New(i18n.Get(renderError))
	}
	output, err := c.callClaude(ctx, operation, prompt, contract)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get(callError), err)
	}
	result, err := parser.ParseOptimizeContentResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}
	return result, nil
}

// AnalyzeWorkspaceProfile 分析工作区结构和跨项目关系
func (c *ClaudeAgent) AnalyzeWorkspaceProfile(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
	data := agent.WorkspacePromptData(agent.WorkspacePromptDataRequest{
		WorkspaceName:      req.WorkspaceName,
		WorkspaceRoot:      req.WorkspaceRoot,
		WorkspaceInputPath: req.WorkspaceInputPath,
		UserContextPath:    req.UserContextPath,
		ProjectIDs:         req.ProjectIDs,
	})
	prompt, err := c.promptLoader.Render("core-workspace-profile", data)
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, errors.New(i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callClaudeWithOptions(ctx, "AnalyzeWorkspaceProfile", prompt, aicontract.ContractWorkspaceProfile, aicontract.StructuredOutputOptions{ProjectIDs: req.ProjectIDs})
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
	data := agent.WorkspacePromptData(agent.WorkspacePromptDataRequest{
		WorkspaceName:        req.WorkspaceName,
		WorkspaceRoot:        req.WorkspaceRoot,
		WorkspaceInputPath:   req.WorkspaceInputPath,
		WorkspaceProfilePath: req.WorkspaceProfilePath,
		UserContextPath:      req.UserContextPath,
		ProjectIDs:           req.ProjectIDs,
	})
	prompt, err := c.promptLoader.Render("core-workspace-spec", data)
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, errors.New(i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callClaudeWithOptions(ctx, "AnalyzeWorkspaceSpec", prompt, aicontract.ContractWorkspaceSpec, aicontract.StructuredOutputOptions{ProjectIDs: req.ProjectIDs})
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
