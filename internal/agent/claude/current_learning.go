package claude

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/learning"
	"github.com/silaswei-io/skills-seed/internal/prompts"
)

// PromptRenderer 返回学习任务使用的提示词渲染器。
func (c *ClaudeAgent) PromptRenderer() prompts.Renderer {
	return c.promptLoader
}

// InvokeStructured 执行已渲染的结构化 Claude 调用。
func (c *ClaudeAgent) InvokeStructured(ctx context.Context, in agent.StructuredInvoke) (agent.StructuredResult, error) {
	output, conversation, err := c.callClaudeInConversationWithOptions(ctx, in.Operation, in.Prompt, in.OutputContract, in.Options, in.Conversation, in.Runtime)
	return agent.StructuredResult{Output: output, Conversation: conversation}, err
}

func (c *ClaudeAgent) PlanLearningAgenda(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	return learning.PlanLearningAgenda(ctx, c, req)
}

func (c *ClaudeAgent) NormalizePatterns(ctx context.Context, req *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
	return learning.NormalizePatterns(ctx, c, req)
}

func (c *ClaudeAgent) ReviewKnowledge(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
	return learning.ReviewKnowledge(ctx, c, req)
}

func (c *ClaudeAgent) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	return learning.AnalyzeCurrentCodebaseBatch(ctx, c, req)
}

func (c *ClaudeAgent) AnalyzeCurrentDeltaBatch(ctx context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	return learning.AnalyzeCurrentDeltaBatch(ctx, c, req)
}

func (c *ClaudeAgent) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	return learning.RefreshProjectProfile(ctx, c, req)
}

func (c *ClaudeAgent) ExtractAuthority(ctx context.Context, req *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	return learning.ExtractAuthority(ctx, c, req)
}

func (c *ClaudeAgent) ReviewAuthority(ctx context.Context, req *agent.ReviewAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	return learning.ReviewAuthority(ctx, c, req)
}
