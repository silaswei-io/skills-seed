package curator

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCurateAndStoreAddsNewCandidate(t *testing.T) {
	candidate := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("When returning repository errors, wrap them with operation context")

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
}

func TestCurateAndStoreCallsAgentWithoutExistingPatterns(t *testing.T) {
	candidate := domain.NewPattern("candidate", "Error Wrapping", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("When repository errors occur, wrap them with operation context")

	var called bool
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
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			called = true
			require.Equal(t, OperationLearnCurrent, req.Operation)
			require.Len(t, req.CandidatePatterns, 1)
			require.Empty(t, req.ExistingPatterns)
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{
						ID:          "candidate",
						Name:        "Error Wrapping",
						Category:    string(domain.CategoryError),
						Rule:        "When repository errors occur, wrap them with operation context",
						Confidence:  0.9,
						Frequency:   1,
						MergedFrom:  []string{"candidate"},
						MergeReason: "new candidate",
						Source:      string(domain.SourceLearnedCurrent),
					},
				},
				Summary: agent.CurateSummary{TotalCandidates: 1, TotalWritten: 1},
			}, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.True(t, called)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, "candidate", saved[0].ID)
}

func TestCurateAndStoreUsesAgentToMergeRelatedExistingPattern(t *testing.T) {
	existing := domain.NewPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.7
	existing.Frequency = 2
	existing.SetRule("When errors occur, return contextual errors")
	candidate := domain.NewPattern("candidate", "Error Handling", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("When repository errors occur, return contextual errors")

	var deleted []string
	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*existing}, nil
		},
		DeleteFn: func(ctx context.Context, id string) error {
			deleted = append(deleted, id)
			return nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			require.Len(t, req.CandidatePatterns, 1)
			require.Len(t, req.ExistingPatterns, 1)
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{
						ID:          "existing",
						Name:        "Error Handling",
						Category:    string(domain.CategoryError),
						Rule:        "When repository errors occur, return contextual errors",
						Confidence:  0.85,
						Frequency:   3,
						MergedFrom:  []string{"existing", "candidate"},
						MergeReason: "same rule",
						Source:      string(domain.SourceLearnedCurrent),
					},
				},
				Summary: agent.CurateSummary{TotalCandidates: 1, TotalExisting: 1, TotalWritten: 1},
			}, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Empty(t, deleted)
	require.Len(t, saved, 1)
	require.Equal(t, "existing", saved[0].ID)
	require.Equal(t, []string{"existing", "candidate"}, saved[0].MergedFrom)
}

func TestCurateAndStoreFallsBackWhenAgentResultInvalid(t *testing.T) {
	existing := domain.NewPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.8
	existing.Frequency = 2
	existing.SetRule("wrap errors with context")
	candidate := domain.NewPattern("candidate", "Error Handling", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("wrap errors with context")

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*existing}, nil
		},
		DeleteFn: func(ctx context.Context, id string) error {
			return nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{ID: "bad", Name: "Bad", Category: "not-a-category", Confidence: 2, MergedFrom: []string{"missing"}},
				},
			}, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, "existing", saved[0].ID)
}

func TestCurateAndStoreNormalizesCategoryAliasesBeforeValidationAndSave(t *testing.T) {
	candidate := domain.NewPattern("path-traversal-protection", "Path Traversal Protection", domain.Category("security"))
	candidate.Confidence = 0.9
	candidate.SetDescription("Validate archive paths before extracting files")
	candidate.SetRule("When extracting archive entries, reject paths outside the target directory")
	candidate.SetExamples("cleanedTarget := filepath.Clean(targetDir)\ncleanedFile := filepath.Clean(filePath)\nif !strings.HasPrefix(cleanedFile, cleanedTarget+string(os.PathSeparator)) {\n\treturn fmt.Errorf(\"invalid path\")\n}", "")

	var saved []*domain.Pattern
	var called bool
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return nil, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			called = true
			require.Len(t, req.CandidatePatterns, 1)
			require.Equal(t, domain.CategoryUtils, req.CandidatePatterns[0].Category)

			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{
						ID:          "path-traversal-protection",
						Name:        "Path Traversal Protection",
						Category:    "security",
						Description: "Validate archive paths before extracting files",
						GoodExample: candidate.GoodExample,
						Rule:        "When extracting archive entries, reject paths outside the target directory",
						Confidence:  0.9,
						Frequency:   1,
						MergedFrom:  []string{"path-traversal-protection"},
						MergeReason: "new candidate",
						Source:      string(domain.SourceLearnedCurrent),
					},
				},
				Summary: agent.CurateSummary{TotalCandidates: 1, TotalWritten: 1},
			}, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.True(t, called)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, domain.CategoryUtils, result.Written[0].Category)
	require.Equal(t, domain.CategoryUtils, saved[0].Category)
	require.Equal(t, "path-traversal-protection", saved[0].ID)
}

func TestCompactDryRunDoesNotWrite(t *testing.T) {
	p1 := domain.NewPattern("p1", "Error Wrap", domain.CategoryError)
	p1.Confidence = 0.8
	p1.SetRule("wrap errors")
	p2 := domain.NewPattern("p2", "Error Wrap", domain.CategoryError)
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
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{
						ID:          "p1",
						Name:        "Error Wrap",
						Category:    string(domain.CategoryError),
						Rule:        "wrap errors",
						Confidence:  0.85,
						Frequency:   2,
						MergedFrom:  []string{"p1", "p2"},
						MergeReason: "same rule",
					},
				},
				Summary: agent.CurateSummary{TotalCandidates: 2, TotalExisting: 2, TotalWritten: 1},
			}, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.Compact(context.Background(), CompactRequest{DryRun: true})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, "p1", result.Written[0].ID)
	require.Equal(t, 2, result.Summary.TotalCandidates)
	require.Equal(t, 2, result.Summary.TotalExisting)
	require.Equal(t, 1, result.Summary.TotalWritten)
}

func TestCompactNormalizesRequestedCategory(t *testing.T) {
	p1 := domain.NewPattern("p1", "Utility Path Guard", domain.CategoryUtils)
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
