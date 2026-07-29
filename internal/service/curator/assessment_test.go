package curator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAssessCurationDoesNotMutateAgentResult(t *testing.T) {
	result := &agent.CuratePatternsResult{
		Patterns: []agent.CuratedPattern{{
			ID:        "curated",
			SourceIDs: []string{"candidate", "invented-source"},
		}},
		Dropped: []agent.CuratedDrop{
			{ID: "candidate", ReasonCode: agent.CuratedDropExactDuplicate, Reason: "duplicate"},
			{ID: "invented-drop", ReasonCode: agent.CuratedDropUnsupportedEvidence, Reason: "invalid"},
		},
	}
	candidates := []domain.Pattern{{ID: "candidate"}}

	assessment := assessCuration(proposalFromAgent(result), candidates, nil)

	require.Equal(t, []string{"candidate", "invented-source"}, result.Patterns[0].SourceIDs)
	require.Len(t, result.Dropped, 2)
	require.Equal(t, []string{"candidate"}, assessment.Result.Patterns[0].MergedFrom)
	require.Equal(t, []Drop{{ID: "candidate", ReasonCode: agent.CuratedDropExactDuplicate, Reason: "duplicate"}}, assessment.Result.Dropped)
}

func TestAssessCurationDropsOrphanProposalAndInfersMatchingSource(t *testing.T) {
	result := &proposal{Patterns: []domain.Pattern{
		{ID: "candidate"},
		{ID: "invented", MergedFrom: []string{"invented-source"}},
	}}

	assessment := assessCuration(result, []domain.Pattern{{ID: "candidate"}}, nil)

	require.Len(t, assessment.Result.Patterns, 1)
	require.Equal(t, []string{"candidate"}, assessment.Result.Patterns[0].MergedFrom)
	require.Equal(t, []string{"invented"}, assessment.IgnoredPatternIDs)
	require.Empty(t, assessment.Coverage.MissingIDs)
}
