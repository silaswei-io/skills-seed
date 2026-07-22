package claim

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/policy"
	"github.com/stretchr/testify/require"
)

func TestGroupsSeparateUserRulesAndReusableSolutions(t *testing.T) {
	solution := domain.NewPattern("existing-capability", "Existing Capability", domain.CategoryBusiness)
	solution.Source = domain.SourceLearnedCurrent
	solution.SetDescription("The existing entry implements the capability within a verified boundary.")
	solution.SetRule("Inspect and prefer ExistingEntry; extend only when its boundary does not fit.")
	solution.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "src/capability.ext", Line: 10, Symbol: "ExistingEntry", Kind: "function", Confidence: 0.89},
	}

	rule := domain.NewPattern("maintained-rule", "Maintained Rule", domain.CategoryStructure)
	rule.Source = domain.SourceUserDefined
	rule.SetRule("Preserve the maintained project boundary.")
	rule.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "src/boundary.ext", Line: 5, Symbol: "Boundary", Kind: "type", Confidence: 0.92},
	}

	groups := Groups([]domain.Pattern{*solution, *rule}, "zh-CN")

	require.Len(t, groups, 2)
	require.Equal(t, "用户规则", groups[0].Title)
	require.Len(t, groups[0].Claims, 1)
	require.Equal(t, policy.StrengthRule, groups[0].Claims[0].Strength)
	require.Equal(t, rule.Rule, groups[0].Claims[0].Text)

	require.Equal(t, "可复用解决方案", groups[1].Title)
	require.Len(t, groups[1].Claims, 1)
	require.Equal(t, policy.StrengthSolution, groups[1].Claims[0].Strength)
	require.Equal(t, solution.Description, groups[1].Claims[0].Text)
	require.Equal(t, []string{"src/capability.ext:10 - ExistingEntry"}, groups[1].Claims[0].Locations)
}

func TestFromPatternKeepsUnverifiedContentAsObservation(t *testing.T) {
	pattern := domain.NewPattern("unverified", "Unverified", domain.CategoryAPI)
	pattern.Description = "A possible local behavior."
	pattern.Rule = "Prefer a presumed entry."
	pattern.ScopePath = "src/component"

	item := FromPattern(*pattern, "en-US")

	require.Equal(t, policy.StrengthObservation, item.Strength)
	require.Equal(t, pattern.Description, item.Text)
	require.Equal(t, "src/component", item.Scope)
	require.NotEmpty(t, item.Usage)
}
