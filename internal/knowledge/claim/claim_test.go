package claim

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/policy"
	"github.com/stretchr/testify/require"
)

func TestGroupsDoNotPromoteSingleEvidenceBusinessPatternToCore(t *testing.T) {
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

	groups := Groups([]domain.Pattern{*local, *core}, "zh-CN")

	require.Len(t, groups, 2)
	require.Equal(t, "核心开发路径", groups[0].Title)
	require.Len(t, groups[0].Claims, 1)
	require.Equal(t, "Stable Business Rule", groups[0].Claims[0].Title)
	require.Equal(t, policy.StrengthCore, groups[0].Claims[0].Strength)
	require.Equal(t, "Apply this stable business rule when changing the related capability.", groups[0].Claims[0].Text)
	require.Equal(t, []string{"internal/service/a.go:10", "internal/service/b.go:20"}, groups[0].Claims[0].Locations)

	require.Equal(t, "局部模块经验", groups[1].Title)
	require.Len(t, groups[1].Claims, 1)
	require.Equal(t, "Local Business Rule", groups[1].Claims[0].Title)
	require.Equal(t, policy.StrengthLocal, groups[1].Claims[0].Strength)
	require.Equal(t, "internal/service/local.go:10", groups[1].Claims[0].Scope)
}

func TestFromPatternUsesRuleDescriptionNameFallbackAndLimitsLocations(t *testing.T) {
	pattern := domain.NewPattern("fallback", "Fallback Name", domain.CategoryAPI)
	pattern.Confidence = 0.8
	pattern.Frequency = 1
	pattern.Description = "Fallback description."
	pattern.ScopePath = "internal/api"
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/api/a.go", Line: 1},
		{Path: "internal/api/b.go", Line: 2},
		{Path: "internal/api/a.go", Line: 1},
		{Path: "internal/api/c.go", Line: 3},
		{Path: "internal/api/d.go", Line: 4},
	}

	item := FromPattern(*pattern, "en-US")

	require.Equal(t, "Fallback description.", item.Text)
	require.Equal(t, "internal/api", item.Scope)
	require.Equal(t, []string{"internal/api/a.go:1", "internal/api/b.go:2", "internal/api/c.go:3"}, item.Locations)
	require.NotEmpty(t, item.Usage)
}
