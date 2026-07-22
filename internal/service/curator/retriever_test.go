package curator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSameScopeUsesOnlySemanticWorkspaceScope(t *testing.T) {
	left := domain.Pattern{AnalysisUnitID: "unit-a"}
	right := domain.Pattern{AnalysisUnitID: "unit-a"}
	require.False(t, sameScope(left, right))

	left.ScopePath = "services/ca-admin"
	right.ScopePath = "services/ca-admin"
	right.AnalysisUnitID = "unit-b"
	require.True(t, sameScope(left, right))

	right.ScopePath = "services/other"
	require.False(t, sameScope(left, right))
}

func TestTokensMatchReorderedChinesePhrases(t *testing.T) {
	left := tokens("用户状态校验")
	right := tokens("校验用户状态")

	require.Greater(t, jaccard(left, right), 0.3)
}
