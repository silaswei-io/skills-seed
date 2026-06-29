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

func TestCurateAndStoreUsesLocalCurationWithoutExistingPatterns(t *testing.T) {
	candidate := domain.NewPattern("candidate", "Error Wrapping", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("When repository errors occur, wrap them with operation context")

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
			require.Fail(t, "learn curation should not call AI by default")
			return nil, nil
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
	require.Equal(t, "candidate", saved[0].ID)
}

func TestCurateAndStoreMergesLocallyAndKeepsHigherQualityPattern(t *testing.T) {
	existing := domain.NewPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.7
	existing.Frequency = 2
	existing.SetRule("When errors occur, return contextual errors")
	candidate := domain.NewPattern("candidate", "Error Handling", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.Frequency = 8
	candidate.SetRule("When errors occur, return contextual errors")
	candidate.SetDescription("Repository operations return contextual errors")
	candidate.SetExamples("return fmt.Errorf(\"load user: %w\", err)", "")
	candidate.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/user.go", Line: 42, Symbol: "LoadUser", Kind: "function"}}

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
			require.Fail(t, "learn curation should not call AI by default")
			return nil, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, []string{"existing"}, deleted)
	require.Len(t, saved, 1)
	require.Equal(t, "candidate", saved[0].ID)
	require.Equal(t, []string{"existing", "candidate"}, saved[0].MergedFrom)
	require.Equal(t, 10, saved[0].Frequency)
	require.Equal(t, "internal/service/user.go:42", saved[0].EvidenceLocations[0].DisplayLocation())
}

func TestCurateAndStoreMergesMultipleDuplicatesLocally(t *testing.T) {
	existing := domain.NewPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.8
	existing.Frequency = 2
	existing.SetRule("wrap errors with context")
	existing.SetDescription("wrap errors with context")
	candidateA := domain.NewPattern("candidate-a", "Error Handling", domain.CategoryError)
	candidateA.Confidence = 0.9
	candidateA.SetRule("wrap errors with context")
	candidateA.SetDescription("wrap errors with context")
	candidateB := domain.NewPattern("candidate-b", "Error Handling", domain.CategoryError)
	candidateB.Confidence = 0.95
	candidateB.Frequency = 8
	candidateB.SetRule("wrap errors with context")
	candidateB.SetDescription("wrap errors with context")
	candidateB.SetExamples("return fmt.Errorf(\"create user: %w\", err)", "")

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
			require.Fail(t, "learn curation should not call AI by default")
			return nil, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidateA, *candidateB},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, "candidate-b", saved[0].ID)
	require.ElementsMatch(t, []string{"existing", "candidate-a", "candidate-b"}, saved[0].MergedFrom)
	require.Equal(t, 11, saved[0].Frequency)
	require.Equal(t, []string{"existing"}, deleted)
}

func TestCurateAndStoreUpdatesExistingPatternWithSameIDOnce(t *testing.T) {
	const patternID = "status-wrap-error-handling"

	existing := domain.NewPattern(patternID, "Status Wrap Error Handling", domain.CategoryError)
	existing.Confidence = 0.7
	existing.Frequency = 2
	existing.SetRule("return status wrapped errors with context")
	candidate := domain.NewPattern(patternID, "Status Wrap Error Handling", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.Frequency = 3
	candidate.SetRule("return status wrapped errors with context")
	candidate.SetExamples("return api.StatusError(ctx, status.Code(err), \"load cluster\", err)", "")

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*existing}, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}
	svc := NewService(nil, repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, patternID, saved[0].ID)
	require.Equal(t, []string{patternID}, saved[0].MergedFrom)
	require.Equal(t, 5, saved[0].Frequency)
	require.Equal(t, "return api.StatusError(ctx, status.Code(err), \"load cluster\", err)", saved[0].GoodExample)
}

func TestDeterministicCurateDeduplicatesExistingPatternsByID(t *testing.T) {
	const existingID = "same-id"

	existingA := domain.NewPattern(existingID, "Error Wrap", domain.CategoryError)
	existingA.Confidence = 0.6
	existingA.Frequency = 1
	existingA.SetRule("wrap errors with context")
	existingB := domain.NewPattern(existingID, "Error Wrap", domain.CategoryError)
	existingB.Confidence = 0.9
	existingB.Frequency = 2
	existingB.SetRule("wrap errors with context")
	candidate := domain.NewPattern("candidate", "Error Wrap", domain.CategoryError)
	candidate.Confidence = 0.8
	candidate.Frequency = 1
	candidate.SetRule("wrap errors with context")

	result := deterministicCurate([]domain.Pattern{*candidate}, []domain.Pattern{*existingA, *existingB})

	require.NoError(t, validateCurateResult(result, []domain.Pattern{*candidate}, []domain.Pattern{*existingA, *existingB}))
	require.Len(t, result.Patterns, 1)
	require.ElementsMatch(t, []string{existingID, "candidate"}, result.Patterns[0].MergedFrom)
}

func TestCurateAndStoreDoesNotUseAIDroppedCandidates(t *testing.T) {
	candidate := domain.NewPattern("candidate", "Error Handling", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("wrap errors with context")

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
			require.Fail(t, "learn curation should not call AI by default")
			return nil, nil
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
	require.Equal(t, "candidate", saved[0].ID)
	require.Empty(t, result.Dropped)
	require.Equal(t, 0, result.Summary.TotalDropped)
}

func TestCurateAndStoreNormalizesCategoryAliasesBeforeValidationAndSave(t *testing.T) {
	candidate := domain.NewPattern("path-traversal-protection", "Path Traversal Protection", domain.Category("security"))
	candidate.Confidence = 0.9
	candidate.SetDescription("Validate archive paths before extracting files")
	candidate.SetRule("When extracting archive entries, reject paths outside the target directory")
	candidate.SetExamples("cleanedTarget := filepath.Clean(targetDir)\ncleanedFile := filepath.Clean(filePath)\nif !strings.HasPrefix(cleanedFile, cleanedTarget+string(os.PathSeparator)) {\n\treturn fmt.Errorf(\"invalid path\")\n}", "")

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
			require.Fail(t, "learn curation should not call AI by default")
			return nil, nil
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
	p1 := domain.NewPattern("p1", "Error Wrap", domain.CategoryError)
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
	p1 := domain.NewPattern("p1", "Error Wrap", domain.CategoryError)
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
			require.Equal(t, OperationCompact, req.Operation)
			require.True(t, req.AllExisting)
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{
						ID:         "p1",
						Name:       "Error Wrap",
						Category:   string(domain.CategoryError),
						Rule:       "wrap errors",
						Confidence: 0.8,
						Frequency:  1,
						MergedFrom: []string{"p1"},
					},
				},
				Summary: agent.CurateSummary{TotalCandidates: 1, TotalExisting: 1, TotalWritten: 1},
			}, nil
		},
	}
	svc := NewService(mockAgent, repo)

	result, err := svc.Compact(context.Background(), CompactRequest{DryRun: true, UseAI: true})

	require.NoError(t, err)
	require.True(t, called)
	require.Len(t, result.Written, 1)
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
