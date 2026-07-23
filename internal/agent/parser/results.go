package parser

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

// ParseSelectFilesResult 解析 AI 文件筛选器输出。
func ParseSelectFilesResult(output string) (*agent.SelectFilesResult, error) {
	var result aicontract.SelectFilesOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}
	return &agent.SelectFilesResult{
		Include:       result.Include,
		Exclude:       result.Exclude,
		SelectedPaths: result.SelectedPaths,
		Reason:        result.Reason,
	}, nil
}

// ParseAnalyzeResult 解析代码分析结果。
func ParseAnalyzeResult(output string) (*agent.AnalyzeResult, error) {
	var result aicontract.AnalyzeCodeOutput

	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}

	issues := make([]domain.Issue, len(result.Issues))
	for i, issue := range result.Issues {
		issues[i] = domain.Issue{
			File:       issue.File,
			Line:       issue.Line,
			Severity:   domain.Severity(issue.Severity),
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
			PatternID:  issue.PatternID,
		}
	}

	return &agent.AnalyzeResult{
		Issues:      issues,
		Suggestions: result.Suggestions,
		Confidence:  result.Confidence,
	}, nil
}

// ParseGenerateFixesResult 解析生成修复结果。
func ParseGenerateFixesResult(output string) (*agent.GenerateFixesResult, error) {
	var result aicontract.GenerateFixesOutput

	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}

	return &agent.GenerateFixesResult{
		Fixes:      result.Fixes,
		Confidence: result.Confidence,
		Summary:    result.Summary,
		Warnings:   result.Warnings,
	}, nil
}

// ParseLearnResult 解析学习结果。
func ParseLearnResult(output string) (*agent.LearnResult, error) {
	var result aicontract.LearnPatternsOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}
	return &agent.LearnResult{Patterns: patternsToDomain(result.Patterns, domain.SourceLearned, time.Now())}, nil
}

// ParseBatchLearnResult 解析批量学习结果。
func ParseBatchLearnResult(output string) (*agent.BatchLearnResult, error) {
	var result aicontract.LearnPatternsOutput
	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}
	return &agent.BatchLearnResult{Patterns: patternsToDomain(result.Patterns, domain.SourceLearned, time.Now())}, nil
}

// ParseCuratePatternsResult 解析模式策展结果。
func ParseCuratePatternsResult(output string) (*agent.CuratePatternsResult, error) {
	var result aicontract.CuratePatternsOutput

	if err := parseJSONPayload(output, &result); err != nil {
		return nil, err
	}

	curateResult := &agent.CuratePatternsResult{
		Patterns: make([]agent.CuratedPattern, len(result.Patterns)),
		Dropped:  make([]agent.CuratedDrop, len(result.Dropped)),
	}

	for i, p := range result.Patterns {
		curateResult.Patterns[i] = curatedPatternToAgent(p)
	}
	for i, dropped := range result.Dropped {
		curateResult.Dropped[i] = agent.CuratedDrop{
			ID:     dropped.ID,
			Reason: dropped.Reason,
		}
	}

	return curateResult, nil
}

// ParseCuratePatternsArtifact 解析直接结构化输出或 Claude CLI 保存的 JSON envelope。
func ParseCuratePatternsArtifact(output string) (*agent.CuratePatternsResult, error) {
	trimmed := strings.TrimSpace(output)
	if strings.HasPrefix(trimmed, "```json") && strings.HasSuffix(trimmed, "```") {
		trimmed = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "```json"), "```"))
	}
	var envelope struct {
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if json.Unmarshal([]byte(trimmed), &envelope) == nil && len(envelope.StructuredOutput) > 0 && string(envelope.StructuredOutput) != "null" {
		trimmed = string(envelope.StructuredOutput)
	}
	return ParseCuratePatternsResult(trimmed)
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

// ParseAnalyzeCurrentCodebaseResult 解析当前代码库分析结果。
func ParseAnalyzeCurrentCodebaseResult(output string) (*agent.AnalyzeCurrentCodebaseResult, error) {
	var payload aicontract.AnalyzeCurrentCodebaseOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}

	now := time.Now()
	return &agent.AnalyzeCurrentCodebaseResult{
		Patterns:                  patternsToDomain(payload.Patterns, domain.SourceLearnedCurrent, now),
		ProfileRefreshRecommended: profileRefreshRecommendationToAgent(payload.ProfileRefreshRecommended),
	}, nil
}

// ParseAnalyzeCurrentCodebaseBatchResult 解析当前代码库批量分析结果。
func ParseAnalyzeCurrentCodebaseBatchResult(output string) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	var payload aicontract.AnalyzeCurrentCodebaseBatchOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}

	now := time.Now()
	units := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, len(payload.Units))
	for _, unit := range payload.Units {
		units = append(units, agent.AnalyzeCurrentCodebaseUnitResult{
			UnitID:                    unit.UnitID,
			UnitName:                  unit.UnitName,
			Patterns:                  patternsToDomain(unit.Patterns, domain.SourceLearnedCurrent, now),
			ProfileRefreshRecommended: profileRefreshRecommendationToAgent(unit.ProfileRefreshRecommended),
		})
	}
	return &agent.AnalyzeCurrentCodebaseBatchResult{Units: units}, nil
}

func profileRefreshRecommendationToAgent(in aicontract.ProfileRefreshRecommendationOutput) agent.ProfileRefreshRecommendation {
	return agent.ProfileRefreshRecommendation{
		Needed: in.Needed,
		Reason: in.Reason,
	}
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
