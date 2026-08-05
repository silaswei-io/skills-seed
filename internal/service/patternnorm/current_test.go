package patternnorm

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAndStoreUsesLocalNormalizationForLearnCurrent(t *testing.T) {
	first := newPatternNormTestPattern("api-contract", "API Contract", domain.CategoryAPI)
	first.Rule = "Preserve the repository-specific API response contract."
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/api/user.go", Line: 10, Symbol: "User", Kind: "handler"}}
	second := newPatternNormTestPattern("order-flow", "Order Flow", domain.CategoryBusiness)
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

	result, err := NewService(repo).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Empty(t, result.Dropped)
	require.Len(t, saved, 2)
}

func TestNormalizeAndStoreKeepsEquivalentCurrentCandidatesForRecall(t *testing.T) {
	first := newPatternNormTestPattern("first", "Shared Error Rule", domain.CategoryError)
	first.Rule = "Wrap repository errors with operation context."
	first.Confidence = 0.70
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "first.go", Line: 10, Symbol: "First", Kind: "func"}}
	second := newPatternNormTestPattern("second", "Shared Error Rule", domain.CategoryError)
	second.Rule = "Wrap repository errors with operation context."
	second.Confidence = 0.90
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "second.go", Line: 20, Symbol: "Second", Kind: "func"}}

	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Equal(t, []string{first.ID}, result.Written[0].MergedFrom)
	require.Equal(t, []string{second.ID}, result.Written[1].MergedFrom)
	require.Equal(t, first.EvidenceLocations, result.Written[0].EvidenceLocations)
	require.Equal(t, second.EvidenceLocations, result.Written[1].EvidenceLocations)
}

func TestNormalizeAndStoreReplaysSavedCurrentNormalizationDecision(t *testing.T) {
	candidate := newPatternNormTestPattern("candidate", "Candidate", domain.CategoryBusiness)
	checkpoint := &memoryDecisionCheckpoint{}
	request := NormalizeRequest{
		Operation:          OperationLearnCurrent,
		Candidates:         []domain.Pattern{*candidate},
		DecisionCheckpoint: checkpoint,
	}

	first, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).NormalizeAndStore(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, first.Written, 1)
	require.Equal(t, 1, checkpoint.saves)

	second, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).NormalizeAndStore(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, second.Written, 1)
	require.Equal(t, 1, checkpoint.saves)
}

func TestNormalizeAndStoreCompactsDuplicateCurrentCandidatesBeforeLocalNormalization(t *testing.T) {
	first := newPatternNormTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "a.go", Line: 10, Symbol: "A", Kind: "func"}}
	second := newPatternNormTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "b.go", Line: 20, Symbol: "B", Kind: "func"}}

	result, err := NewService(&mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
	}).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 1)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), result.Written[0].EvidenceLocations)
}

func TestHydrateCurrentNormalizeResultReplacesEvidenceFromUndeclaredSource(t *testing.T) {
	first := newPatternNormTestPattern("first", "First", domain.CategoryBusiness)
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "first.go", Line: 10, Symbol: "First", Kind: "func"}}
	second := newPatternNormTestPattern("second", "Second", domain.CategoryBusiness)
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "second.go", Line: 20, Symbol: "Second", Kind: "func"}}
	result := &Decision{Patterns: []DecisionPattern{{
		ID:        first.ID,
		SourceIDs: []string{first.ID},
	}}}
	normalized := proposalFromDecision(result)

	require.NoError(t, hydrateNormalizeResult(normalized, []domain.Pattern{*first, *second}, nil))
	require.Equal(t, first.EvidenceLocations, normalized.Patterns[0].EvidenceLocations)
}

func TestCoalesceCurrentCandidatesCombinesEvidenceAcrossFocuses(t *testing.T) {
	first := newPatternNormTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "a.go", Line: 10, Symbol: "A", Kind: "func"}}
	second := newPatternNormTestPattern("shared-rule", "Shared Rule", domain.CategoryBusiness)
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "b.go", Line: 20, Symbol: "B", Kind: "func"}}

	coalesced := coalesceCurrentCandidates([]domain.Pattern{*first, *second})

	require.Len(t, coalesced, 1)
	require.ElementsMatch(t, append(first.EvidenceLocations, second.EvidenceLocations...), coalesced[0].EvidenceLocations)
}

func TestNormalizeAndStoreKeepsDistinctCurrentCandidatesForRecall(t *testing.T) {
	first := newPatternNormTestPattern("auth-error-wrap", "Error Wrapping", domain.CategoryError)
	first.SetDescription("Repository errors in auth flow are wrapped with operation context before returning.")
	first.SetRule("When auth repository calls fail, keep the auth operation context in the returned error.")
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/auth/repo.go", Line: 10, Symbol: "LoadAuth", Kind: "func"}}
	second := newPatternNormTestPattern("order-error-wrap", "Error Wrapping", domain.CategoryError)
	second.SetDescription("Repository errors in order flow are wrapped with operation context before returning.")
	second.SetRule("When order repository calls fail, keep the order operation context in the returned error.")
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/order/repo.go", Line: 20, Symbol: "LoadOrder", Kind: "func"}}

	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn:   func(context.Context, *domain.Pattern) error { return nil },
	}

	result, err := NewService(repo).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*first, *second},
	})

	require.NoError(t, err)
	require.Len(t, result.Written, 2)
	require.Equal(t, []string{"auth-error-wrap"}, result.Written[0].MergedFrom)
	require.Equal(t, []string{"order-error-wrap"}, result.Written[1].MergedFrom)
}

func TestNormalizeAndStoreHydratesSourceOwnedFieldsFromCurrentCandidate(t *testing.T) {
	candidate := newPatternNormTestPattern("candidate", "Error Wrapping", domain.CategoryError)
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

	result, err := NewService(repo).NormalizeAndStore(context.Background(), NormalizeRequest{
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

func TestNormalizeAndStoreDoesNotPersistCurrentCandidatesWhenStoreFails(t *testing.T) {
	candidate := newPatternNormTestPattern("candidate", "Error Wrapping", domain.CategoryError)
	var saved bool
	repo := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) { return nil, nil },
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			saved = true
			return errors.New("db closed")
		},
	}

	result, err := NewService(repo).NormalizeAndStore(context.Background(), NormalizeRequest{
		Operation:  OperationLearnCurrent,
		Candidates: []domain.Pattern{*candidate},
	})

	require.ErrorContains(t, err, i18n.Get("PatternNormApplyPatternsFailed"))
	require.Nil(t, result)
	require.True(t, saved)
}

type memoryDecisionCheckpoint struct {
	key      string
	decision *Decision
	saves    int
}

func (c *memoryDecisionCheckpoint) Load(_ context.Context, key string) (*Decision, bool, error) {
	if c.key != key || c.decision == nil {
		return nil, false, nil
	}
	return c.decision, true, nil
}

func (c *memoryDecisionCheckpoint) Save(_ context.Context, key string, result *Decision) error {
	c.key = key
	c.decision = result
	c.saves++
	return nil
}
