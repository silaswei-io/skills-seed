package codex

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
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

// CodexAgent 实现模型代理
type CodexAgent struct {
	commandPath      string
	timeout          time.Duration
	promptLoader     promptloader.Renderer
	allowUserPlugins bool
	retryCfg         config.RetryConfig
}

// New 创建代理
func New(commandPath string, timeout time.Duration, loader *promptloader.Loader, allowUserPlugins bool, retryCfg config.RetryConfig) *CodexAgent {
	if commandPath == "" {
		commandPath = "codex"
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &CodexAgent{
		commandPath:      commandPath,
		timeout:          timeout,
		promptLoader:     loader,
		allowUserPlugins: allowUserPlugins,
		retryCfg:         retryCfg,
	}
}

// Name 返回代理名称
func (c *CodexAgent) Name() string { return "codex" }

// IsAvailable 检查代理是否可用
func (c *CodexAgent) IsAvailable() bool {
	_, err := exec.LookPath(c.commandPath)
	return err == nil
}

// UserDefinePattern 根据用户自然语言描述生成模式
func (c *CodexAgent) UserDefinePattern(ctx context.Context, req *agent.UserDefinePatternRequest) (*agent.UserDefinePatternResult, error) {
	data, err := agent.UserDefinePatternPromptData(nil, req)
	if err != nil {
		return nil, err
	}

	prompt, err := c.promptLoader.Render("core-user-pattern", data)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderUserDefinePatternPromptFailed"))
	}

	output, err := c.callCodex(ctx, "UserDefinePattern", prompt, aicontract.ContractUserDefinePattern)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentUserDefinePatternFailed"), err)
	}

	result, err := parser.ParseUserDefinePatternResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}

	logger.Diagnostic(i18n.Get("LoggerDiagnosticAgentParseComplete"),
		"agent", c.Name(),
		"operation", "UserDefinePattern",
		"pattern_id", result.Pattern.ID,
	)

	return result, nil
}

// OptimizeWorkflow 将用户工作流说明整理为标准工作流。
func (c *CodexAgent) OptimizeWorkflow(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
	prompt, err := c.promptLoader.Render("core-workflow-optimize", req)
	if err != nil || prompt == "" {
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderOptimizeWorkflowPromptFailed"))
	}

	output, err := c.callCodex(ctx, "OptimizeWorkflow", prompt, aicontract.ContractOptimizeWorkflow)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentOptimizeWorkflowFailed"), err)
	}

	result, err := parser.ParseOptimizeWorkflowResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}
	return result, nil
}

// AnalyzeWorkspaceProfile 分析工作区结构和跨项目关系
func (c *CodexAgent) AnalyzeWorkspaceProfile(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
	prompt, err := c.promptLoader.Render("core-workspace-profile", agent.WorkspacePromptData(req.WorkspaceName, req.WorkspaceRoot, req.WorkspaceInputPath, "", req.UserContextPath))
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callCodex(ctx, "AnalyzeWorkspaceProfile", prompt, aicontract.ContractWorkspaceProfile)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexProjectAnalysisFailed"), err)
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
func (c *CodexAgent) AnalyzeWorkspaceSpec(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
	data := agent.WorkspacePromptData(req.WorkspaceName, req.WorkspaceRoot, req.WorkspaceInputPath, req.WorkspaceProfilePath, req.UserContextPath)
	prompt, err := c.promptLoader.Render("core-workspace-spec", data)
	if err != nil || prompt == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", i18n.Get("AgentRenderProjectAnalysisPromptFailed"))
	}

	output, err := c.callCodex(ctx, "AnalyzeWorkspaceSpec", prompt, aicontract.ContractWorkspaceSpec)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentCodexProjectAnalysisFailed"), err)
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
