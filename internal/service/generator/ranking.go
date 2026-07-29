package generator

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func (s *GeneratorService) patternGenerationInsights(_ context.Context, patterns []domain.Pattern) (map[string]domain.PatternInsight, error) {
	insights := make(map[string]domain.PatternInsight, len(patterns))
	for _, pattern := range patterns {
		insights[pattern.ID] = domain.PatternInsight{}
	}
	return insights, nil
}

func rankPatternsForGeneration(patterns []domain.Pattern, insights map[string]domain.PatternInsight) []domain.Pattern {
	patterns = activePatterns(patterns)
	return domain.RankPatternsForGeneration(patterns, insights)
}

func activePatterns(patterns []domain.Pattern) []domain.Pattern {
	out := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.IsActive() {
			out = append(out, pattern)
		}
	}
	return out
}
