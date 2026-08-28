package patternnorm

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestIndexNormalizationSourcesResolvesUniqueHistoricalLineage(t *testing.T) {
	existing := domain.Pattern{
		ID:         "heartbeat-driven-node-registration-and-sync",
		MergedFrom: []string{"heartbeat-driven-node-registration-and-sync", "heartbeat-driven-node-lifecycle"},
	}

	indexed := indexNormalizationSources(nil, []domain.Pattern{existing})

	require.Equal(t, existing.ID, indexed["heartbeat-driven-node-lifecycle"].ID)
}

func TestIndexNormalizationSourcesRejectsAmbiguousHistoricalLineage(t *testing.T) {
	first := domain.Pattern{ID: "first", MergedFrom: []string{"shared-lineage"}}
	second := domain.Pattern{ID: "second", MergedFrom: []string{"shared-lineage"}}

	indexed := indexNormalizationSources(nil, []domain.Pattern{first, second})

	require.NotContains(t, indexed, "shared-lineage")
}
