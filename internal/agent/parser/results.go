package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
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
	var result struct {
		Include       []string `json:"include"`
		Exclude       []string `json:"exclude"`
		SelectedPaths []string `json:"selected_paths"`
		Reason        string   `json:"reason"`
	}
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

	var result struct {
		Issues []struct {
			File       string `json:"file"`
			Line       int    `json:"line"`
			Severity   string `json:"severity"`
			Message    string `json:"message"`
			Suggestion string `json:"suggestion"`
			PatternID  string `json:"pattern_id"`
		} `json:"issues"`
		Suggestions []string `json:"suggestions"`
		Confidence  float64  `json:"confidence"`
	}

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

	var result struct {
		Fixes      map[string]string `json:"fixes"`
		Confidence float64           `json:"confidence"`
		Summary    string            `json:"summary"`
		Warnings   []string          `json:"warnings"`
	}

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

	var result struct {
		Patterns []patternPayload `json:"patterns"`
	}
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

	var result struct {
		Patterns []patternPayload `json:"patterns"`
	}
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

	var result struct {
		Patterns []curatedPatternPayload `json:"patterns"`
		Dropped  []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"dropped"`
		Summary struct {
			TotalCandidates int `json:"total_candidates"`
			TotalExisting   int `json:"total_existing"`
			TotalWritten    int `json:"total_written"`
			TotalDropped    int `json:"total_dropped"`
			MergeCount      int `json:"merge_count"`
		} `json:"summary"`
	}

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
		curateResult.Patterns[i] = p.toCuratedPattern(now)
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

	var payload patternPayload
	if err := parseJSONPayload(jsonStr, &payload); err != nil {
		return nil, err
	}

	pattern := payload.toDomainPattern(domain.SourceUserDefined, time.Now())
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

	var result projectProfilePayload
	if err := parseJSONPayload(jsonStr, &result); err != nil {
		return nil, err
	}

	return result.toAnalyzeProjectResult(time.Now()), nil
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

	var payload struct {
		Patterns                  []patternPayload                   `json:"patterns"`
		ProfileDelta              projectProfilePayload              `json:"profile_delta"`
		ProfileRefreshRecommended agent.ProfileRefreshRecommendation `json:"profile_refresh_recommended"`
	}
	if err := parseJSONPayload(jsonStr, &payload); err != nil {
		return nil, err
	}

	now := time.Now()
	return &agent.AnalyzeCurrentCodebaseResult{
		Patterns:                  patternsToDomain(payload.Patterns, domain.SourceInit, now),
		ProfileDelta:              payload.ProfileDelta.toProjectProfileDelta(now),
		ProfileRefreshRecommended: payload.ProfileRefreshRecommended,
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

	var payload struct {
		Units []struct {
			UnitID                    string                             `json:"unit_id"`
			UnitName                  string                             `json:"unit_name"`
			Patterns                  []patternPayload                   `json:"patterns"`
			ProfileDelta              projectProfilePayload              `json:"profile_delta"`
			ProfileRefreshRecommended agent.ProfileRefreshRecommendation `json:"profile_refresh_recommended"`
		} `json:"units"`
	}
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
			ProfileDelta:              unit.ProfileDelta.toProjectProfileDelta(now),
			ProfileRefreshRecommended: unit.ProfileRefreshRecommended,
		})
	}
	return &agent.AnalyzeCurrentCodebaseBatchResult{Units: units}, nil
}

// ParseOptimizeWorkflowResult 解析工作流优化结果。
func ParseOptimizeWorkflowResult(output string) (*agent.OptimizeWorkflowResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentNoValidJSONFound"), err)
	}

	var result struct {
		Title       string   `json:"title"`
		Content     string   `json:"content"`
		Summary     string   `json:"summary"`
		Suggestions []string `json:"suggestions"`
	}
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
