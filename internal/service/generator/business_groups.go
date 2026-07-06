package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/routing"
)

type patternGroup = routing.BusinessGroup

func businessPatternGroups(locale string, patterns []domain.Pattern) []patternGroup {
	return routing.BusinessPatternGroups(locale, patterns)
}
