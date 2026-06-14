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

type businessMethodPayload struct {
	Name          string              `json:"name"`
	CodeLocation  domain.CodeLocation `json:"code_location"`
	Description   string              `json:"description"`
	Usage         string              `json:"usage"`
	Type          string              `json:"type"`
	Function      string              `json:"function"`
	Prerequisites string              `json:"prerequisites"`
	Returns       string              `json:"returns"`
}

type workspaceSpecPayload struct {
	domain.WorkspaceSpec
	ChangeOrder []json.RawMessage `json:"change_order"`
}

func (p *businessMethodPayload) toDomain(now time.Time) *domain.BusinessMethod {
	if p == nil {
		return nil
	}
	method := &domain.BusinessMethod{
		Name:          p.Name,
		Description:   p.Description,
		Usage:         p.Usage,
		Type:          p.Type,
		Function:      p.Function,
		Prerequisites: p.Prerequisites,
		Returns:       p.Returns,
		CodeLocation:  p.CodeLocation,
	}
	method.NormalizeCodeLocation(nil, now)
	return method
}

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
	re := regexp.MustCompile("(?s)```(?:json|JSON)?\\s*\\n")
	if re.MatchString(output) {
		jsonStr := extractJSONFromCodeBlock(output)
		if jsonStr != "" {
			if repaired, err := FixAIJSON(jsonStr); err == nil {
				return repaired, nil
			}
		}
	}

	// 2. 找到第一个 { 和匹配的 }
	start := strings.Index(output, "{")
	if start == -1 {
		logger.Error(i18n.Get("AgentNoJSONFound"),
			"output_length", len(output),
			"hint", i18n.Get("AgentNoJSONFoundHint"))
		return "", fmt.Errorf("%s", i18n.Get("AgentNoJSONObjectFound"))
	}

	end := findMatchingBrace(output, start)
	if end == -1 {
		if repaired, repairErr := FixAIJSON(output[start:]); repairErr == nil {
			return repaired, nil
		}
		logger.Error(i18n.Get("AgentUnmatchedBraces"),
			"start", start,
			"output_length", len(output))
		return "", fmt.Errorf("%s", i18n.Get("AgentUnmatchedJSONBraces"))
	}

	jsonStr := strings.TrimSpace(output[start : end+1])

	// 3. 验证 JSON 有效性
	repaired, err := FixAIJSON(jsonStr)
	if err != nil {
		logger.Error(i18n.Get("AgentInvalidJSON"),
			"error", err,
			"json_length", len(jsonStr))
		return "", fmt.Errorf("%s: %w", i18n.Get("AgentInvalidJSONError"), err)
	}

	return repaired, nil
}

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
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	return &agent.SelectFilesResult{
		Include:       result.Include,
		Exclude:       result.Exclude,
		SelectedPaths: result.SelectedPaths,
		Reason:        result.Reason,
	}, nil
}

func repairDuplicatedObjectStarts(jsonStr string) (string, error) {
	var b strings.Builder
	b.Grow(len(jsonStr))

	inString := false
	escapeNext := false
	repaired := false

	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]
		if escapeNext {
			b.WriteByte(ch)
			escapeNext = false
			continue
		}

		if ch == '\\' {
			b.WriteByte(ch)
			if inString {
				escapeNext = true
			}
			continue
		}

		if ch == '"' {
			b.WriteByte(ch)
			inString = !inString
			continue
		}

		if !inString && ch == '{' && i+2 < len(jsonStr) && jsonStr[i+1] == '{' && jsonStr[i+2] == '"' {
			b.WriteByte(ch)
			repaired = true
			i++
			continue
		}
		if !inString && ch == '{' && i+3 < len(jsonStr) && jsonStr[i+1] == '"' && jsonStr[i+2] == '{' && jsonStr[i+3] == '"' {
			b.WriteByte(ch)
			repaired = true
			i += 2
			continue
		}

		b.WriteByte(ch)
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if !repaired {
		return jsonStr, nil
	}
	return b.String(), nil
}

func repairMissingClosingContainers(jsonStr string) (string, error) {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" || jsonStr[0] != '{' {
		return "", fmt.Errorf("JSON object start not found")
	}

	stack := make([]byte, 0, 8)
	inString := false
	escapeNext := false
	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]
		if escapeNext {
			escapeNext = false
			continue
		}
		if ch == '\\' {
			if inString {
				escapeNext = true
			}
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) == 0 || stack[len(stack)-1] != ch {
				return "", fmt.Errorf("JSON containers are mismatched")
			}
			stack = stack[:len(stack)-1]
		}
	}
	if inString {
		return "", fmt.Errorf("unterminated JSON string")
	}
	if len(stack) == 0 {
		return jsonStr, nil
	}

	var b strings.Builder
	b.Grow(len(jsonStr) + len(stack))
	b.WriteString(jsonStr)
	for i := len(stack) - 1; i >= 0; i-- {
		b.WriteByte(stack[i])
	}
	return b.String(), nil
}

