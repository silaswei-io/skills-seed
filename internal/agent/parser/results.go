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

// ParseLearningSessionAck 验证当前代码学习会话初始化结果。
func ParseLearningSessionAck(output string) error {
	var result aicontract.LearningSessionAckOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return err
	}
	if !result.Ready {
		return errors.New(i18n.GetWithParams("AgentLearningSessionNotReady", map[string]interface{}{"Summary": strings.TrimSpace(result.Summary)}))
	}
	return nil
}

// ParseUserDefinePatternResult 解析用户自定义模式结果。
func ParseUserDefinePatternResult(output string) (*agent.UserDefinePatternResult, error) {
	var payload aicontract.PatternOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}

	pattern := patternToDomain(payload, domain.SourceUserDefined, time.Now())
	return &agent.UserDefinePatternResult{Pattern: &pattern}, nil
}

// ParseAnalyzeProjectResult 解析项目分析结果。
func ParseAnalyzeProjectResult(output string) (*agent.AnalyzeProjectResult, error) {
	var result aicontract.ProjectProfileOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}

	return projectProfileToAnalyzeProjectResult(result, time.Now()), nil
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

// ParseOptimizeWorkflowResult 解析工作流优化结果。
func ParseOptimizeWorkflowResult(output string) (*agent.OptimizeWorkflowResult, error) {
	var result aicontract.OptimizeWorkflowOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}
	return &agent.OptimizeWorkflowResult{
		Title:     strings.TrimSpace(result.Title),
		Content:   strings.TrimSpace(result.Content),
		Conflicts: result.Conflicts,
	}, nil
}
