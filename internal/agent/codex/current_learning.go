package codex

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/agent/structuredtask"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

func (c *CodexAgent) PlanLearningAgenda(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	output, _, err := c.callCurrentLearning(ctx, "LearningPackPlan", "learning-pack-plan", "skills-seed-learning-pack-plan", aicontract.ContractPlanLearningAgenda, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-plan", "")), agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.PlanLearningAgendaPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParsePlanLearningAgendaResult(output)
}

func (c *CodexAgent) NormalizePatterns(ctx context.Context, req *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
	output, _, err := c.callCurrentLearning(ctx, "LearningPatternNormalize", "learning-pattern-normalize", "skills-seed-learning-pattern-normalize", aicontract.ContractNormalizePatterns, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pattern-normalize", "")), agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.NormalizePatternsPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParseNormalizePatternsResult(output)
}

func (c *CodexAgent) ReviewKnowledge(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-knowledge-review", req.RuntimeLabel))
	output, _, err := c.callCurrentLearningWithOptions(ctx, agent.ReviewKnowledgeOperation(req), "learning-knowledge-review", agent.RuntimePromptInputPrefix("skills-seed-learning-knowledge-review", req.RuntimeLabel), aicontract.ContractReviewKnowledge, aicontract.StructuredOutputOptions{CandidateIDs: agent.ReviewKnowledgeCandidateIDs(req.Candidates)}, task, agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.ReviewKnowledgePromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseReviewKnowledgeResult(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}
	if err := agent.ValidateKnowledgeReviewCandidates(result.Decisions, agent.ReviewKnowledgeCandidateIDs(req.Candidates)); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentParseResultFailed"), err)
	}
	return result, agent.RequireResult(result, "ReviewKnowledge")
}

func (c *CodexAgent) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-analyze", req.RuntimeLabel))
	output, _, err := c.callCurrentLearningWithOptions(ctx, agent.AnalyzeCurrentCodebaseBatchOperation(req), "learning-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentCodebaseBatch, aicontract.StructuredOutputOptions{FocusIDs: req.FocusIDs()}, task, agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
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
	output, _, err := c.callCurrentLearningWithOptions(ctx, agent.AnalyzeCurrentDeltaBatchOperation(req), "learning-delta-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-delta-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentDeltaBatch, aicontract.StructuredOutputOptions{FocusIDs: req.FocusIDs()}, task, agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
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
	output, _, err := c.callCurrentLearning(ctx, "LearningProfileRefresh", "learning-profile-refresh", "skills-seed-learning-profile-refresh", aicontract.ContractProjectProfile, agent.NewRuntimeTask(agent.RuntimeSlug("learning-profile-refresh", "")), agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
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

func (c *CodexAgent) ExtractAuthority(ctx context.Context, req *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	opts := aicontract.StructuredOutputOptions{
		AuthoritySectionIDs: agent.AuthoritySectionIDs(req.AuthoritySections),
	}
	output, _, err := c.callCurrentLearningWithOptions(
		ctx,
		"LearningAuthorityExtract",
		"learning-authority-extract",
		"skills-seed-learning-authority-extract",
		aicontract.ContractAuthorityExtraction,
		opts,
		agent.NewRuntimeTask(agent.RuntimeSlug("learning-authority-extract", "")),
		agent.Conversation{},
		func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
			return agent.ExtractAuthorityPromptData(inputs, req)
		},
	)
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseExtractAuthorityResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "ExtractAuthority")
}

func (c *CodexAgent) ReviewAuthority(ctx context.Context, req *agent.ReviewAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	opts := aicontract.StructuredOutputOptions{
		AuthoritySectionIDs: agent.AuthoritySectionIDs(req.AuthoritySections),
	}
	output, _, err := c.callCurrentLearningWithOptions(
		ctx,
		"LearningAuthorityReview",
		"learning-authority-review",
		"skills-seed-learning-authority-review",
		aicontract.ContractAuthorityExtraction,
		opts,
		agent.NewRuntimeTask(agent.RuntimeSlug("learning-authority-review", "")),
		agent.Conversation{},
		func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
			return agent.ReviewAuthorityPromptData(inputs, req)
		},
	)
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseExtractAuthorityResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "ReviewAuthority")
}

func (c *CodexAgent) callCurrentLearning(ctx context.Context, operation, templateName, inputPrefix, outputContract string, task agent.RuntimeTask, conversation agent.Conversation, build func(*agent.PromptInputSession) (map[string]interface{}, error)) (string, agent.Conversation, error) {
	return c.callCurrentLearningWithOptions(ctx, operation, templateName, inputPrefix, outputContract, aicontract.StructuredOutputOptions{}, task, conversation, build)
}

func (c *CodexAgent) callCurrentLearningWithOptions(ctx context.Context, operation, templateName, inputPrefix, outputContract string, opts aicontract.StructuredOutputOptions, task agent.RuntimeTask, conversation agent.Conversation, build func(*agent.PromptInputSession) (map[string]interface{}, error)) (string, agent.Conversation, error) {
	runner := structuredtask.NewWithResult(c.promptLoader, func(ctx context.Context, operation, prompt, contract string, runtime agent.RuntimeTask) (structuredtask.Result, error) {
		output, nextConversation, err := c.callCodexInConversationWithOptions(ctx, operation, prompt, contract, opts, conversation, runtime)
		return structuredtask.Result{Output: output, Conversation: nextConversation}, err
	})
	result, err := runner.RunResult(ctx, structuredtask.Task{
		Operation:      operation,
		Template:       templateName,
		InputPrefix:    inputPrefix,
		OutputContract: outputContract,
		Runtime:        task,
		Build:          build,
	})
	return result.Output, result.Conversation, err
}
