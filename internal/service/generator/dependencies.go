package generator

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type patternReader interface {
	GetAll(context.Context) ([]domain.Pattern, error)
}

type patternStatsReader interface {
	GetPatternHitStats(context.Context) ([]domain.PatternHitStats, error)
}

type profileReader interface {
	Get(context.Context) (*domain.ProjectProfile, error)
}

type projectSpecWriter interface {
	SaveSpec(context.Context, *domain.ProjectSpec) error
}
