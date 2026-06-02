// Package parser 提供 AI 输出的 JSON 提取和解析功能
//
// 从 Claude/Codex 等 AI Agent 的文本输出中提取结构化 JSON 数据，
// 并解析为具体的领域模型
package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

// ExtractJSON 从 AI 输出中提取 JSON
func ExtractJSON(output string) (string, error) {
	trimmed := strings.TrimSpace(output)
	if len(trimmed) == 0 {
		logger.Error(i18n.Get("AgentEmptyAIOutput"),
			"output_length", len(output),
			"hint", i18n.Get("AgentEmptyAIOutputHint"))
		return "", fmt.Errorf("%s", i18n.Get("AgentEmptyAIResponse"))
	}

	// 1. 尝试从 markdown 代码块提取
	re := regexp.MustCompile("(?s)```(?:json|JSON)?\\s*\\n?({.*?})\\s*\\n?```")
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		jsonStr := strings.TrimSpace(matches[1])
		if err := validateJSON(jsonStr); err == nil {
			return jsonStr, nil
		}
	}

	// 2. 找到第一个 { 和匹配的 }
	start := strings.Index(output, "{")
	if start == -1 {
		logger.Error(i18n.Get("AgentNoJSONFound"),
			"output_length", len(output),
			"output_preview", TruncString(output, 100),
			"hint", i18n.Get("AgentNoJSONFoundHint"))
		return "", fmt.Errorf("%s", i18n.Get("AgentNoJSONObjectFound"))
	}

	end := findMatchingBrace(output, start)
	if end == -1 {
		logger.Error(i18n.Get("AgentUnmatchedBraces"),
			"start", start,
			"output_length", len(output))
		return "", fmt.Errorf("%s", i18n.Get("AgentUnmatchedJSONBraces"))
	}

	jsonStr := strings.TrimSpace(output[start : end+1])

	// 3. 验证 JSON 有效性
	if err := validateJSON(jsonStr); err != nil {
		if repaired, repairErr := repairInvalidStringEscapes(jsonStr); repairErr == nil {
			if validateJSON(repaired) == nil {
				return repaired, nil
			}
		}
		logger.Error(i18n.Get("AgentInvalidJSON"),
			"error", err,
			"json_length", len(jsonStr),
			"json_preview", TruncString(jsonStr, 200))
		return "", fmt.Errorf("%s: %w", i18n.Get("AgentInvalidJSONError"), err)
	}

	return jsonStr, nil
}

