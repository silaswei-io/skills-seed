package patternnorm

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndStoreKeepsCurrentCandidates(t *testing.T) {
	first := currentPattern("api-contract", 0.9, "internal/api/user.go")
	second := currentPattern("order-flow", 0.9, "internal/service/order.go")

	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{first, second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Empty(t, result.Dropped)
	require.Equal(t, []string{first.ID}, result.Written[0].MergedFrom)
	require.Equal(t, []string{second.ID}, result.Written[1].MergedFrom)
}

func TestNormalizeAndStoreFiltersCurrentCandidatesByEvidenceAndConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		paths      []string
		kept       bool
	}{
		{name: "multiple evidence below threshold", confidence: 0.74, paths: []string{"a.go", "b.go"}},
		{name: "multiple evidence at threshold", confidence: 0.75, paths: []string{"a.go", "b.go"}, kept: true},
		{name: "single evidence below threshold", confidence: 0.84, paths: []string{"a.go"}},
		{name: "single evidence at threshold", confidence: 0.85, paths: []string{"a.go"}, kept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := currentPattern("candidate", tt.confidence, tt.paths...)
			result, err := NewService(&mocks.MockPatternRepository{
				GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
			}).NormalizeAndStore(context.Background(), NormalizeRequest{
				Operation:  OperationLearnCurrent,
				Candidates: []domain.Pattern{candidate},
			})

			require.NoError(t, err)
			if tt.kept {
				require.Len(t, result.Written, 1)
				return
			}
			require.Empty(t, result.Written)
		})
	}
}

func TestNormalizeAndStoreUsesConfiguredAdmissionPolicy(t *testing.T) {
	candidate := currentPattern("candidate", 0.7, "candidate.go")
	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}, AdmissionPolicy{MinConfidence: 0.65, MinSingleEvidenceConfidence: 0.7}).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
}

func TestNormalizeAndStoreCoalescesCurrentCandidatesByID(t *testing.T) {
	first := currentPattern("shared-rule", 0.9, "a.go")
	second := currentPattern("shared-rule", 0.9, "b.go")

	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{first, second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), result.Written[0].EvidenceLocations)
}

func TestNormalizeAndStoreUsesAIToMergeCandidateWithRelatedPattern(t *testing.T) {
	existing := currentPattern("existing-error-wrap", 0.9, "error.go")
	existing.Source = domain.SourceLearnedCurrent
	candidate := currentPattern("candidate-error-wrap", 0.9, "error.go")

	service := NewServiceWithNormalizer(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return []domain.Pattern{existing}, nil },
	}, normalizePatternsFunc(func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
		return &agent.NormalizePatternsResult{Patterns: []agent.PatternNormalization{{
			ID:          existing.ID,
			Name:        existing.Name,
			Category:    string(existing.Category),
			Description: existing.Description,
			Rule:        existing.Rule,
			Confidence:  0.9,
			SourceIDs:   []string{existing.ID, candidate.ID},
		}}}, nil
	}))

	result, err := service.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, existing.ID, result.Written[0].ID)
	require.ElementsMatch(t, []string{existing.ID, candidate.ID}, result.Written[0].MergedFrom)
}

func TestNormalizeAndStoreRejectsMergeAcrossDistinctCapabilityEntries(t *testing.T) {
	submit := currentPattern("dispatcher-submit", 0.9, "internal/job/dispatcher.go")
	submit.BusinessMethod = &domain.BusinessMethod{
		Name:         "Submit",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/job/dispatcher.go:41"},
	}
	closeDispatcher := currentPattern("dispatcher-close", 0.9, "internal/job/dispatcher.go")
	closeDispatcher.BusinessMethod = &domain.BusinessMethod{
		Name:         "Close",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/job/dispatcher.go:88"},
	}
	service := NewServiceWithNormalizer(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}, normalizePatternsFunc(func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
		return mergedNormalization("dispatcher-lifecycle", submit, closeDispatcher), nil
	}))

	result, err := service.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{submit, closeDispatcher},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.ElementsMatch(t, []string{submit.ID, closeDispatcher.ID}, patternIDs(result.Written))
}

func TestNormalizeAndStoreAllowsMergeForSameCapabilityEntry(t *testing.T) {
	first := currentPattern("submit-behavior", 0.9, "internal/job/dispatcher.go")
	first.BusinessMethod = &domain.BusinessMethod{
		Name:         "Submit",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/job/dispatcher.go:41"},
	}
	second := currentPattern("submit-contract", 0.9, "internal/job/dispatcher.go")
	second.BusinessMethod = &domain.BusinessMethod{
		Name:         "Submit",
		CodeLocation: domain.CodeLocation{HistoricalLocation: "internal/job/dispatcher.go:41"},
	}
	service := NewServiceWithNormalizer(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}, normalizePatternsFunc(func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
		return mergedNormalization(first.ID, first, second), nil
	}))

	result, err := service.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{first, second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, "Submit", result.Written[0].BusinessMethod.Name)
	require.ElementsMatch(t, []string{first.ID, second.ID}, result.Written[0].MergedFrom)
}

