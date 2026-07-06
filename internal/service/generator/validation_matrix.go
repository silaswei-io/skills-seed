package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/validation"
)

func validationMatrix(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []ValidationMatrixItem {
	recommendations := validation.Matrix(profile, patterns, locale)
	matrix := make([]ValidationMatrixItem, 0, len(recommendations))
	for _, recommendation := range recommendations {
		matrix = append(matrix, ValidationMatrixItem{
			Area:        recommendation.Area,
			Command:     recommendation.Command,
			When:        recommendation.When,
			Source:      recommendation.Source,
			Evidence:    recommendation.Evidence,
			Confidence:  recommendation.Confidence,
			Coverage:    recommendation.Coverage,
			MatchKind:   recommendation.MatchKind,
			Recommended: recommendation.Recommended,
			Warning:     recommendation.Warning,
		})
	}
	return matrix
}

func validationCommandPaths(command string) []string {
	return validation.CommandPaths(command)
}