// extractJSONFromCodeBlock 从 markdown 代码块中提取 JSON，使用括号计数处理嵌套。
func extractJSONFromCodeBlock(output string) string {
	// 找到所有代码块的开始位置
	start := strings.Index(output, "```")
	for start != -1 {
		blockStart := start + 3
		// 跳过 ```json / ```JSON 语言标记
		for blockStart < len(output) && output[blockStart] != '\n' {
			blockStart++
		}
		if blockStart < len(output) {
			blockStart++ // skip newline
		}

		// 找到代码块结束 ```
		blockEnd := strings.Index(output[blockStart:], "```")
		if blockEnd == -1 {
			break
		}
		blockEnd += blockStart

		blockContent := strings.TrimSpace(output[blockStart:blockEnd])

		// 尝试提取 JSON 对象
		jsonStart := strings.Index(blockContent, "{")
		if jsonStart != -1 {
			jsonEnd := findMatchingBrace(blockContent, jsonStart)
			if jsonEnd != -1 {
				return strings.TrimSpace(blockContent[jsonStart : jsonEnd+1])
			}
		}

		// 查找下一个代码块
		start = strings.Index(output[blockEnd+3:], "```")
		if start != -1 {
			start += blockEnd + 3
		}
	}
	return ""
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

// TruncString 截断字符串用于日志输出（按 rune 截断，不破坏 UTF-8）
func TruncString(s string, maxLen int) string {
	if len([]rune(s)) <= maxLen {
		return s
	}
	return string([]rune(s)[:maxLen]) + "..."
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
			ID             string                 `json:"id"`
			Name           string                 `json:"name"`
			Category       string                 `json:"category"`
			Description    string                 `json:"description"`
			GoodExample    string                 `json:"good_example"`
			BadExample     string                 `json:"bad_example"`
			Rule           string                 `json:"rule"`
			Confidence     float64                `json:"confidence"`
			Frequency      int                    `json:"frequency"`
			BusinessMethod *businessMethodPayload `json:"business_method"`
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

		pattern.BusinessMethod = p.BusinessMethod.toDomain(pattern.CreatedAt)

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
			ID             string                 `json:"id"`
			Name           string                 `json:"name"`
			Category       string                 `json:"category"`
			Description    string                 `json:"description"`
			GoodExample    string                 `json:"good_example"`
			BadExample     string                 `json:"bad_example"`
			Rule           string                 `json:"rule"`
			Confidence     float64                `json:"confidence"`
			Frequency      int                    `json:"frequency"`
			BusinessMethod *businessMethodPayload `json:"business_method"`
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
		pattern.BusinessMethod = p.BusinessMethod.toDomain(pattern.CreatedAt)
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
			Category        string                   `json:"category"`
			Summary         string                   `json:"summary"`
			Patterns        []string                 `json:"patterns"`
			UsageScenes     []string                 `json:"usage_scenes"`
			Priority        int                      `json:"priority"`
			BusinessMethods []*businessMethodPayload `json:"business_methods"`
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
			if method := bm.toDomain(time.Now()); method != nil {
				businessMethods = append(businessMethods, method)
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

// ParseCuratePatternsResult 解析模式策展结果。
func ParseCuratePatternsResult(output string) (*agent.CuratePatternsResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var result struct {
		Patterns []struct {
			ID              string                 `json:"id"`
			Name            string                 `json:"name"`
			Category        string                 `json:"category"`
			Description     string                 `json:"description"`
			GoodExample     string                 `json:"good_example"`
			BadExample      string                 `json:"bad_example"`
			Rule            string                 `json:"rule"`
			Confidence      float64                `json:"confidence"`
			Frequency       int                    `json:"frequency"`
			MergedFrom      []string               `json:"merged_from"`
			MergeReason     string                 `json:"merge_reason"`
			SimilarityScore float64                `json:"similarity_score"`
			Source          string                 `json:"source"`
			BusinessMethod  *businessMethodPayload `json:"business_method"`
			ProjectID       string                 `json:"project_id"`
			ScopePath       string                 `json:"scope_path"`
			WorkspaceRole   string                 `json:"workspace_role"`
		} `json:"patterns"`
		Dropped []struct {
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

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
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
		curateResult.Patterns[i] = agent.CuratedPattern{
			ID:              p.ID,
			Name:            p.Name,
			Category:        p.Category,
			Description:     p.Description,
			GoodExample:     p.GoodExample,
			BadExample:      p.BadExample,
			Rule:            p.Rule,
			Confidence:      p.Confidence,
			Frequency:       p.Frequency,
			MergedFrom:      p.MergedFrom,
			MergeReason:     p.MergeReason,
			SimilarityScore: p.SimilarityScore,
			Source:          p.Source,
			BusinessMethod:  p.BusinessMethod.toDomain(now),
			ProjectID:       p.ProjectID,
			ScopePath:       p.ScopePath,
			WorkspaceRole:   p.WorkspaceRole,
		}
	}
	for i, dropped := range result.Dropped {
		curateResult.Dropped[i] = agent.CuratedDrop{
			ID:     dropped.ID,
			Reason: dropped.Reason,
		}
	}

	return curateResult, nil
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
		ConfigPatterns     []string                   `json:"config_patterns"`
		Dependencies       []string                   `json:"dependencies"`
		BusinessMethods    []businessMethodPayload    `json:"business_methods"`
		ValidationCommands []domain.ValidationCommand `json:"validation_commands"`
		Summary            string                     `json:"summary"`
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
		ProjectName:        result.ProjectName,
		Language:           result.Language,
		Frameworks:         result.Frameworks,
		Architecture:       result.Architecture,
		Structure:          result.Structure,
		DependencyGraph:    result.DependencyGraph,
		DataFlow:           result.DataFlow,
		FrameworkPatterns:  result.FrameworkPatterns,
		ConfigPatterns:     result.ConfigPatterns,
		Dependencies:       result.Dependencies,
		ValidationCommands: result.ValidationCommands,
		Summary:            result.Summary,
		Layers:             make([]domain.ArchitectureLayer, len(result.Layers)),
		CommonUtils:        make([]domain.UtilityFunction, len(result.CommonUtils)),
		KeyModules:         make([]domain.ModuleInfo, len(result.KeyModules)),
		BusinessMethods:    make([]domain.BusinessMethod, len(result.BusinessMethods)),
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
		analyzeResult.BusinessMethods[i] = *method.toDomain(time.Now())
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
		)
		return nil, fmt.Errorf("%s", i18n.Get("AgentNoValidJSONFound"))
	}

	var result struct {
		Patterns []struct {
			ID             string                 `json:"id"`
			Name           string                 `json:"name"`
			Category       string                 `json:"category"`
			Description    string                 `json:"description"`
			GoodExample    string                 `json:"good_example"`
			BadExample     string                 `json:"bad_example"`
			Rule           string                 `json:"rule"`
			Confidence     float64                `json:"confidence"`
			Frequency      int                    `json:"frequency"`
			BusinessMethod *businessMethodPayload `json:"business_method"`
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
		pattern.BusinessMethod = p.BusinessMethod.toDomain(pattern.CreatedAt)
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

	var payload workspaceSpecPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}
	spec := payload.WorkspaceSpec
	spec.ChangeOrder = normalizeWorkspaceChangeOrder(payload.ChangeOrder)
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

func normalizeWorkspaceChangeOrder(items []json.RawMessage) []string {
	if len(items) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value := normalizeWorkspaceChangeOrderItem(item); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeWorkspaceChangeOrderItem(item json.RawMessage) string {
	var text string
	if err := json.Unmarshal(item, &text); err == nil {
		return strings.TrimSpace(text)
	}

	var object struct {
		Step    int    `json:"step"`
		Action  string `json:"action"`
		Details string `json:"details"`
	}
	if err := json.Unmarshal(item, &object); err != nil {
		return ""
	}
	action := strings.TrimSpace(object.Action)
	details := strings.TrimSpace(object.Details)
	if action == "" {
		return details
	}
	if object.Step > 0 {
		action = fmt.Sprintf("%d. %s", object.Step, action)
	}
	if details == "" {
		return action
	}
	return action + "：" + details
}

// ParseUserDefinePatternResult 解析用户自定义模式结果
func ParseUserDefinePatternResult(output string) (*agent.UserDefinePatternResult, error) {
	jsonStr, err := ExtractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentNoValidJSONFound"), err)
	}

	var result struct {
		ID             string                 `json:"id"`
		Name           string                 `json:"name"`
		Category       string                 `json:"category"`
		Description    string                 `json:"description"`
		GoodExample    string                 `json:"good_example"`
		BadExample     string                 `json:"bad_example"`
		Rule           string                 `json:"rule"`
		Confidence     float64                `json:"confidence"`
		BusinessMethod *businessMethodPayload `json:"business_method"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("AgentJSONUnmarshalSimpleFailed"), err)
	}

	pattern := domain.Pattern{
		ID:          result.ID,
		Name:        result.Name,
		Category:    domain.Category(result.Category),
		Description: result.Description,
		GoodExample: result.GoodExample,
		BadExample:  result.BadExample,
		Rule:        result.Rule,
		Confidence:  result.Confidence,
		Source:      domain.SourceUserDefined,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	pattern.BusinessMethod = result.BusinessMethod.toDomain(pattern.CreatedAt)

	return &agent.UserDefinePatternResult{Pattern: &pattern}, nil
}
