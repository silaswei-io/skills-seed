package codex

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts"
)

func (c *CodexAgent) SelectLearningCandidates(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningCandidateSelect", "learning-candidate-select", "skills-seed-learning-candidate-select", aicontract.ContractSelectLearningCandidates, agent.NewRuntimeTask(agent.RuntimeSlug("learning-candidate-select", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.SelectLearningCandidatesPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParseSelectLearningCandidatesResult(output)
}

func (c *CodexAgent) PlanLearningAgenda(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningPackPlan", "learning-pack-plan", "skills-seed-learning-pack-plan", aicontract.ContractPlanLearningAgenda, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-plan", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.PlanLearningAgendaPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParsePlanLearningAgendaResult(output)
}

func (c *CodexAgent) NormalizePatterns(ctx context.Context, req *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningPatternNormalize", "learning-pattern-normalize", "skills-seed-learning-pattern-normalize", aicontract.ContractNormalizePatterns, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pattern-normalize", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.NormalizePatternsPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParseNormalizePatternsResult(output)
}

func (c *CodexAgent) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-analyze", req.RuntimeLabel))
	output, err := c.callCurrentLearning(ctx, agent.AnalyzeCurrentCodebaseBatchOperation(req), "learning-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentCodebaseBatch, task, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.AnalyzeCurrentCodebaseBatchPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseAnalyzeCurrentCodebaseBatchResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "AnalyzeCurrentCodebaseBatch")
}

func (c *CodexAgent) AnalyzeCurrentDeltaBatch(ctx context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-delta-pack-analyze", req.RuntimeLabel))
	output, err := c.callCurrentLearning(ctx, agent.AnalyzeCurrentDeltaBatchOperation(req), "learning-delta-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-delta-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentDeltaBatch, task, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.AnalyzeCurrentDeltaBatchPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseAnalyzeCurrentDeltaBatchResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "AnalyzeCurrentDeltaBatch")
}

func (c *CodexAgent) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningProfileRefresh", "learning-profile-refresh", "skills-seed-learning-profile-refresh", aicontract.ContractProjectProfile, agent.NewRuntimeTask(agent.RuntimeSlug("learning-profile-refresh", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.AnalyzeProjectPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseAnalyzeProjectResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "AnalyzeProject")
}

func (c *CodexAgent) callCurrentLearning(ctx context.Context, operation, templateName, inputPrefix, outputContract string, task agent.RuntimeTask, build func(*agent.PromptInputSession) (map[string]interface{}, error)) (string, error) {
	inputs, err := agent.NewPromptInputSessionForContext(ctx, inputPrefix)
	if err != nil {
		return "", err
	}
	defer inputs.Cleanup()

	data, err := build(inputs)
	if err != nil {
		return "", err
	}
	prompt, err := c.promptLoader.RenderForRuntimeTask(templateName, data, promptRuntimeTask(task))
	if err != nil || prompt == "" {
		return "", fmt.Errorf("%s", i18n.Get("AgentRenderInitSkillsPromptFailed"))
	}
	return c.callCodex(ctx, operation, prompt, outputContract, task)
}

func promptRuntimeTask(task agent.RuntimeTask) promptloader.RuntimeTask {
	return promptloader.RuntimeTask{ID: task.ID, Slug: task.Slug}
}
