package agent

import (
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/stretchr/testify/require"
)

func TestRuntimeLabelFromEvidenceFocusPrefersID(t *testing.T) {
	label := RuntimeLabelFromEvidenceFocus("auth-login", "认证登录")

	require.Equal(t, "focus-auth-login", label)
}

func TestRuntimeLabelFromEvidenceFocusFallsBackToBoundedName(t *testing.T) {
	label := RuntimeLabelFromEvidenceFocus("", strings.Repeat("payment-", 20))

	require.True(t, strings.HasPrefix(label, "focus-payment"))
	require.LessOrEqual(t, len([]rune(strings.TrimPrefix(label, "focus-"))), runtimefiles.MaxSafePartLength)
}

func TestRuntimeLabelFromEvidenceFocusSkipsNonASCIIName(t *testing.T) {
	label := RuntimeLabelFromEvidenceFocus("", "认证登录")

	require.Empty(t, label)
}

func TestAnalyzeCurrentCodebaseBatchOperationIncludesRuntimeLabel(t *testing.T) {
	operation := AnalyzeCurrentCodebaseBatchOperation(&AnalyzeCurrentCodebaseBatchRequest{RuntimeLabel: "focus-auth-login"})

	require.Equal(t, "AnalyzeCurrentCodebaseBatch/focus-auth-login", operation)
	require.Equal(t, "focus-auth-login", OperationLabel(operation))
}

func TestRuntimeSlugKeepsDistinctLabel(t *testing.T) {
	slug := RuntimeSlug("learning-pack-analyze", "focus-auth")

	require.Equal(t, "learning-pack-analyze-focus-auth", slug)
}

func TestRuntimeSlugTrimsOverlappingBatchLabel(t *testing.T) {
	slug := RuntimeSlug("learning-pack-analyze-batch", "batch-008")

	require.Equal(t, "learning-pack-analyze-batch-008", slug)
}
