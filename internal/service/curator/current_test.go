package curator

import (
	"context"
	"errors"
	"fmt"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCurateAndStoreUsesSemanticCurationForLearnCurrent(t *testing.T) {
	keep := newCuratorTestPattern("keep", "Keep", domain.CategoryBusiness)
	keep.Confidence = 0.80
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
					ID:        keep.ID,
					Name:      keep.Name,
					Category:  string(keep.Category),
					Rule:      keep.Rule,
					SourceIDs: []string{keep.ID},
				}},
				Dropped: []agent.CuratedDrop{{ID: drop.ID, Reason: "local fact without a reusable invariant"}},
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

func TestCurateAndStorePrefersCurrentPatternOverConflictingDrop(t *testing.T) {
	candidate := newCuratorTestPattern("config-backup-export-import-flow", "Config Backup Flow", domain.CategoryBusiness)
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(context.Context, *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{{
					ID:        candidate.ID,
					Name:      candidate.Name,
					Category:  string(candidate.Category),
					Rule:      candidate.Rule,
					SourceIDs: []string{candidate.ID},
				}},
				Dropped: []agent.CuratedDrop{{ID: candidate.ID, Reason: "duplicate decision"}},
			}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Empty(t, result.Dropped)
}

func TestCurateAndStoreRecoversUnclassifiedCoverageLocally(t *testing.T) {
	first := newCuratorTestPattern("first", "First", domain.CategoryError)
	first.Rule = "Wrap repository errors with operation context."
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "first.go", Line: 10, Symbol: "First", Kind: "function"}}
	second := newCuratorTestPattern("second", "Second", domain.CategoryBusiness)
	second.Rule = "Check the account state before activation."
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "second.go", Line: 20, Symbol: "Second", Kind: "function"}}

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved = append(saved, pattern)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{{
				ID:        first.ID,
				Name:      first.Name,
				Category:  string(first.Category),
				Rule:      first.Rule,
				SourceIDs: []string{first.ID},
			}}}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Len(t, saved, 2)
}

func TestCurateAndStoreDoesNotRetryIncompleteCoverage(t *testing.T) {
	candidates := make([]domain.Pattern, 0, 4)
	for i := 0; i < 4; i++ {
		pattern := newCuratorTestPattern(fmt.Sprintf("candidate-%d", i), fmt.Sprintf("Candidate %d", i), domain.CategoryBusiness)
		pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: fmt.Sprintf("service-%d.go", i), Line: 10, Symbol: fmt.Sprintf("Apply%d", i), Kind: "function"}}
		candidates = append(candidates, *pattern)
	}

	var calls int
	repo := &mocks.MockPatternRepository{GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil }}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			calls++
			limit := 1
			patterns := make([]agent.CuratedPattern, 0, limit)
			for i := 0; i < limit; i++ {
				patterns = append(patterns, agent.CuratedPattern{
					ID:        candidates[i].ID,
					Name:      candidates[i].Name,
					Category:  string(candidates[i].Category),
					Rule:      candidates[i].Rule,
					SourceIDs: []string{candidates[i].ID},
				})
			}
			return &agent.CuratePatternsResult{Patterns: patterns}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: candidates,
	})

	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Len(t, result.Written, 4)
}

func TestCurateAndStoreIgnoresUnknownMergedSourceWhenKnownSourcesRemain(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Candidate", domain.CategoryBusiness)
	candidate.Rule = "Reuse the verified business entry."
	candidate.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Line: 10, Symbol: "Apply", Kind: "function"}}

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved = append(saved, pattern)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{{
				ID:        candidate.ID,
				Name:      candidate.Name,
				Category:  string(candidate.Category),
				Rule:      candidate.Rule,
				SourceIDs: []string{candidate.ID, "invented-source"},
			}}}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Len(t, saved, 1)
	require.Equal(t, []string{candidate.ID}, saved[0].MergedFrom)
}

