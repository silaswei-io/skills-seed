package learning

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// PlanLearningAgenda 规划当前代码学习的证据焦点议程。
func PlanLearningAgenda(ctx context.Context, rt agent.LearningRuntime, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	output, _, err := call(ctx, rt, "LearningPackPlan", "learning-pack-plan", "skills-seed-learning-pack-plan", aicontract.ContractPlanLearningAgenda, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-plan", "")), agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.PlanLearningAgendaPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParsePlanLearningAgendaResult(output)
}

// NormalizePatterns 归并当前学习候选模式。
func NormalizePatterns(ctx context.Context, rt agent.LearningRuntime, req *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
	output, _, err := call(ctx, rt, "LearningPatternNormalize", "learning-pattern-normalize", "skills-seed-learning-pattern-normalize", aicontract.ContractNormalizePatterns, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pattern-normalize", "")), agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.NormalizePatternsPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParseNormalizePatternsResult(output)
}

// ReviewKnowledge 独立复核源码学习候选。
func ReviewKnowledge(ctx context.Context, rt agent.LearningRuntime, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-knowledge-review", req.RuntimeLabel))
	output, _, err := callWithOptions(ctx, rt, agent.ReviewKnowledgeOperation(req), "learning-knowledge-review", agent.RuntimePromptInputPrefix("skills-seed-learning-knowledge-review", req.RuntimeLabel), aicontract.ContractReviewKnowledge, aicontract.StructuredOutputOptions{CandidateIDs: agent.ReviewKnowledgeCandidateIDs(req.Candidates)}, task, req.Conversation, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
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

// AnalyzeCurrentFocusBatch 按 Mode 分发到 full 或 delta 分析实现。
func AnalyzeCurrentFocusBatch(ctx context.Context, rt agent.LearningRuntime, req *agent.AnalyzeCurrentFocusBatchRequest) (*agent.AnalyzeCurrentFocusBatchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("AnalyzeCurrentFocusBatch request is nil")
	}
	mode := req.Mode
	if mode == "" {
		mode = agent.FocusAnalysisModeFull
	}
	switch mode {
	case agent.FocusAnalysisModeDelta:
		deltaReq := &agent.AnalyzeCurrentDeltaBatchRequest{
			ProjectName:           req.ProjectName,
			RootPath:              req.RootPath,
			Language:              req.Language,
			RuntimeLabel:          req.RuntimeLabel,
			SharedContextPath:     req.SharedContextPath,
			Structure:             req.Structure,
			StructurePath:         req.StructurePath,
			StructuralContext:     req.StructuralContext,
			StructuralContextPath: req.StructuralContextPath,
			UserContext:           req.UserContext,
			UserContextPath:       req.UserContextPath,
			MaintainedGuidance:    req.MaintainedGuidance,
			LearningMode:          req.LearningMode,
			ChangeProfile:         req.ChangeProfile,
			Focuses:               make([]agent.AnalyzeCurrentDeltaFocus, 0, len(req.Focuses)),
		}
		for _, focus := range req.Focuses {
			deltaReq.Focuses = append(deltaReq.Focuses, agent.AnalyzeCurrentDeltaFocus{
				EvidenceFocus:   focus.EvidenceFocus,
				FocusPaths:      append([]string(nil), focus.FocusPaths...),
				ContextFiles:    append([]agent.SampleFile(nil), focus.SampleFiles...),
				DiffFiles:       append([]agent.DiffFileRef(nil), focus.DiffFiles...),
				RelatedPatterns: append([]domain.Pattern(nil), focus.RelatedPatterns...),
			})
		}
		result, err := AnalyzeCurrentDeltaBatch(ctx, rt, deltaReq)
		if err != nil {
			return nil, err
		}
		out := &agent.AnalyzeCurrentFocusBatchResult{Mode: agent.FocusAnalysisModeDelta}
		if result != nil {
			out.Changes = result.Changes
			out.ProfileRefreshRecommended = result.ProfileRefreshRecommended
		}
		return out, nil
	default:
		fullReq := &agent.AnalyzeCurrentCodebaseBatchRequest{
			ProjectName:           req.ProjectName,
			RootPath:              req.RootPath,
			Language:              req.Language,
			RuntimeLabel:          req.RuntimeLabel,
			SharedContextPath:     req.SharedContextPath,
			Structure:             req.Structure,
			StructurePath:         req.StructurePath,
			StructuralContext:     req.StructuralContext,
			StructuralContextPath: req.StructuralContextPath,
			MainFiles:             append([]string(nil), req.MainFiles...),
			UserContext:           req.UserContext,
			UserContextPath:       req.UserContextPath,
			MaintainedGuidance:    req.MaintainedGuidance,
			LearningMode:          req.LearningMode,
			ChangeProfile:         req.ChangeProfile,
			Focuses:               make([]agent.AnalyzeCurrentEvidenceFocus, 0, len(req.Focuses)),
		}
		for _, focus := range req.Focuses {
			fullReq.Focuses = append(fullReq.Focuses, agent.AnalyzeCurrentEvidenceFocus{
				EvidenceFocus: focus.EvidenceFocus,
				FocusPaths:    append([]string(nil), focus.FocusPaths...),
				SampleFiles:   append([]agent.SampleFile(nil), focus.SampleFiles...),
				DiffFiles:     append([]agent.DiffFileRef(nil), focus.DiffFiles...),
			})
		}
		result, err := AnalyzeCurrentCodebaseBatch(ctx, rt, fullReq)
		if err != nil {
			return nil, err
		}
		out := &agent.AnalyzeCurrentFocusBatchResult{Mode: agent.FocusAnalysisModeFull}
		if result != nil {
			out.Focuses = result.Focuses
			out.Conversation = result.Conversation
			for _, focus := range result.Focuses {
				if focus.ProfileRefreshRecommended.Needed {
					out.ProfileRefreshRecommended = focus.ProfileRefreshRecommended
					break
				}
			}
		}
		return out, nil
	}
}

// AnalyzeCurrentCodebaseBatch 批量分析当前代码学习焦点。
func AnalyzeCurrentCodebaseBatch(ctx context.Context, rt agent.LearningRuntime, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-analyze", req.RuntimeLabel))
	output, conversation, err := callWithOptions(ctx, rt, agent.AnalyzeCurrentCodebaseBatchOperation(req), "learning-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentCodebaseBatch, aicontract.StructuredOutputOptions{FocusIDs: req.FocusIDs()}, task, agent.Conversation{Provider: rt.Name()}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.AnalyzeCurrentCodebaseBatchPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseAnalyzeCurrentCodebaseBatchResult(output)
	if err != nil {
		return nil, err
	}
	result.Conversation = conversation
	return result, agent.RequireResult(result, "AnalyzeCurrentCodebaseBatch")
}

// AnalyzeCurrentDeltaBatch 基于 diff 判断知识变化。
func AnalyzeCurrentDeltaBatch(ctx context.Context, rt agent.LearningRuntime, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-delta-pack-analyze", req.RuntimeLabel))
	output, _, err := callWithOptions(ctx, rt, agent.AnalyzeCurrentDeltaBatchOperation(req), "learning-delta-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-delta-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentDeltaBatch, aicontract.StructuredOutputOptions{FocusIDs: req.FocusIDs()}, task, agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
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

// RefreshProjectProfile 刷新项目画像。
func RefreshProjectProfile(ctx context.Context, rt agent.LearningRuntime, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	output, _, err := call(ctx, rt, "LearningProfileRefresh", "learning-profile-refresh", "skills-seed-learning-profile-refresh", aicontract.ContractProjectProfile, agent.NewRuntimeTask(agent.RuntimeSlug("learning-profile-refresh", "")), agent.Conversation{}, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
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

// ExtractAuthority 从权威文件提取工程规则。
func ExtractAuthority(ctx context.Context, rt agent.LearningRuntime, req *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	opts := aicontract.StructuredOutputOptions{
		AuthoritySectionIDs: agent.AuthoritySectionIDs(req.AuthoritySections),
	}
	output, _, err := callWithOptions(
		ctx,
		rt,
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

// ReviewAuthority 复核权威规则提取结果。
func ReviewAuthority(ctx context.Context, rt agent.LearningRuntime, req *agent.ReviewAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	opts := aicontract.StructuredOutputOptions{
		AuthoritySectionIDs: agent.AuthoritySectionIDs(req.AuthoritySections),
	}
	output, _, err := callWithOptions(
		ctx,
		rt,
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
