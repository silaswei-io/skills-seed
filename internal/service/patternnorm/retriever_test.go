package patternnorm

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRetrieveRelatedPatternsUsesVerifiedSourceRelations(t *testing.T) {
	candidate := domain.Pattern{
		ID: "candidate",
		EvidenceLocations: []domain.PatternEvidenceLocation{{
			Path: "internal/auth/login.go",
		}},
	}
	related := domain.Pattern{
		ID: "related",
		EvidenceLocations: []domain.PatternEvidenceLocation{{
			Path: "internal/auth/login.go",
		}},
	}
	textOnly := domain.Pattern{
		ID:          "text-only",
		Name:        "Authentication Flow",
		Description: "Authentication guidance.",
		EvidenceLocations: []domain.PatternEvidenceLocation{{
			Path: "internal/payment/pay.go",
		}},
	}

	result := retrieveRelatedPatterns([]domain.Pattern{candidate}, []domain.Pattern{related, textOnly})

	require.Equal(t, []string{related.ID}, result.existingByCandidate[candidate.ID])
	require.Equal(t, []domain.Pattern{related}, result.related)
}
