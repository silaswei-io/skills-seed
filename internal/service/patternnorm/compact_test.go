package patternnorm

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCompactDryRunDoesNotWrite(t *testing.T) {
	p1 := newPatternNormTestPattern("p1", "Error Wrap", domain.CategoryError)
	p1.Confidence = 0.8
	p1.SetRule("wrap errors")
	p2 := newPatternNormTestPattern("p2", "Error Wrap", domain.CategoryError)
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
	svc := NewService(repo)

	result, err := svc.Compact(context.Background(), CompactRequest{DryRun: true})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, "p2", result.Written[0].ID)
	require.Equal(t, 2, result.Summary.TotalCandidates)
	require.Equal(t, 2, result.Summary.TotalExisting)
	require.Equal(t, 1, result.Summary.TotalWritten)
}

func TestCompactSinglePatternDoesNotSelfMerge(t *testing.T) {
	p1 := newPatternNormTestPattern("p1", "Error Wrap", domain.CategoryError)
	p1.Confidence = 0.8
	p1.Frequency = 3
	p1.SetRule("wrap errors with context")

	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*p1}, nil
		},
	}
	svc := NewService(repo)

	result, err := svc.Compact(context.Background(), CompactRequest{DryRun: true})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, "p1", result.Written[0].ID)
	require.False(t, result.Written[0].Merged)
	require.Equal(t, []string{"p1"}, result.Written[0].MergedFrom)
	require.Equal(t, 3, result.Written[0].Frequency)
}

func TestCompactNormalizesRequestedCategory(t *testing.T) {
	p1 := newPatternNormTestPattern("p1", "Utility Path Guard", domain.CategoryUtils)
	p1.Confidence = 0.8
	p1.SetRule("reject unsafe paths")

	var requested domain.Category
	repo := &mocks.MockPatternRepository{
		GetByCategoryFn: func(ctx context.Context, category domain.Category) ([]domain.Pattern, error) {
			requested = category
			return []domain.Pattern{*p1}, nil
		},
	}
	svc := NewService(repo)

	result, err := svc.Compact(context.Background(), CompactRequest{Category: " Security ", DryRun: true})

	require.NoError(t, err)
	require.Equal(t, domain.CategoryUtils, requested)
	require.Len(t, result.Written, 1)
	require.Equal(t, domain.CategoryUtils, result.Written[0].Category)
}
