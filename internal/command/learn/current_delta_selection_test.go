package learn

import (
	"fmt"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSelectRelatedDeltaPatternsKeepsAllDirectEvidenceMatches(t *testing.T) {
	patterns := make([]domain.Pattern, 0, 9)
	for i := 0; i < 9; i++ {
		pattern := domain.NewPattern(fmt.Sprintf("pattern-%02d", i), fmt.Sprintf("Pattern %d", i), domain.CategoryBusiness)
		pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/run.go", Kind: "file"}}
		patterns = append(patterns, *pattern)
	}

	selected := selectRelatedDeltaPatterns(domain.EvidenceFocus{}, []string{"internal/service/run.go"}, patterns)

	require.Len(t, selected, len(patterns))
}

func TestSelectRelatedDeltaPatternsDropsTextOnlyMatches(t *testing.T) {
	patterns := make([]domain.Pattern, 0, 9)
	for i := 0; i < 9; i++ {
		pattern := domain.NewPattern(fmt.Sprintf("fuzzy-%02d", i), fmt.Sprintf("Authentication Flow %d", i), domain.CategoryBusiness)
		pattern.Description = "Authentication flow guidance."
		pattern.Rule = "Inspect the authentication flow."
		patterns = append(patterns, *pattern)
	}

	selected := selectRelatedDeltaPatterns(domain.EvidenceFocus{Name: "Authentication Flow"}, []string{"internal/service/run.go"}, patterns)

	require.Empty(t, selected)
}
