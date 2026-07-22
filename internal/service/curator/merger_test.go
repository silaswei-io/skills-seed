package curator

import (
	"context"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCurateAndStoreMergesLocallyAndKeepsHigherQualityPattern(t *testing.T) {
	existing := newCuratorTestPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.7
	existing.Frequency = 2
	existing.SetRule("When errors occur, return contextual errors")
	candidate := newCuratorTestPattern("candidate", "Error Handling", domain.CategoryError)
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
	svc := NewService(newDeterministicCuratorAgent(), repo)

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
	require.Equal(t, 2, saved[0].Frequency)
	require.Equal(t, "internal/service/user.go:42", saved[0].EvidenceLocations[0].DisplayLocation())
}

func TestCurateAndStoreMergesMultipleDuplicatesLocally(t *testing.T) {
	existing := newCuratorTestPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.8
	existing.Frequency = 2
	existing.SetRule("wrap errors with context")
	existing.SetDescription("wrap errors with context")
	candidateA := newCuratorTestPattern("candidate-a", "Error Handling", domain.CategoryError)
	candidateA.Confidence = 0.9
	candidateA.SetRule("wrap errors with context")
	candidateA.SetDescription("wrap errors with context")
	candidateB := newCuratorTestPattern("candidate-b", "Error Handling", domain.CategoryError)
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
	svc := NewService(newDeterministicCuratorAgent(), repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidateA, *candidateB},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, "candidate-b", saved[0].ID)
	require.ElementsMatch(t, []string{"existing", "candidate-a", "candidate-b"}, saved[0].MergedFrom)
	require.Equal(t, 3, saved[0].Frequency)
	require.Equal(t, []string{"existing"}, deleted)
}

func TestCurateAndStoreUpdatesExistingPatternWithSameIDOnce(t *testing.T) {
	const patternID = "status-wrap-error-handling"

	existing := newCuratorTestPattern(patternID, "Status Wrap Error Handling", domain.CategoryError)
	existing.Confidence = 0.7
	existing.Frequency = 2
	existing.SetRule("return status wrapped errors with context")
	candidate := newCuratorTestPattern(patternID, "Status Wrap Error Handling", domain.CategoryError)
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
	svc := NewService(newDeterministicCuratorAgent(), repo)

	result, err := svc.CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, patternID, saved[0].ID)
	require.Equal(t, []string{patternID}, saved[0].MergedFrom)
	require.Equal(t, 1, saved[0].Frequency)
	require.Equal(t, "return api.StatusError(ctx, status.Code(err), \"load cluster\", err)", saved[0].GoodExample)
}

func TestDeterministicCurateDeduplicatesExistingPatternsByID(t *testing.T) {
	const existingID = "same-id"

	existingA := newCuratorTestPattern(existingID, "Error Wrap", domain.CategoryError)
	existingA.Confidence = 0.6
	existingA.Frequency = 1
	existingA.SetRule("wrap errors with context")
	existingB := newCuratorTestPattern(existingID, "Error Wrap", domain.CategoryError)
	existingB.Confidence = 0.9
	existingB.Frequency = 2
	existingB.SetRule("wrap errors with context")
	candidate := newCuratorTestPattern("candidate", "Error Wrap", domain.CategoryError)
	candidate.Confidence = 0.8
	candidate.Frequency = 1
	candidate.SetRule("wrap errors with context")

	result := deterministicCurate([]domain.Pattern{*candidate}, []domain.Pattern{*existingA, *existingB})

	require.NoError(t, validateCurateResult(result, []domain.Pattern{*candidate}, []domain.Pattern{*existingA, *existingB}))
	require.Len(t, result.Patterns, 1)
	require.ElementsMatch(t, []string{existingID, "candidate"}, result.Patterns[0].MergedFrom)
}

func TestCurateAndStoreDoesNotUseAIDroppedCandidates(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Error Handling", domain.CategoryError)
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
	svc := NewService(newDeterministicCuratorAgent(), repo)

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
	candidate := newCuratorTestPattern("path-traversal-protection", "Path Traversal Protection", domain.Category("security"))
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
	svc := NewService(newDeterministicCuratorAgent(), repo)

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
