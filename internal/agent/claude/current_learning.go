package claude

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/agent/structuredtask"
)

func (c *ClaudeAgent) PlanLearningAgenda(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningPackPlan", "learning-pack-plan", "skills-seed-learning-pack-plan", aicontract.ContractPlanLearningAgenda, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-plan", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.PlanLearningAgendaPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParsePlanLearningAgendaResult(output)
}

func (c *ClaudeAgent) NormalizePatterns(ctx context.Context, req *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningPatternNormalize", "learning-pattern-normalize", "skills-seed-learning-pattern-normalize", aicontract.ContractNormalizePatterns, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pattern-normalize", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.NormalizePatternsPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParseNormalizePatternsResult(output)
}

func (c *ClaudeAgent) ReviewKnowledge(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningKnowledgeReview", "learning-knowledge-review", "skills-seed-learning-knowledge-review", aicontract.ContractReviewKnowledge, agent.NewRuntimeTask(agent.RuntimeSlug("learning-knowledge-review", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.ReviewKnowledgePromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseReviewKnowledgeResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "ReviewKnowledge")
}

func (c *ClaudeAgent) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
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

func (c *ClaudeAgent) AnalyzeCurrentDeltaBatch(ctx context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
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

func (c *ClaudeAgent) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
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

func (c *ClaudeAgent) ExtractAuthority(ctx context.Context, req *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	output, err := c.callCurrentLearning(ctx, "LearningAuthorityExtract", "learning-authority-extract", "skills-seed-learning-authority-extract", aicontract.ContractAuthorityExtraction, agent.NewRuntimeTask(agent.RuntimeSlug("learning-authority-extract", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.ExtractAuthorityPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseExtractAuthorityResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "ExtractAuthority")
}

func (c *ClaudeAgent) callCurrentLearning(ctx context.Context, operation, templateName, inputPrefix, outputContract string, task agent.RuntimeTask, build func(*agent.PromptInputSession) (map[string]interface{}, error)) (string, error) {
	runner := structuredtask.New(c.promptLoader, func(ctx context.Context, operation, prompt, contract string, runtime agent.RuntimeTask) (string, error) {
		return c.callClaude(ctx, operation, prompt, contract, runtime)
	})
	return runner.Run(ctx, structuredtask.Task{
		Operation:      operation,
		Template:       templateName,
		InputPrefix:    inputPrefix,
		OutputContract: outputContract,
		Runtime:        task,
		Build:          build,
	})
}
