package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
)

func businessPatternGroups(locale string, patterns []domain.Pattern) []patternGroup {
	routed := knowledge.BusinessPatternGroups(locale, patterns)
	groups := make([]patternGroup, 0, len(routed))
	for _, group := range routed {
		groups = append(groups, patternGroup{
			BusinessGroup: group,
			Patterns:      patternsForTemplate(group.Patterns),
		})
	}
	return groups
}

func businessCoverageWarnings(groups []patternGroup, locale string) []CoverageWarning {
	warnings := make([]CoverageWarning, 0)
	for _, group := range groups {
		if len(group.Patterns) != 1 {
			continue
		}
		warnings = append(warnings, CoverageWarning{
			Title:   group.Title,
			Path:    group.Path,
			Message: generatorText(locale, "GeneratorBusinessCoverageSinglePatternWarning"),
		})
	}
	return warnings
}
