package learner

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestNewLearnerService(t *testing.T) {
	svc := NewLearnerService(curator.NewService(&mocks.MockPatternRepository{}))

	require.NotNil(t, svc)
}

func TestCurateAndSavePatternsReturnsErrorWithoutCurator(t *testing.T) {
	svc := NewLearnerService(nil)

	count, err := svc.CurateAndSavePatterns(context.Background(), []domain.Pattern{
		*newLearnerTestPattern("candidate", "Error Handling", domain.CategoryError),
	}, curator.OperationLearnCurrent)

	require.Error(t, err)
	require.Zero(t, count)
	require.Contains(t, err.Error(), "pattern curator is not configured")
}

func TestCurateAndSavePatternsWithOptionsUsesLocalCuration(t *testing.T) {
	var saved []string
	mockPattern := &mocks.MockPatternRepository{
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p.ID)
			return nil
		},
	}
	svc := NewLearnerService(curator.NewService(mockPattern))

	count, err := svc.CurateAndSavePatterns(context.Background(), []domain.Pattern{
		*newLearnerTestPattern("candidate", "Error Handling", domain.CategoryError),
	}, curator.OperationLearnCurrent)

	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, []string{"candidate"}, saved)
}

func TestCurateAndSavePatternsReturnsStoreError(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			return errors.New("db closed")
		},
	}
	svc := NewLearnerService(curator.NewService(mockPattern))

	count, err := svc.CurateAndSavePatterns(context.Background(), []domain.Pattern{
		*newLearnerTestPattern("candidate", "Error Handling", domain.CategoryError),
	}, curator.OperationLearnCurrent)

	require.Error(t, err)
	require.Zero(t, count)
	require.Contains(t, err.Error(), "db closed")
}

func newLearnerTestPattern(id, name string, category domain.Category) *domain.Pattern {
	pattern := domain.NewPattern(id, name, category)
	pattern.Description = "Source-backed " + name + " pattern."
	pattern.Rule = "Preserve the project-specific " + name + " rule."
	pattern.Confidence = 0.9
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/example.go", Line: 1, Kind: "file"}}
	return pattern
}
