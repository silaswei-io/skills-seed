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

func TestCurateAndStoreUsesLocalCurationForLearnCurrent(t *testing.T) {
	first := newCuratorTestPattern("api-contract", "API Contract", domain.CategoryAPI)
	first.Rule = "Preserve the repository-specific API response contract."
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/api/user.go", Line: 10, Symbol: "User", Kind: "handler"}}
	second := newCuratorTestPattern("order-flow", "Order Flow", domain.CategoryBusiness)
	second.Rule = "Preserve the repository-specific order state transition."
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/service/order.go", Line: 20, Symbol: "CreateOrder", Kind: "function"}}

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved = append(saved, pattern)
			return nil
		},
	}

	result, err := NewService(repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Empty(t, result.Dropped)
	require.Len(t, saved, 2)
}

func TestCurateAndStoreMergesEquivalentCurrentCandidatesLocally(t *testing.T) {
	first := newCuratorTestPattern("first", "Shared Error Rule", domain.CategoryError)
	first.Rule = "Wrap repository errors with operation context."
	first.Confidence = 0.70
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "first.go", Line: 10, Symbol: "First", Kind: "func"}}
	second := newCuratorTestPattern("second", "Shared Error Rule", domain.CategoryError)
	second.Rule = "Wrap repository errors with operation context."
	second.Confidence = 0.90
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "second.go", Line: 20, Symbol: "Second", Kind: "func"}}

	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.ElementsMatch(t, []string{first.ID, second.ID}, result.Written[0].MergedFrom)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), result.Written[0].EvidenceLocations)
}

func TestCurateAndStoreReplaysSavedCurrentCurationDecision(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Candidate", domain.CategoryBusiness)
	checkpoint := &memoryDecisionCheckpoint{}
	request := CurateRequest{
		Operation:          OperationLearnCurrent,
		Candidates:         []domain.Pattern{*candidate},
		DecisionCheckpoint: checkpoint,
	}

	first, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).CurateAndStore(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, first.Written, 1)
	require.Equal(t, 1, checkpoint.saves)

	second, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).CurateAndStore(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, second.Written, 1)
	require.Equal(t, 1, checkpoint.saves)
}

func TestCurateAndStoreCompactsDuplicateCurrentCandidatesBeforeLocalCuration(t *testing.T) {
	first := newCuratorTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "a.go", Line: 10, Symbol: "A", Kind: "func"}}
	second := newCuratorTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "b.go", Line: 20, Symbol: "B", Kind: "func"}}

	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), result.Written[0].EvidenceLocations)
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

func TestCoalesceCurrentCandidatesCombinesEvidenceAcrossFocuses(t *testing.T) {
	first := newCuratorTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "a.go", Line: 10, Symbol: "A", Kind: "func"}}
	second := newCuratorTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "b.go", Line: 20, Symbol: "B", Kind: "func"}}

	coalesced := coalesceCurrentCandidates([]domain.Pattern{*first, *second})

	require.Len(t, coalesced, 1)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), coalesced[0].EvidenceLocations)
}

func TestCurateAndStoreHydratesSourceOwnedFieldsFromCurrentCandidate(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Error Wrapping", domain.CategoryError)
	candidate.Confidence = 0.9
	candidate.SetRule("When repository errors occur, wrap them with operation context")
	candidate.BadExample = "Return the repository error without operation context."
	candidate.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "repository.go", Line: 10, Symbol: "Load", Kind: "func"}}
	candidate.Source = domain.SourceLearnedCurrent
	candidate.ProjectID = "ca-admin"
	candidate.ScopePath = "services/ca-admin"
	candidate.WorkspaceRole = "service"

	var saved []*domain.Pattern
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p)
			return nil
		},
	}

	result, err := NewService(repo).CurateAndStore(context.Background(), CurateRequest{
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
	require.Equal(t, 1, result.Summary.TotalCandidates)
	require.Equal(t, 1, result.Summary.TotalWritten)
	require.Zero(t, result.Summary.TotalDropped)
}

func TestCurateAndStoreDoesNotPersistCurrentCandidatesWhenStoreFails(t *testing.T) {
	candidate := newCuratorTestPattern("candidate", "Error Wrapping", domain.CategoryError)
	var saved bool
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved = true
			return errors.New("db closed")
		},
	}

	result, err := NewService(repo).CurateAndStore(context.Background(), CurateRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.ErrorContains(t, err, "apply curated patterns")
	require.Nil(t, result)
	require.True(t, saved)
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
