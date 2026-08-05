package patternnorm

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type patternStore interface {
	GetAll(context.Context) ([]domain.Pattern, error)
	GetByCategory(context.Context, domain.Category) ([]domain.Pattern, error)
	ApplyPatternMutation(context.Context, domain.PatternMutation) error
}
