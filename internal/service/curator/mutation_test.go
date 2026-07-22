package curator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildMutationPlanDoesNotDeleteDroppedCandidateDuringLearning(t *testing.T) {
	existing := []domain.Pattern{{ID: "same-id"}}
	dropped := []Drop{{ID: "same-id", Reason: "not reusable"}}

	plan := buildMutationPlan(nil, dropped, existing, storeCandidates)

	require.Empty(t, plan.Mutation.DeleteIDs)
}

func TestBuildMutationPlanDeletesDroppedExistingPatternDuringCompaction(t *testing.T) {
	existing := []domain.Pattern{{ID: "obsolete"}}
	dropped := []Drop{{ID: "obsolete", Reason: "duplicate"}}

	plan := buildMutationPlan(nil, dropped, existing, compactLibrary)

	require.Equal(t, []string{"obsolete"}, plan.Mutation.DeleteIDs)
}
