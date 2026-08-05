package patternnorm

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndStoreKeepsCurrentCandidateSeparateFromSimilarExistingPattern(t *testing.T) {
	existing := newPatternNormTestPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.7
	existing.Frequency = 2
	existing.SetRule("When errors occur, return contextual errors")
	candidate := newPatternNormTestPattern("candidate", "Error Handling", domain.CategoryError)
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
	svc := NewService(repo)

	result, err := svc.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Empty(t, deleted)
	require.Len(t, saved, 1)
	require.Equal(t, "candidate", saved[0].ID)
	require.Equal(t, []string{"candidate"}, saved[0].MergedFrom)
	require.Equal(t, 1, saved[0].Frequency)
	require.Equal(t, "internal/service/user.go:42", saved[0].EvidenceLocations[0].DisplayLocation())
}

func TestNormalizeAndStoreKeepsMultipleCurrentCandidatesSeparateFromSimilarExistingPattern(t *testing.T) {
	existing := newPatternNormTestPattern("existing", "Error Handling", domain.CategoryError)
	existing.Confidence = 0.8
	existing.Frequency = 2
	existing.SetRule("wrap errors with context")
	existing.SetDescription("wrap errors with context")
	candidateA := newPatternNormTestPattern("candidate-a", "Error Handling", domain.CategoryError)
	candidateA.Confidence = 0.9
	candidateA.SetRule("wrap errors with context")
	candidateA.SetDescription("wrap errors with context")
	candidateB := newPatternNormTestPattern("candidate-b", "Error Handling", domain.CategoryError)
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
	svc := NewService(repo)

	result, err := svc.NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidateA, *candidateB},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Len(t, saved, 2)
	require.Equal(t, "candidate-a", saved[0].ID)
	require.Equal(t, "candidate-b", saved[1].ID)
	require.Equal(t, []string{"candidate-a"}, saved[0].MergedFrom)
	require.Equal(t, []string{"candidate-b"}, saved[1].MergedFrom)
	require.Empty(t, deleted)
}

func TestNormalizeAndStoreUpdatesExistingPatternWithSameIDOnce(t *testing.T) {
	const patternID = "status-wrap-error-handling"

	existing := newPatternNormTestPattern(patternID, "Status Wrap Error Handling", domain.CategoryError)
	existing.Confidence = 0.7
	existing.Frequency = 2
	existing.SetRule("return status wrapped errors with context")
	candidate := newPatternNormTestPattern(patternID, "Status Wrap Error Handling", domain.CategoryError)
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
	svc := NewService(repo)

	result, err := svc.NormalizeAndStore(context.Background(), NormalizeRequest{
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

func TestDeterministicNormalizeDeduplicatesExistingPatternsByID(t *testing.T) {
	const existingID = "same-id"

	existingA := newPatternNormTestPattern(existingID, "Error Wrap", domain.CategoryError)
	existingA.Confidence = 0.6
	existingA.Frequency = 1
	existingA.SetRule("wrap errors with context")
	existingB := newPatternNormTestPattern(existingID, "Error Wrap", domain.CategoryError)
	existingB.Confidence = 0.9
	existingB.Frequency = 2
	existingB.SetRule("wrap errors with context")
	candidate := newPatternNormTestPattern("candidate", "Error Wrap", domain.CategoryError)
	candidate.Confidence = 0.8
	candidate.Frequency = 1
	candidate.SetRule("wrap errors with context")

	result := deterministicNormalize([]domain.Pattern{*candidate}, []domain.Pattern{*existingA, *existingB})

	require.NoError(t, validateNormalizeResult(result, []domain.Pattern{*candidate}, []domain.Pattern{*existingA, *existingB}))
	require.Len(t, result.Patterns, 1)
	require.ElementsMatch(t, []string{existingID, "candidate"}, result.Patterns[0].MergedFrom)
}

func TestDeterministicNormalizeDoesNotMergeHighRiskBoundaryIntoNormalCapability(t *testing.T) {
	existing := newPatternNormTestPattern("resource-update", "Resource Lifecycle Update", domain.CategoryBusiness)
	existing.Confidence = 0.9
	existing.SetDescription("Update resource lifecycle state through the verified capability entry.")
	existing.SetRule("When changing resource state, inspect the lifecycle entry.")
	existing.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/resource/lifecycle.ts", Symbol: "updateResource"}}

	candidate := newPatternNormTestPattern("resource-destroy", "Resource Lifecycle Destroy", domain.CategoryBusiness)
	candidate.Confidence = 0.9
	candidate.SetDescription("Destroy command deletes resource state and has external environment side effects.")
	candidate.SetRule("When changing destroy behavior, inspect the command safeguards before modifying it.")
	candidate.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "tools/commands/destroy.ts", Symbol: "destroyResource"}}

	result := deterministicNormalize([]domain.Pattern{*candidate}, []domain.Pattern{*existing})

	require.Len(t, result.Patterns, 1)
	require.Equal(t, "resource-destroy", result.Patterns[0].ID)
	require.Equal(t, []string{"resource-destroy"}, result.Patterns[0].MergedFrom)
}

func TestNormalizeAndStoreDoesNotUseAIDroppedCandidates(t *testing.T) {
	candidate := newPatternNormTestPattern("candidate", "Error Handling", domain.CategoryError)
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
	svc := NewService(repo)

	result, err := svc.NormalizeAndStore(context.Background(), NormalizeRequest{
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

func TestNormalizeAndStoreNormalizesCategoryAliasesBeforeValidationAndSave(t *testing.T) {
	candidate := newPatternNormTestPattern("path-traversal-protection", "Path Traversal Protection", domain.Category("security"))
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
	svc := NewService(repo)

	result, err := svc.NormalizeAndStore(context.Background(), NormalizeRequest{
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
