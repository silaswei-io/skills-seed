package generator

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func (s *GeneratorService) buildDeterministicSummary(patterns []domain.Pattern, insights map[string]domain.PatternInsight) generationSummary {
	locale := ""
	if s.skillsLoader != nil {
		locale = s.skillsLoader.GetLocale()
	}
	return generationSummary{
		CategorySummaries: s.ensureCategorySummaries(patterns, nil),
		KeyInsights:       deterministicKeyInsights(patterns, insights, locale),
	}
}

func deterministicKeyInsights(patterns []domain.Pattern, insights map[string]domain.PatternInsight, locale string) []string {
	if len(patterns) == 0 {
		return nil
	}
	categories := domain.CategoryNamesWithPatterns(patterns)
	sort.Strings(categories)
	out := []string{
		generatorText(locale, "GeneratorInsightDeterministicGeneration"),
	}
	if len(categories) > 0 {
		out = append(out, generatorTextWithParams(locale, "GeneratorInsightCoveredCategories", map[string]interface{}{
			"Categories": strings.Join(categories, ", "),
		}))
	}
	return out
}
