package patternnorm

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/patternview"
)

func deterministicNormalize(candidates, existing []domain.Pattern) *proposal {
	return &proposal{
		Patterns: patternview.Compact(candidates, existing),
		Dropped:  []Drop{},
	}
}