// findMatchingBrace 找到匹配的结束括号
func findMatchingBrace(s string, start int) int {
	braceCount := 0
	inString := false
	escapeNext := false

	for i := start; i < len(s); i++ {
		ch := s[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if ch == '\\' {
			escapeNext = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if ch == '{' {
			braceCount++
		} else if ch == '}' {
			braceCount--
			if braceCount == 0 {
				return i
			}
		}
	}

	return -1
}

func validateJSON(jsonStr string) error {
	var js interface{}
	return json.Unmarshal([]byte(jsonStr), &js)
}

func repairInvalidStringEscapes(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	inString := false
	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]
		if !inString {
			b.WriteByte(ch)
			if ch == '"' {
				inString = true
			}
			continue
		}

		switch ch {
		case '"':
			b.WriteByte(ch)
			inString = false
		case '\\':
			if i+1 >= len(jsonStr) {
				b.WriteString(`\\`)
				continue
			}
			next := jsonStr[i+1]
			if isValidJSONEscape(next) {
				b.WriteByte(ch)
				b.WriteByte(next)
				i++
				if next == 'u' {
					for j := 0; j < 4 && i+1 < len(jsonStr); j++ {
						i++
						b.WriteByte(jsonStr[i])
					}
				}
				continue
			}
			b.WriteString(`\\`)
		default:
			b.WriteByte(ch)
		}
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	return b.String(), nil
}

func isValidJSONEscape(ch byte) bool {
	switch ch {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
		return true
	default:
		return false
	}
}

// TruncString 截断字符串用于日志输出
func TruncString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ParseAnalyzeResult 解析代码分析结果
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

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
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

// ParseLearnResult 解析学习结果
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
		Patterns []struct {
			ID             string  `json:"id"`
			Name           string  `json:"name"`
			Category       string  `json:"category"`
			Description    string  `json:"description"`
			GoodExample    string  `json:"good_example"`
			BadExample     string  `json:"bad_example"`
			Rule           string  `json:"rule"`
			Confidence     float64 `json:"confidence"`
			Frequency      int     `json:"frequency"`
			BusinessMethod *struct {
				Name        string `json:"name"`
				Location    string `json:"location"`
				Description string `json:"description"`
				Usage       string `json:"usage"`
				Type        string `json:"type"`
				Function    string `json:"function"`
			} `json:"business_method"`
		} `json:"patterns"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	patterns := make([]domain.Pattern, len(result.Patterns))
	for i, p := range result.Patterns {
		pattern := domain.Pattern{
			ID:          p.ID,
			Name:        p.Name,
			Category:    domain.Category(p.Category),
			Description: p.Description,
			GoodExample: p.GoodExample,
			BadExample:  p.BadExample,
			Rule:        p.Rule,
			Confidence:  p.Confidence,
			Frequency:   p.Frequency,
			Source:      domain.SourceLearned,
			CreatedAt:   time.Now(),
		}

		if p.BusinessMethod != nil {
			pattern.BusinessMethod = &domain.BusinessMethod{
				Name:        p.BusinessMethod.Name,
				Location:    p.BusinessMethod.Location,
				Description: p.BusinessMethod.Description,
				Usage:       p.BusinessMethod.Usage,
				Type:        p.BusinessMethod.Type,
				Function:    p.BusinessMethod.Function,
			}
		}

		patterns[i] = pattern
	}

	return &agent.LearnResult{
		Patterns: patterns,
	}, nil
}

// ParseBatchLearnResult 解析批量学习结果
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
		Patterns []struct {
			ID             string  `json:"id"`
			Name           string  `json:"name"`
			Category       string  `json:"category"`
			Description    string  `json:"description"`
			GoodExample    string  `json:"good_example"`
			BadExample     string  `json:"bad_example"`
			Rule           string  `json:"rule"`
			Confidence     float64 `json:"confidence"`
			Frequency      int     `json:"frequency"`
			BusinessMethod *struct {
				Name          string `json:"name"`
				Location      string `json:"location"`
				Description   string `json:"description"`
				Usage         string `json:"usage"`
				Type          string `json:"type"`
				Function      string `json:"function"`
				Prerequisites string `json:"prerequisites"`
				Returns       string `json:"returns"`
			} `json:"business_method"`
		} `json:"patterns"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	patterns := make([]domain.Pattern, len(result.Patterns))
	for i, p := range result.Patterns {
		pattern := domain.Pattern{
			ID:          p.ID,
			Name:        p.Name,
			Category:    domain.Category(p.Category),
			Description: p.Description,
			GoodExample: p.GoodExample,
			BadExample:  p.BadExample,
			Rule:        p.Rule,
			Confidence:  p.Confidence,
			Frequency:   p.Frequency,
			Source:      domain.SourceLearned,
			CreatedAt:   time.Now(),
		}
		if p.BusinessMethod != nil {
			pattern.BusinessMethod = &domain.BusinessMethod{
				Name:          p.BusinessMethod.Name,
				Location:      p.BusinessMethod.Location,
				Description:   p.BusinessMethod.Description,
				Usage:         p.BusinessMethod.Usage,
				Type:          p.BusinessMethod.Type,
				Function:      p.BusinessMethod.Function,
				Prerequisites: p.BusinessMethod.Prerequisites,
				Returns:       p.BusinessMethod.Returns,
			}
		}
		patterns[i] = pattern
	}

	return &agent.BatchLearnResult{
		Patterns: patterns,
	}, nil
}

// ParseGenerateFixesResult 解析生成修复结果
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

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	return &agent.GenerateFixesResult{
		Fixes:      result.Fixes,
		Confidence: result.Confidence,
		Summary:    result.Summary,
		Warnings:   result.Warnings,
	}, nil
}

