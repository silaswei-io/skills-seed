package curator

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

type curationAgent interface {
	CuratePatterns(context.Context, *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error)
}

type patternStore interface {
	GetAll(context.Context) ([]domain.Pattern, error)
	GetByCategory(context.Context, domain.Category) ([]domain.Pattern, error)
	ApplyPatternMutation(context.Context, domain.PatternMutation) error
}
