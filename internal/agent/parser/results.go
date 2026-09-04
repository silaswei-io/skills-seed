package parser

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

// ParseUserDefinePatternResult 解析用户自定义模式结果。
func ParseUserDefinePatternResult(output string) (*agent.UserDefinePatternResult, error) {
	var payload aicontract.PatternOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}
	if err := validatePatternOutput(payload, "pattern"); err != nil {
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
	return &agent.OptimizeContentResult{
		Content:    strings.TrimSpace(result.Content),
		Summary:    strings.TrimSpace(result.Summary),
		RouteTerms: stringx.UniqueNonBlank(result.RouteTerms),
	}, nil
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
	if payload.Decisions == nil {
		return nil, missingRequiredOutputField("decisions")
	}
	now := time.Now()
	result := &agent.ReviewKnowledgeResult{Decisions: make([]agent.KnowledgeReviewDecision, 0, len(payload.Decisions))}
	for _, item := range payload.Decisions {
		if err := validateKnowledgeReviewDecision(item); err != nil {
			return nil, err
		}
		verdict := strings.TrimSpace(item.Verdict)
		decision := agent.KnowledgeReviewDecision{
			CandidateID: strings.TrimSpace(item.CandidateID),
			Verdict:     verdict,
			ReasonCode:  strings.TrimSpace(item.ReasonCode),
			Reason:      strings.TrimSpace(item.Reason),
		}
		// 能力入口只有“完整替换”与“省略即移除”两种状态，避免判定字段与对象互相矛盾。
		if item.BusinessMethod != nil {
			decision.BusinessMethod = businessMethodToDomain(item.BusinessMethod, now)
		}
		if verdict == "revise" && item.Revision != nil {
			decision.Revision = &agent.KnowledgeRevision{
				Name: strings.TrimSpace(item.Revision.Name), Category: string(domain.NormalizePatternCategory(domain.Category(item.Revision.Category))),
				Description: strings.TrimSpace(item.Revision.Description), Rule: strings.TrimSpace(item.Revision.Rule),
				Confidence: item.Revision.Confidence, KnowledgeFlags: append([]string(nil), item.Revision.KnowledgeFlags...),
			}
		}
		result.Decisions = append(result.Decisions, decision)
	}
	return result, nil
}

// validateKnowledgeReviewDecision 在解析边界拒绝缺失或越界的 AI 字段。
// 结构化输出提供方可能只校验 JSON 语法，不能依赖其替我们执行业务契约校验。
func validateKnowledgeReviewDecision(item aicontract.KnowledgeReviewDecisionOutput) error {
	candidateID := strings.TrimSpace(item.CandidateID)
	if candidateID == "" {
		return missingRequiredOutputField("decisions[].candidate_id")
	}
	verdict := strings.TrimSpace(item.Verdict)
	if !agent.ValidKnowledgeReviewVerdict(verdict) {
		return fmt.Errorf("knowledge review decision %q has invalid verdict %q", candidateID, item.Verdict)
	}
	reasonCode := strings.TrimSpace(item.ReasonCode)
	if !agent.ValidKnowledgeReviewReasonCode(reasonCode) {
		return fmt.Errorf("knowledge review decision %q has invalid reason_code %q", candidateID, item.ReasonCode)
	}
	if strings.TrimSpace(item.Reason) == "" {
		return missingRequiredOutputField("decisions[].reason")
	}
	if verdict != "revise" || item.Revision == nil {
		if verdict == "revise" {
			return missingRequiredOutputField("decisions[].revision")
		}
		return nil
	}
	if strings.TrimSpace(item.Revision.Name) == "" {
		return missingRequiredOutputField("decisions[].revision.name")
	}
	if !domain.IsValidPatternCategory(domain.Category(strings.TrimSpace(item.Revision.Category))) {
		return fmt.Errorf("knowledge review decision %q has invalid revision category %q", candidateID, item.Revision.Category)
	}
	if strings.TrimSpace(item.Revision.Description) == "" {
		return missingRequiredOutputField("decisions[].revision.description")
	}
	if strings.TrimSpace(item.Revision.Rule) == "" {
		return missingRequiredOutputField("decisions[].revision.rule")
	}
	if item.Revision.Confidence < 0 || item.Revision.Confidence > 1 {
		return fmt.Errorf("knowledge review decision %q has revision confidence outside [0,1]", candidateID)
	}
	if !domain.ValidKnowledgeFlags(item.Revision.KnowledgeFlags) {
		return fmt.Errorf("knowledge review decision %q has invalid revision flags", candidateID)
	}
	return nil
}