// ParseGenerateSkillsResult 解析 Skills 生成结果
func ParseGenerateSkillsResult(output string) (*agent.GenerateSkillsResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var result struct {
		CategorySummaries map[string]struct {
			Category        string   `json:"category"`
			Summary         string   `json:"summary"`
			Patterns        []string `json:"patterns"`
			UsageScenes     []string `json:"usage_scenes"`
			Priority        int      `json:"priority"`
			BusinessMethods []*struct {
				Name          string `json:"name"`
				Location      string `json:"location"`
				Description   string `json:"description"`
				Usage         string `json:"usage"`
				Type          string `json:"type"`
				Function      string `json:"function"`
				Prerequisites string `json:"prerequisites"`
				Returns       string `json:"returns"`
			} `json:"business_methods"`
		} `json:"category_summaries"`
		KeyPatterns []struct {
			Name       string `json:"name"`
			Category   string `json:"category"`
			Importance string `json:"importance"`
			Summary    string `json:"summary"`
			WhenToUse  string `json:"when_to_use"`
		} `json:"key_patterns"`
		BusinessRules          []string `json:"business_rules"`
		BestPractices          []string `json:"best_practices"`
		CommonPatterns         []string `json:"common_patterns"`
		KeyInsights            []string `json:"key_insights"`
		ImprovementSuggestions []string `json:"improvement_suggestions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	generateResult := &agent.GenerateSkillsResult{
		CategorySummaries:      make(map[string]agent.CategorySummary),
		KeyPatterns:            []agent.PatternSummary{},
		BusinessRules:          result.BusinessRules,
		BestPractices:          result.BestPractices,
		CommonPatterns:         result.CommonPatterns,
		KeyInsights:            result.KeyInsights,
		ImprovementSuggestions: result.ImprovementSuggestions,
	}

	for key, summary := range result.CategorySummaries {
		var businessMethods []*domain.BusinessMethod
		for _, bm := range summary.BusinessMethods {
			if bm != nil {
				businessMethods = append(businessMethods, &domain.BusinessMethod{
					Name:          bm.Name,
					Location:      bm.Location,
					Description:   bm.Description,
					Usage:         bm.Usage,
					Type:          bm.Type,
					Function:      bm.Function,
					Prerequisites: bm.Prerequisites,
					Returns:       bm.Returns,
				})
			}
		}

		generateResult.CategorySummaries[key] = agent.CategorySummary{
			Category:        summary.Category,
			Summary:         summary.Summary,
			Patterns:        summary.Patterns,
			UsageScenes:     summary.UsageScenes,
			Priority:        summary.Priority,
			BusinessMethods: businessMethods,
		}
	}

	for _, p := range result.KeyPatterns {
		generateResult.KeyPatterns = append(generateResult.KeyPatterns, agent.PatternSummary{
			Name:       p.Name,
			Category:   p.Category,
			Importance: p.Importance,
			Summary:    p.Summary,
			WhenToUse:  p.WhenToUse,
		})
	}

	return generateResult, nil
}

// ParseMergePatternsResult 解析模式合并结果
func ParseMergePatternsResult(output string) (*agent.MergePatternsResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var result struct {
		MergedPatterns []struct {
			ID              string   `json:"id"`
			Name            string   `json:"name"`
			Category        string   `json:"category"`
			Description     string   `json:"description"`
			GoodExample     string   `json:"good_example"`
			BadExample      string   `json:"bad_example"`
			Rule            string   `json:"rule"`
			Confidence      float64  `json:"confidence"`
			MergedFrom      []string `json:"merged_from"`
			MergeReason     string   `json:"merge_reason"`
			SimilarityScore float64  `json:"similarity_score"`
			BusinessMethod  *struct {
				Name          string `json:"name"`
				Location      string `json:"location"`
				Description   string `json:"description"`
				Usage         string `json:"usage"`
				Type          string `json:"type"`
				Function      string `json:"function"`
				Prerequisites string `json:"prerequisites"`
				Returns       string `json:"returns"`
			} `json:"business_method"`
		} `json:"merged_patterns"`
		UnchangedPatterns []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"unchanged_patterns"`
		Summary struct {
			TotalInput     int `json:"total_input"`
			TotalMerged    int `json:"total_merged"`
			TotalUnchanged int `json:"total_unchanged"`
			MergeCount     int `json:"merge_count"`
		} `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	mergeResult := &agent.MergePatternsResult{
		MergedPatterns:    make([]agent.MergedPattern, len(result.MergedPatterns)),
		UnchangedPatterns: make([]agent.UnchangedPattern, len(result.UnchangedPatterns)),
		Summary: agent.MergeSummary{
			TotalInput:     result.Summary.TotalInput,
			TotalMerged:    result.Summary.TotalMerged,
			TotalUnchanged: result.Summary.TotalUnchanged,
			MergeCount:     result.Summary.MergeCount,
		},
	}

	for i, p := range result.MergedPatterns {
		var businessMethod *domain.BusinessMethod
		if p.BusinessMethod != nil {
			businessMethod = &domain.BusinessMethod{
				Name:          p.BusinessMethod.Name,
				Location:      p.BusinessMethod.Location,
				Description:   p.BusinessMethod.Description,
				Usage:         p.BusinessMethod.Usage,
				Type:          p.BusinessMethod.Type,
				Function:      p.BusinessMethod.Function,
				Prerequisites: p.BusinessMethod.Prerequisites,
				Returns:       p.BusinessMethod.Returns,
			}
		}
		mergeResult.MergedPatterns[i] = agent.MergedPattern{
			ID:              p.ID,
			Name:            p.Name,
			Category:        p.Category,
			Description:     p.Description,
			GoodExample:     p.GoodExample,
			BadExample:      p.BadExample,
			Rule:            p.Rule,
			Confidence:      p.Confidence,
			MergedFrom:      p.MergedFrom,
			MergeReason:     p.MergeReason,
			SimilarityScore: p.SimilarityScore,
			BusinessMethod:  businessMethod,
		}
	}

	for i, p := range result.UnchangedPatterns {
		mergeResult.UnchangedPatterns[i] = agent.UnchangedPattern{
			ID:     p.ID,
			Reason: p.Reason,
		}
	}

	return mergeResult, nil
}

// ParseAnalyzeProjectResult 解析项目分析结果
func ParseAnalyzeProjectResult(output string) (*agent.AnalyzeProjectResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Error(i18n.Get("AgentExtractJSONFailed"), "error", err)
		logger.Error(i18n.Get("AgentOriginalOutput"), "output", output)
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentExtractJSONError"), err)
	}

	var result struct {
		ProjectName  string   `json:"project_name"`
		Language     string   `json:"language"`
		Frameworks   []string `json:"frameworks"`
		Architecture string   `json:"architecture"`
		Structure    string   `json:"structure"`
		Layers       []struct {
			Name             string   `json:"name"`
			Description      string   `json:"description"`
			Responsibilities []string `json:"responsibilities"`
			Files            []string `json:"files"`
		} `json:"layers"`
		DependencyGraph   string   `json:"dependency_graph"`
		DataFlow          string   `json:"data_flow"`
		FrameworkPatterns []string `json:"framework_patterns"`
		CommonUtils       []struct {
			Name        string `json:"name"`
			File        string `json:"file"`
			Signature   string `json:"signature"`
			Description string `json:"description"`
			Usage       string `json:"usage"`
		} `json:"common_utils"`
		KeyModules []struct {
			Name             string   `json:"name"`
			Path             string   `json:"path"`
			Description      string   `json:"description"`
			Responsibilities []string `json:"responsibilities"`
			Dependencies     []string `json:"dependencies"`
			Dependents       []string `json:"dependents"`
			KeyMethods       []string `json:"key_methods"`
		} `json:"key_modules"`
		ConfigPatterns  []string `json:"config_patterns"`
		Dependencies    []string `json:"dependencies"`
		BusinessMethods []struct {
			Name          string `json:"name"`
			Location      string `json:"location"`
			Description   string `json:"description"`
			Usage         string `json:"usage"`
			Type          string `json:"type"`
			Function      string `json:"function"`
			Prerequisites string `json:"prerequisites"`
			Returns       string `json:"returns"`
		} `json:"business_methods"`
		Summary string `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	if result.Frameworks == nil {
		result.Frameworks = []string{}
	}
	if result.Dependencies == nil {
		result.Dependencies = []string{}
	}
	if result.ConfigPatterns == nil {
		result.ConfigPatterns = []string{}
	}

	analyzeResult := &agent.AnalyzeProjectResult{
		ProjectName:       result.ProjectName,
		Language:          result.Language,
		Frameworks:        result.Frameworks,
		Architecture:      result.Architecture,
		Structure:         result.Structure,
		DependencyGraph:   result.DependencyGraph,
		DataFlow:          result.DataFlow,
		FrameworkPatterns: result.FrameworkPatterns,
		ConfigPatterns:    result.ConfigPatterns,
		Dependencies:      result.Dependencies,
		Summary:           result.Summary,
		Layers:            make([]domain.ArchitectureLayer, len(result.Layers)),
		CommonUtils:       make([]domain.UtilityFunction, len(result.CommonUtils)),
		KeyModules:        make([]domain.ModuleInfo, len(result.KeyModules)),
		BusinessMethods:   make([]domain.BusinessMethod, len(result.BusinessMethods)),
	}

	for i, layer := range result.Layers {
		analyzeResult.Layers[i] = domain.ArchitectureLayer{
			Name:             layer.Name,
			Description:      layer.Description,
			Responsibilities: layer.Responsibilities,
			Files:            layer.Files,
		}
	}

	for i, util := range result.CommonUtils {
		analyzeResult.CommonUtils[i] = domain.UtilityFunction{
			Name:        util.Name,
			File:        util.File,
			Signature:   util.Signature,
			Description: util.Description,
			Usage:       util.Usage,
		}
	}

	for i, module := range result.KeyModules {
		analyzeResult.KeyModules[i] = domain.ModuleInfo{
			Name:             module.Name,
			Path:             module.Path,
			Description:      module.Description,
			Responsibilities: module.Responsibilities,
			Dependencies:     module.Dependencies,
			Dependents:       module.Dependents,
			KeyMethods:       module.KeyMethods,
		}
	}

	for i, method := range result.BusinessMethods {
		analyzeResult.BusinessMethods[i] = domain.BusinessMethod{
			Name:          method.Name,
			Location:      method.Location,
			Description:   method.Description,
			Usage:         method.Usage,
			Type:          method.Type,
			Function:      method.Function,
			Prerequisites: method.Prerequisites,
			Returns:       method.Returns,
		}
	}

	return analyzeResult, nil
}

// ParseAnalyzeCurrentCodebaseResult 解析当前代码库分析结果
func ParseAnalyzeCurrentCodebaseResult(output string) (*agent.AnalyzeCurrentCodebaseResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		logger.Warn(i18n.Get("LoggerAgentExtractJSONFallback"),
			"method", "ParseAnalyzeCurrentCodebaseResult",
			"error", err,
			"output_length", len(output),
			"output_preview", TruncString(output, 500),
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var result struct {
		Patterns []struct {
			ID             string  `json:"id"`
			Name           string  `json:"name"`
			Category       string  `json:"category"`
			Description    string  `json:"description"`
			GoodExample    string  `json:"good_example"`
			BadExample     string  `json:"bad_example"`
			Rule           string  `json:"rule"`
			Confidence     float64 `json:"confidence"`
			Frequency      int     `json:"frequency"`
			BusinessMethod *struct {
				Name          string `json:"name"`
				Location      string `json:"location"`
				Description   string `json:"description"`
				Usage         string `json:"usage"`
				Type          string `json:"type"`
				Function      string `json:"function"`
				Prerequisites string `json:"prerequisites"`
				Returns       string `json:"returns"`
			} `json:"business_method"`
		} `json:"patterns"`
		CategorySummaries map[string]struct {
			Summary     string   `json:"summary"`
			Patterns    []string `json:"patterns"`
			UsageScenes []string `json:"usage_scenes"`
			Priority    int      `json:"priority"`
		} `json:"category_summaries"`
		BusinessRules  []string `json:"business_rules"`
		BestPractices  []string `json:"best_practices"`
		CommonPatterns []string `json:"common_patterns"`
		Summary        string   `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	patterns := make([]domain.Pattern, len(result.Patterns))
	for i, p := range result.Patterns {
		pattern := domain.Pattern{
			ID:          p.ID,
			Name:        p.Name,
			Category:    domain.Category(p.Category),
			Description: p.Description,
			GoodExample: p.GoodExample,
			BadExample:  p.BadExample,
			Rule:        p.Rule,
			Confidence:  p.Confidence,
			Frequency:   p.Frequency,
			Source:      domain.SourceInit,
			CreatedAt:   time.Now(),
		}
		if p.BusinessMethod != nil {
			pattern.BusinessMethod = &domain.BusinessMethod{
				Name:          p.BusinessMethod.Name,
				Location:      p.BusinessMethod.Location,
				Description:   p.BusinessMethod.Description,
				Usage:         p.BusinessMethod.Usage,
				Type:          p.BusinessMethod.Type,
				Function:      p.BusinessMethod.Function,
				Prerequisites: p.BusinessMethod.Prerequisites,
				Returns:       p.BusinessMethod.Returns,
			}
		}
		patterns[i] = pattern
	}

	categorySummaries := make(map[string]agent.CategorySummary)
	for category, summary := range result.CategorySummaries {
		categorySummaries[category] = agent.CategorySummary{
			Category:    category,
			Summary:     summary.Summary,
			Patterns:    summary.Patterns,
			UsageScenes: summary.UsageScenes,
			Priority:    summary.Priority,
		}
	}

	return &agent.AnalyzeCurrentCodebaseResult{
		Patterns:          patterns,
		CategorySummaries: categorySummaries,
		BusinessRules:     result.BusinessRules,
		BestPractices:     result.BestPractices,
		CommonPatterns:    result.CommonPatterns,
		Summary:           result.Summary,
	}, nil
}

// ParseWorkspaceProfile 解析工作区画像结果
func ParseWorkspaceProfile(output string) (*domain.WorkspaceProfile, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentExtractJSONError"), err)
	}

	var profile domain.WorkspaceProfile
	if err := json.Unmarshal([]byte(jsonStr), &profile); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	if profile.Projects == nil {
		profile.Projects = []domain.WorkspaceProject{}
	}
	if profile.Shared == nil {
		profile.Shared = []domain.WorkspacePath{}
	}
	if profile.Contracts == nil {
		profile.Contracts = []domain.WorkspacePath{}
	}
	if profile.Infra == nil {
		profile.Infra = []domain.WorkspacePath{}
	}
	if profile.Dependencies == nil {
		profile.Dependencies = []domain.WorkspaceDependency{}
	}
	if profile.ImpactRoutes == nil {
		profile.ImpactRoutes = []domain.WorkspaceRoute{}
	}
	return &profile, nil
}

// ParseWorkspaceSpec 解析工作区开发规范结果
func ParseWorkspaceSpec(output string) (*domain.WorkspaceSpec, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentExtractJSONError"), err)
	}

	var spec domain.WorkspaceSpec
	if err := json.Unmarshal([]byte(jsonStr), &spec); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	if spec.Projects == nil {
		spec.Projects = []domain.WorkspaceProject{}
	}
	if spec.Routing == nil {
		spec.Routing = []domain.WorkspaceRoute{}
	}
	if spec.Rules == nil {
		spec.Rules = []domain.WorkspaceRule{}
	}
	if spec.ChangeOrder == nil {
		spec.ChangeOrder = []string{}
	}
	if spec.ParallelAgentGuidance == nil {
		spec.ParallelAgentGuidance = []domain.WorkspaceParallelGuidance{}
	}
	if spec.LoadMultipleSkillsWhen == nil {
		spec.LoadMultipleSkillsWhen = []domain.WorkspaceLoadMultipleSkill{}
	}
	return &spec, nil
}
