package agent

import (
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/stretchr/testify/require"
)

func TestRuntimeLabelFromAnalysisUnitPrefersID(t *testing.T) {
	label := RuntimeLabelFromAnalysisUnit("auth-login", "认证登录")

	require.Equal(t, "unit-auth-login", label)
}

func TestRuntimeLabelFromAnalysisUnitFallsBackToBoundedName(t *testing.T) {
	label := RuntimeLabelFromAnalysisUnit("", strings.Repeat("payment-", 20))

	require.True(t, strings.HasPrefix(label, "unit-payment"))
	require.LessOrEqual(t, len([]rune(strings.TrimPrefix(label, "unit-"))), runtimefiles.MaxSafePartLength)
}

func TestRuntimeLabelFromAnalysisUnitSkipsNonASCIIName(t *testing.T) {
	label := RuntimeLabelFromAnalysisUnit("", "认证登录")

	require.Empty(t, label)
}

func TestAnalyzeCurrentCodebaseOperationIncludesRuntimeLabel(t *testing.T) {
	operation := AnalyzeCurrentCodebaseOperation(&AnalyzeCurrentCodebaseRequest{RuntimeLabel: "unit-auth-login"})

	require.Equal(t, "AnalyzeCurrentCodebase/unit-auth-login", operation)
	require.Equal(t, "unit-auth-login", OperationLabel(operation))
}

func TestRuntimeSlugKeepsDistinctLabel(t *testing.T) {
	slug := RuntimeSlug("pattern-learn-current", "unit-auth")

	require.Equal(t, "pattern-learn-current-unit-auth", slug)
}

func TestRuntimeSlugTrimsOverlappingBatchLabel(t *testing.T) {
	slug := RuntimeSlug("pattern-learn-current-batch", "batch-008")

	require.Equal(t, "pattern-learn-current-batch-008", slug)
}
