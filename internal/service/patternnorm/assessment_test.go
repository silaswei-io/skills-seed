package patternnorm

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAssessNormalizationDoesNotMutateAgentResult(t *testing.T) {
	result := &Decision{
		Patterns: []DecisionPattern{{
			ID:        "normalized",
			SourceIDs: []string{"candidate", "invented-source"},
		}},
		Dropped: []Drop{
			{ID: "candidate", ReasonCode: DropExactDuplicate, Reason: "duplicate"},
			{ID: "invented-drop", ReasonCode: DropUnsupportedEvidence, Reason: "invalid"},
		},
	}
	candidates := []domain.Pattern{{ID: "candidate"}}

	assessment := assessNormalization(proposalFromDecision(result), candidates, nil)

	require.Equal(t, []string{"candidate", "invented-source"}, result.Patterns[0].SourceIDs)
	require.Len(t, result.Dropped, 2)
	require.Equal(t, []string{"candidate"}, assessment.Result.Patterns[0].MergedFrom)
	require.Equal(t, []Drop{{ID: "candidate", ReasonCode: DropExactDuplicate, Reason: "duplicate"}}, assessment.Result.Dropped)
}

func TestAssessNormalizationDropsOrphanProposalAndInfersMatchingSource(t *testing.T) {
	result := &proposal{Patterns: []domain.Pattern{
		{ID: "candidate"},
		{ID: "invented", MergedFrom: []string{"invented-source"}},
	}}

	assessment := assessNormalization(result, []domain.Pattern{{ID: "candidate"}}, nil)

	require.Len(t, assessment.Result.Patterns, 1)
	require.Equal(t, []string{"candidate"}, assessment.Result.Patterns[0].MergedFrom)
	require.Equal(t, []string{"invented"}, assessment.IgnoredPatternIDs)
	require.Empty(t, assessment.Coverage.MissingIDs)
}
