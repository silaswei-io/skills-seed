package generator

import (
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge"
)

func validationMatrix(profile *domain.ProjectProfile, patterns []domain.Pattern, locale string) []ValidationMatrixItem {
	recommendations := knowledge.ValidationMatrix(profile, patterns, locale)
	matrix := make([]ValidationMatrixItem, 0, len(recommendations))
	for _, recommendation := range recommendations {
		matrix = append(matrix, ValidationMatrixItem{
			Area:     recommendation.Area,
			Command:  recommendation.Command,
			When:     recommendation.When,
			Source:   recommendation.Source,
			Evidence: recommendation.Evidence,
		})
	}
	return matrix
}
