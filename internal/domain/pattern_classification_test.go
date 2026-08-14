package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKnowledgeFlagsAreControlledAndCanonical(t *testing.T) {
	flags := MergeKnowledgeFlags(
		[]string{KnowledgeFlagOperationalRisk, KnowledgeFlagOperationalRisk},
		nil,
	)

	require.Equal(t, []string{KnowledgeFlagOperationalRisk}, flags)
	require.True(t, ValidKnowledgeFlags(flags))
	require.False(t, ValidKnowledgeFlags([]string{"invented"}))
}

func TestHighRiskOperationalUsesReviewedFlagOnly(t *testing.T) {
	pattern := NewPattern("resource-boundary", "Resource boundary", CategoryBusiness)
	pattern.SetDescription("Text and paths do not classify the pattern in runtime code.")

	require.False(t, pattern.HighRiskOperational())

	pattern.KnowledgeFlags = []string{KnowledgeFlagOperationalRisk}
	require.True(t, pattern.HighRiskOperational())
}
