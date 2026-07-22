package curator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestValidateProposalOwnershipRejectsSourceUsedByMultipleOutputs(t *testing.T) {
	result := &proposal{Patterns: []domain.Pattern{
		{ID: "first", MergedFrom: []string{"candidate"}},
		{ID: "second", MergedFrom: []string{"candidate"}},
	}}

	err := validateProposalOwnership(result)

	require.ErrorContains(t, err, "consumed by both")
}

func TestValidateProposalOwnershipRejectsOutputThatIsDropped(t *testing.T) {
	result := &proposal{
		Patterns: []domain.Pattern{{ID: "candidate", MergedFrom: []string{"candidate"}}},
		Dropped:  []Drop{{ID: "candidate", Reason: "noise"}},
	}

	err := validateProposalOwnership(result)

	require.ErrorContains(t, err, "both an output and dropped")
}
