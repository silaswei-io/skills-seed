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

func TestCurateAndStoreUsesSemanticCurationForLearnCurrent(t *testing.T) {
	keep := newCuratorTestPattern("keep", "Keep", domain.CategoryBusiness)
	keep.Rule = "When changing the flow, preserve the verified invariant."
	keep.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Line: 10, Symbol: "Create", Kind: "func"}}
	drop := newCuratorTestPattern("drop", "Drop", domain.CategoryBusiness)
	drop.Rule = "Describe a local implementation detail."
	drop.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Line: 20, Symbol: "Enable", Kind: "func"}}

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved = append(saved, pattern)
			return nil
		},
	}
	called := false
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			called = true
			require.Len(t, req.CandidatePatterns, 2)
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{{
					ID:                keep.ID,
					Name:              keep.Name,
					Category:          string(keep.Category),
					Rule:              keep.Rule,
					EvidenceLocations: keep.EvidenceLocations,
					MergedFrom:        []string{keep.ID},
					Source:            string(domain.SourceLearnedCurrent),
				}},
				Dropped: []agent.CuratedDrop{{ID: drop.ID, Reason: "local fact without a reusable invariant"}},
				Summary: agent.CurateSummary{TotalCandidates: 2, TotalWritten: 1, TotalDropped: 1},
			}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*keep, *drop},
	})

	require.NoError(t, err)
	require.True(t, called)
	require.Len(t, result.Written, 1)
	require.Len(t, result.Dropped, 1)
	require.Len(t, saved, 1)
	require.Equal(t, 1, saved[0].Frequency)
	require.Equal(t, 0.80, saved[0].Confidence)
}

func TestHydrateCurrentCurateResultReplacesEvidenceFromUndeclaredSource(t *testing.T) {
	first := newCuratorTestPattern("first", "First", domain.CategoryBusiness)
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "first.go", Line: 10, Symbol: "First", Kind: "func"}}
	second := newCuratorTestPattern("second", "Second", domain.CategoryBusiness)
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "second.go", Line: 20, Symbol: "Second", Kind: "func"}}
	result := &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{{
		ID:                first.ID,
		MergedFrom:        []string{first.ID},
		EvidenceLocations: second.EvidenceLocations,
	}}}

	require.NoError(t, hydrateCurrentCurateResult(result, []domain.Pattern{*first, *second}, nil))
	require.Equal(t, first.EvidenceLocations, result.Patterns[0].EvidenceLocations)
}

func TestCoalesceCurrentCandidatesCombinesEvidenceAcrossUnits(t *testing.T) {
	first := newCuratorTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	first.AnalysisUnitID = "unit-a"
	first.AnalysisUnitName = "Unit A"
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "a.go", Line: 10, Symbol: "A", Kind: "func"}}
	second := newCuratorTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	second.AnalysisUnitID = "unit-b"
	second.AnalysisUnitName = "Unit B"
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "b.go", Line: 20, Symbol: "B", Kind: "func"}}

	coalesced := coalesceCurrentCandidates([]domain.Pattern{*first, *second})

	require.Len(t, coalesced, 1)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), coalesced[0].EvidenceLocations)
	require.Empty(t, coalesced[0].AnalysisUnitID)
	require.Empty(t, coalesced[0].AnalysisUnitName)
}

func TestCurateAndStoreHydratesSourceOwnedFieldsFromMergedSources(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Error Wrapping", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("When repository errors occur, wrap them with operation context")
	candidate.BadExample = "Return the repository error without operation context."
	candidate.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "repository.go", Line: 10, Symbol: "Load", Kind: "func"}}
	candidate.Source = domain.SourceLearnedCurrent
	candidate.ProjectID = "ca-admin"
	candidate.ScopePath = "services/ca-admin"
	candidate.WorkspaceRole = "service"
	candidate.AnalysisUnitID = "repository-errors"
	candidate.AnalysisUnitName = "Repository errors"

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
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{{
					ID:                candidate.ID,
					Name:              candidate.Name,
					Category:          string(candidate.Category),
					Rule:              candidate.Rule,
					GoodExample:       "invented natural-language example",
					BadExample:        "rewritten bad example",
					Source:            string(domain.SourceUserDefined),
					ProjectID:         "invented-project",
					ScopePath:         "invented/scope",
					WorkspaceRole:     "invented-role",
					AnalysisUnitID:    "invented-unit",
					AnalysisUnitName:  "Invented unit",
					MergedFrom:        []string{candidate.ID},
					EvidenceLocations: []domain.PatternEvidenceLocation{{Path: "invented.go", Line: 99}},
				}},
				Summary: agent.CurateSummary{TotalCandidates: 99, TotalWritten: 99, TotalDropped: 99},
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
	require.Equal(t, "candidate", saved[0].ID)
	require.Empty(t, saved[0].GoodExample)
	require.Equal(t, candidate.BadExample, saved[0].BadExample)
	require.Equal(t, candidate.EvidenceLocations, saved[0].EvidenceLocations)
	require.Equal(t, candidate.Source, saved[0].Source)
	require.Equal(t, candidate.ProjectID, saved[0].ProjectID)
	require.Equal(t, candidate.ScopePath, saved[0].ScopePath)
	require.Equal(t, candidate.WorkspaceRole, saved[0].WorkspaceRole)
	require.Equal(t, candidate.AnalysisUnitID, saved[0].AnalysisUnitID)
	require.Equal(t, candidate.AnalysisUnitName, saved[0].AnalysisUnitName)
	require.Equal(t, 1, result.Summary.TotalCandidates)
	require.Equal(t, 1, result.Summary.TotalWritten)
	require.Zero(t, result.Summary.TotalDropped)
}

func TestCurateAndStoreDoesNotPersistCurrentCandidatesWhenSemanticCurationFails(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Error Wrapping", domain.CategoryError)
	var saved bool
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved = true
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return nil, errors.New("curator unavailable")
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.ErrorContains(t, err, "curate current patterns")
	require.Nil(t, result)
	require.False(t, saved)
}

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
			return deterministicCurate(req.CandidatePatterns, req.ExistingPatterns), nil
		},
	}
}
