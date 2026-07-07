package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/routing"
)

type patternGroup = routing.BusinessGroup

func businessPatternGroups(locale string, patterns []domain.Pattern) []patternGroup {
	groups := routing.BusinessPatternGroups(locale, patterns)
	for i := range groups {
		groups[i].Patterns = patternsForTemplate(groups[i].Patterns, locale)
	}
	return groups
}

func businessCoverageWarnings(groups []patternGroup, locale string) []CoverageWarning {
	warnings := make([]CoverageWarning, 0)
	for _, group := range groups {
		if len(group.Patterns) != 1 {
			continue
		}
		message := "该子域当前只有 1 个已学习模式，覆盖可能不完整；命中时先按证据路径复核当前代码，不要推断为完整规则集。"
		if len(locale) >= 2 && locale[:2] == "en" {
			message = "This domain currently has only 1 learned pattern. Treat it as incomplete coverage; verify current code through the evidence path before generalizing."
		}
		warnings = append(warnings, CoverageWarning{
			Title:   group.Title,
			Path:    group.Path,
			Message: message,
		})
	}
	return warnings
}
