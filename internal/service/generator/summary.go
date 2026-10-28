package generator

import "github.com/silaswei-io/skills-seed/internal/domain"

func (s *GeneratorService) buildDeterministicSummary(patterns []domain.Pattern) generationSummary {
	return generationSummary{
		CategorySummaries: s.ensureCategorySummaries(patterns, nil),
	}
}
