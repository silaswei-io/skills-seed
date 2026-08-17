package parser

import (
	"errors"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

// ParseUserDefinePatternResult 解析用户自定义模式结果。
func ParseUserDefinePatternResult(output string) (*agent.UserDefinePatternResult, error) {
	var payload aicontract.PatternOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}

	pattern := patternToDomain(payload, domain.SourceUserDefined, time.Now())
	return &agent.UserDefinePatternResult{Pattern: &pattern}, nil
}

// ParseOptimizeContentResult 解析用户维护资源的优化结果。
func ParseOptimizeContentResult(output string) (*agent.OptimizeContentResult, error) {
	var result aicontract.OptimizedContentOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}
	return &agent.OptimizeContentResult{Content: strings.TrimSpace(result.Content)}, nil
}

// ParseAnalyzeProjectResult 解析项目分析结果。
func ParseAnalyzeProjectResult(output string) (*agent.AnalyzeProjectResult, error) {
	var result aicontract.ProjectProfileOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}

	return projectProfileToAnalyzeProjectResult(result), nil
}

// ParseExtractAuthorityResult 解析独立权威知识提取结果。
func ParseExtractAuthorityResult(output string) (*agent.ExtractAuthorityResult, error) {
	var result aicontract.AuthorityExtractionOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}
	return authorityExtractionToResult(result), nil
}

// ParseReviewKnowledgeResult 解析独立知识审查结果。
func ParseReviewKnowledgeResult(output string) (*agent.ReviewKnowledgeResult, error) {
	var payload aicontract.ReviewKnowledgeOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}
	now := time.Now()
	result := &agent.ReviewKnowledgeResult{Decisions: make([]agent.KnowledgeReviewDecision, 0, len(payload.Decisions))}
	for _, item := range payload.Decisions {
		decision := agent.KnowledgeReviewDecision{
			CandidateID: item.CandidateID,
			Verdict:     item.Verdict,
			ReasonCode:  item.ReasonCode,
			Reason:      item.Reason,
		}
		// 能力入口只有“完整替换”与“省略即移除”两种状态，避免判定字段与对象互相矛盾。
		if item.BusinessMethod != nil {
			decision.BusinessMethod = businessMethodToDomain(item.BusinessMethod, now)
		}
		if item.Verdict == "revise" && item.Revision != nil {
			decision.Revision = &agent.KnowledgeRevision{
				Name: item.Revision.Name, Category: item.Revision.Category,
				Description: item.Revision.Description, Rule: item.Revision.Rule,
				Confidence: item.Revision.Confidence, KnowledgeFlags: append([]string(nil), item.Revision.KnowledgeFlags...),
			}
		}
		result.Decisions = append(result.Decisions, decision)
	}
	return result, nil
}

// ParseAnalyzeCurrentCodebaseBatchResult 解析当前代码库批量分析结果。
func ParseAnalyzeCurrentCodebaseBatchResult(output string) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	var payload aicontract.AnalyzeCurrentCodebaseBatchOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}

	now := time.Now()
	focuses := make([]agent.AnalyzeCurrentEvidenceResult, 0, len(payload.Focuses))
	for _, unit := range payload.Focuses {
		if unit.ProfileRefreshRecommended == nil {
			return nil, missingRequiredOutputField("focuses[].profile_refresh_recommended")
		}
		focuses = append(focuses, agent.AnalyzeCurrentEvidenceResult{
			FocusID:                   unit.FocusID,
			FocusName:                 unit.FocusName,
			Patterns:                  patternsToDomain(unit.Patterns, domain.SourceLearnedCurrent, now),
			ProfileRefreshRecommended: profileRefreshRecommendationToAgent(*unit.ProfileRefreshRecommended),
		})
	}
	return &agent.AnalyzeCurrentCodebaseBatchResult{Focuses: focuses}, nil
}

// ParseAnalyzeCurrentDeltaBatchResult 解析 diff 锚定增量学习结果。
func ParseAnalyzeCurrentDeltaBatchResult(output string) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	var payload aicontract.AnalyzeCurrentDeltaBatchOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}
	if payload.ProfileRefreshRecommended == nil {
		return nil, missingRequiredOutputField("profile_refresh_recommended")
	}
	for _, change := range payload.KnowledgeChanges {
		if strings.TrimSpace(change.FocusID) == "" {
			return nil, missingRequiredOutputField("knowledge_changes[].focus_id")
		}
	}

	now := time.Now()
	return &agent.AnalyzeCurrentDeltaBatchResult{
		Changes:                   knowledgeChangesToDomain(payload.KnowledgeChanges, now),
		ProfileRefreshRecommended: profileRefreshRecommendationToAgent(*payload.ProfileRefreshRecommended),
	}, nil
}

func profileRefreshRecommendationToAgent(in aicontract.ProfileRefreshRecommendationOutput) agent.ProfileRefreshRecommendation {
	return agent.ProfileRefreshRecommendation{
		Needed: in.Needed,
		Reason: in.Reason,
	}
}

func missingRequiredOutputField(field string) error {
	return errors.New(i18n.GetWithParams("AgentRequiredOutputFieldMissing", map[string]interface{}{"Field": field}))
}
