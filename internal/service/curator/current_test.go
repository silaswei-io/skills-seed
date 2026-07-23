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

func TestCurateAndStoreResolvesOverlappingSourceOwnership(t *testing.T) {
	contextInjection := newCuratorTestPattern("plugin-core-context-injection", "Plugin Context", domain.CategoryStructure)
	pluginLifecycle := newCuratorTestPattern("plugin-lifecycle-registration", "Plugin Lifecycle", domain.CategoryStructure)
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(context.Context, *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{
				Patterns: []agent.CuratedPattern{
					{
						ID:        contextInjection.ID,
						Name:      contextInjection.Name,
						Category:  string(contextInjection.Category),
						Rule:      contextInjection.Rule,
						SourceIDs: []string{contextInjection.ID},
					},
					{
						ID:        pluginLifecycle.ID,
						Name:      pluginLifecycle.Name,
						Category:  string(pluginLifecycle.Category),
						Rule:      pluginLifecycle.Rule,
						SourceIDs: []string{pluginLifecycle.ID, contextInjection.ID},
					},
				},
				Dropped: []agent.CuratedDrop{{ID: contextInjection.ID, Reason: "merged into lifecycle"}},
			}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*contextInjection, *pluginLifecycle},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Empty(t, result.Dropped)
	written := make(map[string]domain.Pattern, len(result.Written))
	for _, pattern := range result.Written {
		written[pattern.ID] = pattern
	}
	require.Equal(t, []string{contextInjection.ID}, written[contextInjection.ID].MergedFrom)
	require.Equal(t, []string{pluginLifecycle.ID}, written[pluginLifecycle.ID].MergedFrom)
}

func TestCurateAndStoreRecoversAmbiguousSourceOwnership(t *testing.T) {
	candidate := newCuratorTestPattern("shared-source", "Shared Source", domain.CategoryBusiness)
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(context.Context, *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{
				{ID: "first-output", Name: "First", Category: string(domain.CategoryBusiness), Rule: "First rule", SourceIDs: []string{candidate.ID}},
				{ID: "second-output", Name: "Second", Category: string(domain.CategoryBusiness), Rule: "Second rule", SourceIDs: []string{candidate.ID}},
			}}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.Equal(t, candidate.ID, result.Written[0].ID)
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

func TestCurateAndStoreCanonicalizesDuplicateDroppedDecisions(t *testing.T) {
	dropped := newCuratorTestPattern("service-group-coordinated-startup", "Service Startup", domain.CategoryStructure)
	recovered := newCuratorTestPattern("login-log-list-query", "Login Log Query", domain.CategoryDatabase)

	repo := &mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(context.Context, *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			return &agent.CuratePatternsResult{Dropped: []agent.CuratedDrop{
				{ID: dropped.ID, Reason: "framework convention"},
				{ID: dropped.ID, Reason: "framework convention"},
			}}, nil
		},
	}

	result, err := NewService(mockAgent, repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*dropped, *recovered},
	})

	require.NoError(t, err)
	require.Len(t, result.Dropped, 1)
	require.Equal(t, dropped.ID, result.Dropped[0].ID)
	require.Len(t, result.Written, 1)
	require.Equal(t, recovered.ID, result.Written[0].ID)
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

func TestCurateAndStoreReplaysSavedAIDecision(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Candidate", domain.CategoryBusiness)
	checkpoint := &memoryDecisionCheckpoint{}
	calls := 0
	mockAgent := &mocks.MockAgent{
		NameVal: "mock", AvailableVal: true,
		CuratePatternsFn: func(context.Context, *agent.CuratePatternsRequest) (*agent.CuratePatternsResult, error) {
			calls++
			return &agent.CuratePatternsResult{Patterns: []agent.CuratedPattern{{
				ID:        candidate.ID,
				Name:      candidate.Name,
				Category:  string(candidate.Category),
				Rule:      candidate.Rule,
				SourceIDs: []string{candidate.ID},
			}}}, nil
		},
	}
	request := CurateRequest{
		Operation:          OperationLearnCurrent,
		Candidates:         []domain.Pattern{*candidate},
		DecisionCheckpoint: checkpoint,
	}

	first, err := NewService(mockAgent, &mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).CurateAndStore(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, first.Written, 1)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, checkpoint.saves)

	second, err := NewService(nil, &mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).CurateAndStore(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, second.Written, 1)
	require.Equal(t, 1, calls)
}

type memoryDecisionCheckpoint struct {
	key      string
	decision *agent.CuratePatternsResult
	saves    int
}

func (c *memoryDecisionCheckpoint) Load(_ context.Context, key string) (*agent.CuratePatternsResult, bool, error) {
	if c.key != key || c.decision == nil {
		return nil, false, nil
	}
	return c.decision, true, nil
}

func (c *memoryDecisionCheckpoint) Save(_ context.Context, key string, result *agent.CuratePatternsResult) error {
	c.key = key
	c.decision = result
	c.saves++
	return nil
}
