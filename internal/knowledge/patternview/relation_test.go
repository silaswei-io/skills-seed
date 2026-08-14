package patternview

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRelateUsesEvidencePathAndScope(t *testing.T) {
	left := domain.Pattern{
		ProjectID: "project",
		ScopePath: "internal/auth",
		EvidenceLocations: []domain.PatternEvidenceLocation{{
			Path: "internal/auth/login.go",
		}},
	}
	right := domain.Pattern{
		ProjectID: "project",
		ScopePath: "internal/auth",
		EvidenceLocations: []domain.PatternEvidenceLocation{{
			Path: "internal/auth/session.go",
		}},
	}

	relation := Relate(left, right)

	require.Equal(t, []RelationKind{RelationScope}, relation.Kinds)
	require.True(t, RelatedToPaths(left, []string{"internal/auth"}))
}

func TestRelateDoesNotTreatTextSimilarityAsSourceRelation(t *testing.T) {
	left := domain.Pattern{
		Name: "Authentication Flow",
		Rule: "Inspect authentication flow.",
		EvidenceLocations: []domain.PatternEvidenceLocation{{
			Path: "internal/auth/login.go",
		}},
	}
	right := domain.Pattern{
		Name: "Authentication Flow",
		Rule: "Inspect authentication flow.",
		EvidenceLocations: []domain.PatternEvidenceLocation{{
			Path: "internal/payment/pay.go",
		}},
	}

	require.False(t, Relate(left, right).Exists())
}
