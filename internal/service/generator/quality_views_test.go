package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPatternImportanceGroupsDoNotPromoteSingleEvidenceBusinessPatternToCore(t *testing.T) {
	local := domain.NewPattern("local-business", "Local Business Rule", domain.CategoryBusiness)
	local.Confidence = 0.89
	local.Frequency = 1
	local.SetRule("Review the nearby implementation before changing this local flow.")
	local.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/local.go", Line: 10, Symbol: "Run", Kind: "function", Confidence: 0.89},
	}

	core := domain.NewPattern("stable-business", "Stable Business Rule", domain.CategoryBusiness)
	core.Confidence = 0.93
	core.Frequency = 2
	core.SetRule("Apply this stable business rule when changing the related capability.")
	core.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/a.go", Line: 10, Symbol: "RunA", Kind: "function", Confidence: 0.92},
		{Path: "internal/service/b.go", Line: 20, Symbol: "RunB", Kind: "function", Confidence: 0.91},
	}

	groups := patternImportanceGroups([]domain.Pattern{*local, *core}, "zh-CN")

	require.Len(t, groups, 2)
	require.Equal(t, "核心开发路径", groups[0].Title)
	require.Len(t, groups[0].Patterns, 1)
	require.Equal(t, "stable-business", groups[0].Patterns[0].ID)
	require.Equal(t, "局部模块经验", groups[1].Title)
	require.Len(t, groups[1].Patterns, 1)
	require.Equal(t, "local-business", groups[1].Patterns[0].ID)
}
