package workspace

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestEffectiveParallelism(t *testing.T) {
	require.Equal(t, 1, EffectiveParallelism(domain.ModeProject, 0, 5))
	require.Equal(t, 3, EffectiveParallelism(domain.ModeWorkspace, 0, 3))
	require.Equal(t, defaultWorkspaceParallelismCap, EffectiveParallelism(domain.ModeWorkspace, 0, 20))
	require.Equal(t, 9, EffectiveParallelism(domain.ModeWorkspace, 9, 20))
}
