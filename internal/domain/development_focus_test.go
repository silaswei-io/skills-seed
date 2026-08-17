package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDevelopmentFocusFromEvidenceFocusPreservesStableNavigationFields(t *testing.T) {
	focus := DevelopmentFocusFromEvidenceFocus(EvidenceFocus{
		ID:         "certificate-lifecycle",
		Name:       "Certificate Lifecycle",
		RouteTerms: []string{"certificate", "issue", "certificate"},
		EntryPaths: []string{"internal/certificate/lifecycle.go", "internal/certificate/lifecycle.go"},
	})

	require.Equal(t, &DevelopmentFocus{
		ID:         "certificate-lifecycle",
		Name:       "Certificate Lifecycle",
		RouteTerms: []string{"certificate", "issue"},
		EntryPaths: []string{"internal/certificate/lifecycle.go"},
	}, focus)
}

func TestDevelopmentFocusFromEvidenceFocusRejectsUnsafeID(t *testing.T) {
	focus := DevelopmentFocusFromEvidenceFocus(EvidenceFocus{ID: "../unsafe", Name: "Unsafe"})

	require.Nil(t, focus)
}

func TestDevelopmentFocusFromEvidenceFocusExcludesCoverageOnlyFocus(t *testing.T) {
	focus := DevelopmentFocusFromEvidenceFocus(EvidenceFocus{
		ID:      "unassigned-evidence",
		Name:    "Unassigned evidence",
		Purpose: EvidenceFocusPurposeCoverage,
	})

	require.Nil(t, focus)
}
