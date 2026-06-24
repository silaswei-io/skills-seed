package generator

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

func (s *GeneratorService) buildDeterministicSummary(patterns []domain.Pattern, insights map[string]domain.PatternInsight) *agent.GenerateSkillsResult {
	locale := ""
	if s.skillsLoader != nil {
		locale = s.skillsLoader.GetLocale()
	}
	result := &agent.GenerateSkillsResult{
		CategorySummaries: map[string]agent.CategorySummary{},
		KeyPatterns:       deterministicKeyPatterns(patterns, insights),
		BusinessRules:     deterministicRulesForCategory(patterns, domain.CategoryBusiness),
		BestPractices:     deterministicBestPractices(patterns),
		CommonPatterns:    deterministicCommonPatterns(patterns),
		KeyInsights:       deterministicKeyInsights(patterns, insights, locale),
	}
	result.CategorySummaries = s.ensureCategorySummaries(patterns, result.CategorySummaries)
	return result
}

func deterministicKeyPatterns(patterns []domain.Pattern, insights map[string]domain.PatternInsight) []agent.PatternSummary {
	limit := len(patterns)
	if limit > 8 {
		limit = 8
	}
	out := make([]agent.PatternSummary, 0, limit)
	for _, pattern := range patterns {
		if len(out) >= limit {
			break
		}
		name := strings.TrimSpace(pattern.Name)
		if name == "" {
			continue
		}
		insight := insights[pattern.ID]
		out = append(out, agent.PatternSummary{
			Name:       name,
			Category:   string(pattern.Category),
			Importance: patternImportance(pattern, insight),
			Summary:    firstNonEmptyString(pattern.Description, pattern.Rule),
			WhenToUse:  firstNonEmptyString(pattern.Rule, pattern.Description),
		})
	}
	return out
}

func deterministicRulesForCategory(patterns []domain.Pattern, category domain.Category) []string {
	out := make([]string, 0)
	for _, pattern := range patterns {
		if pattern.Category != category {
			continue
		}
		rule := strings.TrimSpace(pattern.Rule)
		if rule == "" {
			rule = strings.TrimSpace(pattern.Description)
		}
		if rule != "" {
			out = append(out, rule)
		}
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func deterministicBestPractices(patterns []domain.Pattern) []string {
	out := make([]string, 0)
	for _, pattern := range patterns {
		if pattern.Category == domain.CategoryBusiness {
			continue
		}
		rule := strings.TrimSpace(pattern.Rule)
		if rule == "" {
			rule = strings.TrimSpace(pattern.Description)
		}
		if rule != "" {
			out = append(out, rule)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func deterministicCommonPatterns(patterns []domain.Pattern) []string {
	names := domain.PatternNames(patterns)
	if len(names) > 10 {
		names = names[:10]
	}
	return names
}

func deterministicKeyInsights(patterns []domain.Pattern, insights map[string]domain.PatternInsight, locale string) []string {
	if len(patterns) == 0 {
		return nil
	}
	categories := domain.CategoryNamesWithPatterns(patterns)
	sort.Strings(categories)
	out := []string{
		localizedText(locale, "skills 由已入库模式、项目画像和工作流确定性生成；生成阶段不调用 AI。", "Skills are deterministically generated from stored patterns, project profile, and workflows; generation does not call AI."),
	}
	if len(categories) > 0 {
		out = append(out, localizedText(locale, "已覆盖模式分类：", "Covered pattern categories: ")+strings.Join(categories, ", "))
	}
	hitCount := 0
	for _, insight := range insights {
		hitCount += insight.HitCount
	}
	if hitCount > 0 {
		out = append(out, localizedText(locale, "已结合 check 命中统计排序。", "Pattern ordering includes check hit statistics."))
	}
	return out
}

func patternImportance(pattern domain.Pattern, insight domain.PatternInsight) string {
	if pattern.Metrics.EffectiveScore >= 0.75 || pattern.Confidence >= 0.85 || insight.HitCount > 0 {
		return "high"
	}
	if pattern.Metrics.EffectiveScore >= 0.45 || pattern.Confidence >= 0.65 {
		return "medium"
	}
	return "low"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
