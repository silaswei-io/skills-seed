package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

// ParseSelectFilesResult 解析 AI 文件选择器输出。
func ParseSelectFilesResult(output string) (*agent.SelectFilesResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, err
	}
	var result aicontract.SelectFilesOutput
	if err := parseJSONPayload(jsonStr, &result); err != nil {
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
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentExtractJSONFallback"),
			"method", "ParseAnalyzeResult",
			"error", err,
			"output_length", len(output),
		)
		return nil, err
	}

	var result aicontract.AnalyzeCodeOutput

	if err := parseJSONPayload(jsonStr, &result); err != nil {
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
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentExtractJSONFallback"),
			"method", "ParseGenerateFixesResult",
			"error", err,
			"output_length", len(output),
		)
		return &agent.GenerateFixesResult{
			Fixes:      make(map[string]string),
			Confidence: 0.0,
		}, nil
	}

	var result aicontract.GenerateFixesOutput

	if err := parseJSONPayload(jsonStr, &result); err != nil {
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
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentExtractJSONFallback"),
			"method", "ParseLearnResult",
			"error", err,
			"output_length", len(output),
		)
		return nil, err
	}

	var result aicontract.LearnPatternsOutput
	if err := parseJSONPayload(jsonStr, &result); err != nil {
		return nil, err
	}
	return &agent.LearnResult{Patterns: patternsToDomain(result.Patterns, domain.SourceLearned, time.Now())}, nil
}

// ParseBatchLearnResult 解析批量学习结果。
func ParseBatchLearnResult(output string) (*agent.BatchLearnResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentExtractJSONFallback"),
			"method", "ParseBatchLearnResult",
			"error", err,
			"output_length", len(output),
		)
		return nil, err
	}

	var result aicontract.LearnPatternsOutput
	if err := parseJSONPayload(jsonStr, &result); err != nil {
		return nil, err
	}
	return &agent.BatchLearnResult{Patterns: patternsToDomain(result.Patterns, domain.SourceLearned, time.Now())}, nil
}

// ParseCuratePatternsResult 解析模式策展结果。
func ParseCuratePatternsResult(output string) (*agent.CuratePatternsResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var result aicontract.CuratePatternsOutput

	if err := parseJSONPayload(jsonStr, &result); err != nil {
		return nil, err
	}

	curateResult := &agent.CuratePatternsResult{
		Patterns: make([]agent.CuratedPattern, len(result.Patterns)),
		Dropped:  make([]agent.CuratedDrop, len(result.Dropped)),
		Summary: agent.CurateSummary{
			TotalCandidates: result.Summary.TotalCandidates,
			TotalExisting:   result.Summary.TotalExisting,
			TotalWritten:    result.Summary.TotalWritten,
			TotalDropped:    result.Summary.TotalDropped,
			MergeCount:      result.Summary.MergeCount,
		},
	}

	now := time.Now()
	for i, p := range result.Patterns {
		curateResult.Patterns[i] = curatedPatternToAgent(p, now)
	}
	for i, dropped := range result.Dropped {
		curateResult.Dropped[i] = agent.CuratedDrop{
			ID:     dropped.ID,
			Reason: dropped.Reason,
		}
	}

	return curateResult, nil
}

// ParseUserDefinePatternResult 解析用户自定义模式结果。
func ParseUserDefinePatternResult(output string) (*agent.UserDefinePatternResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentNoValidJSONFound"), err)
	}

	var payload aicontract.PatternOutput
	if err := parseJSONPayload(jsonStr, &payload); err != nil {
		return nil, err
	}

	pattern := patternToDomain(payload, domain.SourceUserDefined, time.Now())
	return &agent.UserDefinePatternResult{Pattern: &pattern}, nil
}

// ParseAnalyzeProjectResult 解析项目分析结果。
func ParseAnalyzeProjectResult(output string) (*agent.AnalyzeProjectResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Error(i18n.Get("AgentExtractJSONFailed"), "error", err)
		logger.Error(i18n.Get("AgentOriginalOutput"), "output", output)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentExtractJSONError"), err)
	}

	var result aicontract.ProjectProfileOutput
	if err := parseJSONPayload(jsonStr, &result); err != nil {
		return nil, err
	}

	return projectProfileToAnalyzeProjectResult(result, time.Now()), nil
}

// ParseAnalyzeCurrentCodebaseResult 解析当前代码库分析结果。
func ParseAnalyzeCurrentCodebaseResult(output string) (*agent.AnalyzeCurrentCodebaseResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentExtractJSONFallback"),
			"method", "ParseAnalyzeCurrentCodebaseResult",
			"error", err,
			"output_length", len(output),
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var payload aicontract.AnalyzeCurrentCodebaseOutput
	if err := parseJSONPayload(jsonStr, &payload); err != nil {
		return nil, err
	}

	now := time.Now()
	return &agent.AnalyzeCurrentCodebaseResult{
		Patterns:                  patternsToDomain(payload.Patterns, domain.SourceInit, now),
		ProfileDelta:              projectProfileDeltaToDomain(payload.ProfileDelta, now),
		ProfileRefreshRecommended: profileRefreshRecommendationToAgent(payload.ProfileRefreshRecommended),
	}, nil
}

// ParseAnalyzeCurrentCodebaseBatchResult 解析当前代码库批量分析结果。
func ParseAnalyzeCurrentCodebaseBatchResult(output string) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentExtractJSONFallback"),
			"method", "ParseAnalyzeCurrentCodebaseBatchResult",
			"error", err,
			"output_length", len(output),
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var payload aicontract.AnalyzeCurrentCodebaseBatchOutput
	if err := parseJSONPayload(jsonStr, &payload); err != nil {
		return nil, err
	}

	now := time.Now()
	units := make([]agent.AnalyzeCurrentCodebaseUnitResult, 0, len(payload.Units))
	for _, unit := range payload.Units {
		units = append(units, agent.AnalyzeCurrentCodebaseUnitResult{
			UnitID:                    unit.UnitID,
			UnitName:                  unit.UnitName,
			Patterns:                  patternsToDomain(unit.Patterns, domain.SourceInit, now),
			ProfileDelta:              projectProfileDeltaToDomain(unit.ProfileDelta, now),
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
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentNoValidJSONFound"), err)
	}

	var result aicontract.OptimizeWorkflowOutput
	if err := parseJSONPayload(jsonStr, &result); err != nil {
		return nil, err
	}
	return &agent.OptimizeWorkflowResult{
		Title:       strings.TrimSpace(result.Title),
		Content:     strings.TrimSpace(result.Content),
		Summary:     strings.TrimSpace(result.Summary),
		Suggestions: result.Suggestions,
	}, nil
}
