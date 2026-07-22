package curator

import (
	"context"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCurateAndStoreAddsNewCandidate(t *testing.T) {
	candidate := newCuratorTestPattern("p1", "Error Wrapping", domain.CategoryError)
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
	svc := NewService(&mocks.MockAgent{NameVal: "mock", AvailableVal: true}, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, "p1", saved[0].ID)
	require.Equal(t, "internal/service/user.go:42", saved[0].EvidenceLocations[0].DisplayLocation())
}

func newCuratorTestPattern(id, name string, category domain.Category) *domain.Pattern {
	pattern := domain.NewPattern(id, name, category)
	pattern.Rule = "Preserve the project-specific " + name + " rule."
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: id + ".go", Line: 1, Kind: "file"}}
	return pattern
}

func newDeterministicCuratorAgent() *mocks.MockAgent {
	return &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			planned := deterministicCurate(req.CandidatePatterns, req.ExistingPatterns)
			result := &agent.CuratePatternsResult{
				Patterns: make([]agent.CuratedPattern, 0, len(planned.Patterns)),
				Dropped:  make([]agent.CuratedDrop, 0, len(planned.Dropped)),
			}
			for _, pattern := range planned.Patterns {
				result.Patterns = append(result.Patterns, agent.CuratedPattern{
					ID:          pattern.ID,
					Name:        pattern.Name,
					Category:    string(pattern.Category),
					Description: pattern.Description,
					Rule:        pattern.Rule,
					Confidence:  pattern.Confidence,
					SourceIDs:   pattern.MergedFrom,
				})
			}
			for _, item := range planned.Dropped {
				result.Dropped = append(result.Dropped, agent.CuratedDrop{ID: item.ID, Reason: item.Reason})
			}
			return result, nil
		},
	}
}
