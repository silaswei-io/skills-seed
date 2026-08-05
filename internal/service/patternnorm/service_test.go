package patternnorm

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndStoreAddsNewCandidate(t *testing.T) {
	candidate := newPatternNormTestPattern("p1", "Error Wrapping", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("When returning repository errors, wrap them with operation context")
	candidate.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/user.go", Line: 42, Symbol: "LoadUser", Kind: "function", Description: "wraps repository errors", Confidence: 0.86},
	}

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return nil, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}
	svc := NewService(repo)

	result, err := svc.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, "p1", saved[0].ID)
	require.Equal(t, "internal/service/user.go:42", saved[0].EvidenceLocations[0].DisplayLocation())
}

func newPatternNormTestPattern(id, name string, category domain.Category) *domain.Pattern {
	pattern := domain.NewPattern(id, name, category)
	pattern.Rule = "Preserve the project-specific " + name + " rule."
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: id + ".go", Line: 1, Kind: "file"}}
	return pattern
}