func TestNormalizeAndStoreAllowsMergeWithoutCapabilityEntries(t *testing.T) {
	first := currentPattern("retry-observation", 0.9, "internal/worker/retry.go")
	second := currentPattern("retry-boundary", 0.9, "internal/worker/retry.go")
	service := NewServiceWithNormalizer(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}, normalizePatternsFunc(func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
		return mergedNormalization(first.ID, first, second), nil
	}))

	result, err := service.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{first, second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Nil(t, result.Written[0].BusinessMethod)
	require.ElementsMatch(t, []string{first.ID, second.ID}, result.Written[0].MergedFrom)
}

func TestNormalizeAndStoreRepairsInvalidAIOwnershipLocally(t *testing.T) {
	first := currentPattern("first", 0.9, "first.go")
	second := currentPattern("second", 0.9, "second.go")
	service := NewServiceWithNormalizer(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}, normalizePatternsFunc(func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
		return &agent.NormalizePatternsResult{Patterns: []agent.PatternNormalization{{
			ID:          first.ID,
			Name:        first.Name,
			Category:    string(first.Category),
			Description: first.Description,
			Rule:        first.Rule,
			Confidence:  first.Confidence,
			SourceIDs:   []string{first.ID, "invented", first.ID},
		}}}, nil
	}))

	result, err := service.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{first, second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.ElementsMatch(t, []string{first.ID, second.ID}, patternIDs(result.Written))
}

func TestNormalizeAndStoreDeletesEligibleRetiredCurrentPattern(t *testing.T) {
	learned := currentPattern("legacy-flow", 0.9, "legacy.go")
	learned.Source = domain.SourceLearnedCurrent
	userDefined := currentPattern("explicit-policy", 0.9, "policy.go")
	userDefined.Source = domain.SourceUserDefined
	var mutation domain.PatternMutation

	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return []domain.Pattern{learned, userDefined}, nil },
		ApplyPatternMutationFn: func(_ context.Context, input domain.PatternMutation) error {
			mutation = input
			return nil
		},
	}).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:         OperationLearnCurrent,
		RetiredPatternIDs: []string{"legacy-flow", "explicit-policy", "legacy-flow", "unknown"},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"legacy-flow"}, result.RetiredPatternIDs)
	require.Equal(t, []string{"legacy-flow"}, mutation.DeleteIDs)
}

func TestNormalizeAndStoreReportsStoreFailure(t *testing.T) {
	candidate := currentPattern("candidate", 0.9, "repository.go")
	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn:   func(context.Context, *domain.Pattern) error { return errors.New("db closed") },
	}).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{candidate},
	})

	require.ErrorContains(t, err, i18n.Get("PatternNormApplyPatternsFailed"))
	require.Nil(t, result)
}

func currentPattern(id string, confidence float64, paths ...string) domain.Pattern {
	pattern := newPatternNormTestPattern(id, "Current Pattern", domain.CategoryBusiness)
	pattern.Confidence = confidence
	pattern.EvidenceLocations = make([]domain.PatternEvidenceLocation, 0, len(paths))
	for index, path := range paths {
		pattern.EvidenceLocations = append(pattern.EvidenceLocations, domain.PatternEvidenceLocation{
			Path:   path,
			Line:   index + 1,
			Symbol: "Current",
			Kind:   "function",
		})
	}
	return *pattern
}

type normalizePatternsFunc func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error)

func (f normalizePatternsFunc) NormalizePatterns(ctx context.Context, req *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
	return f(ctx, req)
}

func mergedNormalization(id string, sources ...domain.Pattern) *agent.NormalizePatternsResult {
	first := sources[0]
	sourceIDs := make([]string, 0, len(sources))
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.ID)
	}
	return &agent.NormalizePatternsResult{Patterns: []agent.PatternNormalization{{
		ID:          id,
		Name:        first.Name,
		Category:    string(first.Category),
		Description: first.Description,
		Rule:        first.Rule,
		Confidence:  first.Confidence,
		SourceIDs:   sourceIDs,
	}}}
}

func patternIDs(patterns []domain.Pattern) []string {
	ids := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		ids = append(ids, pattern.ID)
	}
	return ids
}
