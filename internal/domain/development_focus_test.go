package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDevelopmentFocusFromEvidenceFocusPreservesStableNavigationFields(t *testing.T) {
	focus := DevelopmentFocusFromEvidenceFocus(EvidenceFocus{
		ID:          "certificate-lifecycle",
		Name:        "Certificate Lifecycle",
		RouteTerms:  []string{"certificate", "issue", "certificate"},
		EntryPaths:  []string{"internal/certificate/lifecycle.go", "internal/certificate/lifecycle.go"},
		ScopeReason: "状态流转与签发入口共同决定证书生命周期边界",
		Attributes:  []string{"stateful", "cross_module"},
		RiskSignals: []string{"external effect"},
	})

	require.Equal(t, &DevelopmentFocus{
		ID:          "certificate-lifecycle",
		Name:        "Certificate Lifecycle",
		RouteTerms:  []string{"certificate", "issue"},
		EntryPaths:  []string{"internal/certificate/lifecycle.go"},
		ScopeReason: "状态流转与签发入口共同决定证书生命周期边界",
		Attributes:  []string{"stateful", "cross_module"},
		RiskSignals: []string{"external effect"},
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
