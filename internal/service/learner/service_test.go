package learner

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
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

func TestCurateAndSavePatternsWithOptionsUsesLearningSession(t *testing.T) {
	var curatedViaSession bool
	var saved []string
	mockAgent := &mocks.MockAgent{
		NameVal:      "test",
		AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			curatedViaSession = true
			require.Equal(t, string(curator.OperationLearnCurrent), req.Operation)
			require.Len(t, req.CandidatePatterns, 1)
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{{
					ID:          req.CandidatePatterns[0].ID,
					Name:        req.CandidatePatterns[0].Name,
					Category:    string(req.CandidatePatterns[0].Category),
					Description: req.CandidatePatterns[0].Description,
					Rule:        req.CandidatePatterns[0].Rule,
					Confidence:  req.CandidatePatterns[0].Confidence,
					SourceIDs:   []string{req.CandidatePatterns[0].ID},
				}},
			}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p.ID)
			return nil
		},
	}
	session, err := mockAgent.StartLearningSession(context.Background(), agent.LearningSessionRequest{})
	require.NoError(t, err)
	svc := NewLearnerService(curator.NewService(mockPattern))

	count, err := svc.CurateAndSavePatternsWithOptions(context.Background(), []domain.Pattern{
		*newLearnerTestPattern("candidate", "Error Handling", domain.CategoryError),
	}, curator.OperationLearnCurrent, CurateOptions{LearningSession: session})

	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.True(t, curatedViaSession)
	require.Equal(t, []string{"candidate"}, saved)
}

func TestCurateAndSavePatternsRequiresSessionForCurrentLearning(t *testing.T) {
	svc := NewLearnerService(curator.NewService(&mocks.MockPatternRepository{}))

	count, err := svc.CurateAndSavePatterns(context.Background(), []domain.Pattern{
		*newLearnerTestPattern("candidate", "Error Handling", domain.CategoryError),
	}, curator.OperationLearnCurrent)

	require.Error(t, err)
	require.Zero(t, count)
	require.Contains(t, err.Error(), i18n.Get("CuratorLearningSessionRequiredForCurrent"))
}

func TestCurateAndSavePatternsReturnsStoreError(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockPattern := &mocks.MockPatternRepository{
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			return errors.New("db closed")
		},
	}
	session, err := mockAgent.StartLearningSession(context.Background(), agent.LearningSessionRequest{})
	require.NoError(t, err)
	svc := NewLearnerService(curator.NewService(mockPattern))

	count, err := svc.CurateAndSavePatternsWithOptions(context.Background(), []domain.Pattern{
		*newLearnerTestPattern("candidate", "Error Handling", domain.CategoryError),
	}, curator.OperationLearnCurrent, CurateOptions{LearningSession: session})

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
