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

func splitBusinessPatternGroups(groups []patternGroup) (detailGroups []patternGroup, inlineGroups []patternGroup) {
	for _, group := range groups {
		if businessGroupNeedsDetail(group) {
			detailGroups = append(detailGroups, group)
			continue
		}
		inlineGroups = append(inlineGroups, group)
	}
	return detailGroups, inlineGroups
}

func businessGroupNeedsDetail(group patternGroup) bool {
	return len(group.Patterns) > 1
}
