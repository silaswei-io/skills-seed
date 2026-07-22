package curator

import (
	"context"
	"errors"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCompactDryRunDoesNotWrite(t *testing.T) {
	p1 := newCuratorTestPattern("p1", "Error Wrap", domain.CategoryError)
	p1.Confidence = 0.8
	p1.SetRule("wrap errors")
	p2 := newCuratorTestPattern("p2", "Error Wrap", domain.CategoryError)
	p2.Confidence = 0.9
	p2.SetRule("wrap errors")

	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*p1, *p2}, nil
		},
		DeleteFn: func(ctx context.Context, id string) error {
			return errors.New("should not delete")
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			return errors.New("should not save")
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			require.Fail(t, "compact should not call AI unless UseAI is true")
			return nil, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.Compact(context.Background(), CompactRequest{DryRun: true})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, "p2", result.Written[0].ID)
	require.Equal(t, 2, result.Summary.TotalCandidates)
	require.Equal(t, 2, result.Summary.TotalExisting)
	require.Equal(t, 1, result.Summary.TotalWritten)
}

func TestCompactSinglePatternDoesNotSelfMerge(t *testing.T) {
	p1 := newCuratorTestPattern("p1", "Error Wrap", domain.CategoryError)
	p1.Confidence = 0.8
	p1.Frequency = 3
	p1.SetRule("wrap errors with context")

	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*p1}, nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			require.Fail(t, "compact should not call AI unless UseAI is true")
			return nil, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.Compact(context.Background(), CompactRequest{DryRun: true})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, "p1", result.Written[0].ID)
	require.False(t, result.Written[0].Merged)
	require.Equal(t, []string{"p1"}, result.Written[0].MergedFrom)
	require.Equal(t, 3, result.Written[0].Frequency)
}

func TestCompactUsesAIWhenRequested(t *testing.T) {
	p1 := newCuratorTestPattern("p1", "Error Wrap", domain.CategoryError)
	p1.Confidence = 0.8
	p1.SetRule("wrap errors")

	var called bool
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*p1}, nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			called = true
			require.Equal(t, string(OperationCompact), req.Operation)
			require.True(t, req.AllExisting)
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{
						ID:         "p1",
						Name:       "Error Wrap",
						Category:   string(domain.CategoryError),
						Rule:       "wrap errors",
						Confidence: 0.8,
						SourceIDs:  []string{"p1"},
					},
				},
			}, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.Compact(context.Background(), CompactRequest{DryRun: true, UseAI: true})

	require.NoError(t, err)
	require.True(t, called)
	require.Len(t, result.Written, 1)
}

func TestCompactAtomicallyDeletesDroppedExistingPatterns(t *testing.T) {
	keep := newCuratorTestPattern("keep", "Keep", domain.CategoryError)
	drop := newCuratorTestPattern("drop", "Drop", domain.CategoryError)
	var mutation domain.PatternMutation
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*keep, *drop}, nil
		},
		ApplyPatternMutationFn: func(ctx context.Context, got domain.PatternMutation) error {
			mutation = got
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{{
					ID:        keep.ID,
					Name:      keep.Name,
					Category:  string(keep.Category),
					Rule:      keep.Rule,
					SourceIDs: []string{keep.ID},
				}},
				Dropped: []agent.CuratedDrop{{ID: drop.ID, Reason: "noise"}},
			}, nil
		},
	}

	result, err := NewService(mockAgent, repo).Compact(context.Background(), CompactRequest{UseAI: true})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, []string{drop.ID}, mutation.DeleteIDs)
	require.Len(t, mutation.Save, 1)
}

func TestCompactDoesNotFallbackWhenAICurationIsInvalid(t *testing.T) {
	pattern := newCuratorTestPattern("p1", "Pattern", domain.CategoryError)
	var mutated bool
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return []domain.Pattern{*pattern}, nil },
		ApplyPatternMutationFn: func(ctx context.Context, mutation domain.PatternMutation) error {
			mutated = true
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{{
				ID:        pattern.ID,
				Name:      pattern.Name,
				Category:  "invalid-category",
				Rule:      pattern.Rule,
				SourceIDs: []string{pattern.ID},
			}}}, nil
		},
	}

	result, err := NewService(mockAgent, repo).Compact(context.Background(), CompactRequest{UseAI: true})

	require.ErrorContains(t, err, "validate compact curation")
	require.Nil(t, result)
	require.False(t, mutated)
}

func TestCompactNormalizesRequestedCategory(t *testing.T) {
	p1 := newCuratorTestPattern("p1", "Utility Path Guard", domain.CategoryUtils)
	p1.Confidence = 0.8
	p1.SetRule("reject unsafe paths")

	var requested domain.Category
	repo := &mocks.MockPatternRepository{
		GetByCategoryFn: func(ctx context.Context, category domain.Category) ([]domain.Pattern, error) {
			requested = category
			return []domain.Pattern{*p1}, nil
		},
	}
	svc := NewService(nil, repo)

	result, err := svc.Compact(context.Background(), CompactRequest{Category: " Security ", DryRun: true})

	require.NoError(t, err)
	require.Equal(t, domain.CategoryUtils, requested)
	require.Len(t, result.Written, 1)
	require.Equal(t, domain.CategoryUtils, result.Written[0].Category)
}
