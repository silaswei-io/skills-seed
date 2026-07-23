package analyzer

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestStructuralProviderUsesCodeGraphForAuto(t *testing.T) {
	provider := newStructuralProvider(config.StructuralConfig{Provider: config.StructuralProviderAuto})

	_, ok := provider.(*codeGraphProvider)
	require.True(t, ok)
}

func TestStructuralProviderUsesTreeSitterOnlyWhenExplicit(t *testing.T) {
	provider := newStructuralProvider(config.StructuralConfig{Provider: config.StructuralProviderTreeSitter})

	_, ok := provider.(*treesitterCollector)
	require.True(t, ok)
}