func TestCurateAndStoreIgnoresOrphanPatternWithoutTraceableSource(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Candidate", domain.CategoryBusiness)
	candidate.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "service.go", Line: 10, Symbol: "Apply", Kind: "function"}}

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
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{{
					ID:        "invented-pattern",
					Name:      "Invented Pattern",
					Category:  string(domain.CategoryBusiness),
					Rule:      "Invented rule",
					SourceIDs: []string{"invented-source"},
				}},
				Dropped: []agent.CuratedDrop{{ID: candidate.ID, Reason: "not reusable"}},
			}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.Written)
	require.Len(t, result.Dropped, 1)
	require.False(t, saved)
}

func TestLearnCurrentCurationRegressionKeepsEighteenPatternsInsteadOfAllCandidates(t *testing.T) {
	const (
		candidateCount = 206
		curatedCount   = 18
		droppedCount   = 22
	)
	candidates := make([]domain.Pattern, 0, candidateCount)
	for i := 0; i < candidateCount; i++ {
		pattern := newCuratorTestPattern(fmt.Sprintf("candidate-%03d", i), fmt.Sprintf("Candidate %03d", i), domain.CategoryBusiness)
		pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: fmt.Sprintf("internal/service/service_%03d.go", i), Line: 10, Symbol: fmt.Sprintf("Apply%03d", i), Kind: "function"}}
		candidates = append(candidates, *pattern)
	}

	curated := make([]agent.CuratedPattern, curatedCount)
	mergeCandidateCount := candidateCount - droppedCount
	for i := 0; i < curatedCount; i++ {
		start := i * mergeCandidateCount / curatedCount
		end := (i + 1) * mergeCandidateCount / curatedCount
		mergedFrom := make([]string, 0, end-start+1)
		for candidateIndex := start; candidateIndex < end; candidateIndex++ {
			mergedFrom = append(mergedFrom, candidates[candidateIndex].ID)
		}
		curated[i] = agent.CuratedPattern{
			ID:        fmt.Sprintf("canonical-%02d", i),
			Name:      fmt.Sprintf("Canonical %02d", i),
			Category:  string(domain.CategoryBusiness),
			Rule:      fmt.Sprintf("Reuse canonical flow %02d", i),
			SourceIDs: mergedFrom,
		}
	}
	curated[0].SourceIDs = append(curated[0].SourceIDs, "ca-layered-error-code-pattern")
	dropped := make([]agent.CuratedDrop, 0, droppedCount)
	for i := mergeCandidateCount; i < candidateCount; i++ {
		dropped = append(dropped, agent.CuratedDrop{ID: candidates[i].ID, Reason: "local duplicate"})
	}

	var saved int
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved++
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(ctx context.Context, req *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{Patterns: curated, Dropped: dropped}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: candidates,
	})

	require.NoError(t, err)
	require.Len(t, result.Written, curatedCount)
	require.Len(t, result.Dropped, droppedCount)
	require.Equal(t, curatedCount, saved)
}

func TestHydrateCurrentCurateResultReplacesEvidenceFromUndeclaredSource(t *testing.T) {
	first := newCuratorTestPattern("first", "First", domain.CategoryBusiness)
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "first.go", Line: 10, Symbol: "First", Kind: "func"}}
	second := newCuratorTestPattern("second", "Second", domain.CategoryBusiness)
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "second.go", Line: 20, Symbol: "Second", Kind: "func"}}
	result := &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{{
		ID:        first.ID,
		SourceIDs: []string{first.ID},
	}}}
	curated := proposalFromAgent(result)

	require.NoError(t, hydrateCurateResult(curated, []domain.Pattern{*first, *second}, nil))
	require.Equal(t, first.EvidenceLocations, curated.Patterns[0].EvidenceLocations)
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
					ID:        candidate.ID,
					Name:      candidate.Name,
					Category:  string(candidate.Category),
					Rule:      candidate.Rule,
					SourceIDs: []string{candidate.ID},
				}},
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
