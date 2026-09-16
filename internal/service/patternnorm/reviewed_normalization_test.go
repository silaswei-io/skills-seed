package patternnorm

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/maintained"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCurrentNormalizationPreservesReviewedSingletonText(t *testing.T) {
	candidate := currentPattern("verified", 0.91, "verified.go")
	candidate.Description = "May leave partial progress after failure."
	candidate.Rule = "Inspect partial progress before retrying."
	result, err := finalizeAndValidateCurrentNormalization(&proposal{Patterns: []domain.Pattern{{
		ID: "renamed", Name: "Invented guarantee", Category: domain.Category("invented"),
		Description: "Always atomic.", Rule: "Retry freely.", Confidence: 1,
		MergedFrom: []string{candidate.ID},
	}}}, []domain.Pattern{candidate}, nil)
	require.NoError(t, err)
	require.Len(t, result.Patterns, 1)
	got := result.Patterns[0]
	require.Equal(t, candidate.ID, got.ID)
	require.Equal(t, candidate.Name, got.Name)
	require.Equal(t, candidate.Category, got.Category)
	require.Equal(t, candidate.Description, got.Description)
	require.Equal(t, candidate.Rule, got.Rule)
	require.Equal(t, candidate.Confidence, got.Confidence)
}

func TestCurrentNormalizationSkipsAgentForIsolatedKnowledge(t *testing.T) {
	candidate := currentPattern("independent", 0.9, "independent.go")
	called := false
	service := NewServiceWithNormalizer(&mocks.MockPatternRepository{}, normalizePatternsFunc(func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
		called = true
		return nil, nil
	}))
	result, err := service.NormalizeAndStore(context.Background(), NormalizeRequest{Operation: OperationLearnCurrent, Candidates: []domain.Pattern{candidate}})
	require.NoError(t, err)
	require.False(t, called)
	require.Len(t, result.Written, 1)
	require.Equal(t, candidate.Rule, result.Written[0].Rule)
}

func TestNormalizationCheckpointBindsExistingKnowledgeAndGuidance(t *testing.T) {
	candidates := []domain.Pattern{currentPattern("current", 0.9, "current.go")}
	related := []domain.Pattern{currentPattern("existing", 0.9, "existing.go")}
	key, err := normalizationDecisionKey(candidates, related, "", maintained.Snapshot{})
	require.NoError(t, err)
	related[0].Rule = "Changed existing boundary"
	updated, err := normalizationDecisionKey(candidates, related, "", maintained.Snapshot{})
	require.NoError(t, err)
	require.NotEqual(t, key, updated)
	withContext, err := normalizationDecisionKey(candidates, related, "Changed intent", maintained.Snapshot{})
	require.NoError(t, err)
	require.NotEqual(t, updated, withContext)
	withGuidance, err := normalizationDecisionKey(candidates, related, "", maintained.Snapshot{Rules: []domain.Rule{{ID: "rule", Content: "New maintained rule"}}})
	require.NoError(t, err)
	require.NotEqual(t, updated, withGuidance)
}