// ParseAnalyzeCurrentCodebaseBatchResult 解析当前代码库批量分析结果。
func ParseAnalyzeCurrentCodebaseBatchResult(output string) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	var payload aicontract.AnalyzeCurrentCodebaseBatchOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}
	if payload.Focuses == nil {
		return nil, missingRequiredOutputField("focuses")
	}

	now := time.Now()
	focuses := make([]agent.AnalyzeCurrentEvidenceResult, 0, len(payload.Focuses))
	for _, unit := range payload.Focuses {
		if strings.TrimSpace(unit.FocusID) == "" {
			return nil, missingRequiredOutputField("focuses[].focus_id")
		}
		if strings.TrimSpace(unit.FocusName) == "" {
			return nil, missingRequiredOutputField("focuses[].focus_name")
		}
		if unit.Patterns == nil {
			return nil, missingRequiredOutputField("focuses[].patterns")
		}
		if unit.ProfileRefreshRecommended == nil {
			return nil, missingRequiredOutputField("focuses[].profile_refresh_recommended")
		}
		for _, pattern := range unit.Patterns {
			if err := validatePatternOutput(pattern, "focuses[].patterns[]"); err != nil {
				return nil, err
			}
		}
		focuses = append(focuses, agent.AnalyzeCurrentEvidenceResult{
			FocusID:                   strings.TrimSpace(unit.FocusID),
			FocusName:                 strings.TrimSpace(unit.FocusName),
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
	if payload.KnowledgeChanges == nil {
		return nil, missingRequiredOutputField("knowledge_changes")
	}
	if payload.ProfileRefreshRecommended == nil {
		return nil, missingRequiredOutputField("profile_refresh_recommended")
	}
	for _, change := range payload.KnowledgeChanges {
		if strings.TrimSpace(change.FocusID) == "" {
			return nil, missingRequiredOutputField("knowledge_changes[].focus_id")
		}
		if change.Proposal != nil {
			if err := validatePatternOutput(*change.Proposal, "knowledge_changes[].proposal"); err != nil {
				return nil, err
			}
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

// validatePatternOutput 校验所有模式学习入口共用的候选模式契约。
func validatePatternOutput(pattern aicontract.PatternOutput, field string) error {
	if strings.TrimSpace(pattern.ID) == "" {
		return missingRequiredOutputField(field + ".id")
	}
	if strings.TrimSpace(pattern.Name) == "" {
		return missingRequiredOutputField(field + ".name")
	}
	if !domain.IsValidPatternCategory(domain.Category(strings.TrimSpace(pattern.Category))) {
		return fmt.Errorf("%s has invalid category %q", field, pattern.Category)
	}
	if strings.TrimSpace(pattern.Description) == "" {
		return missingRequiredOutputField(field + ".description")
	}
	if strings.TrimSpace(pattern.Rule) == "" {
		return missingRequiredOutputField(field + ".rule")
	}
	if pattern.Confidence < 0 || pattern.Confidence > 1 {
		return fmt.Errorf("%s has confidence outside [0,1]", field)
	}
	if pattern.Frequency < 1 {
		return fmt.Errorf("%s has frequency below 1", field)
	}
	if !domain.ValidKnowledgeFlags(pattern.KnowledgeFlags) {
		return fmt.Errorf("%s has invalid knowledge flags", field)
	}
	return nil
}
