package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatternClassificationDetectsHighRiskOperationalBoundary(t *testing.T) {
	pattern := NewPattern("destroy-resource", "Destroy Workspace Resource", CategoryBusiness)
	pattern.SetDescription("Destroy command deletes external resources and updates environment state.")
	pattern.EvidenceLocations = []PatternEvidenceLocation{{Path: "tools/commands/destroy.ts", Symbol: "destroyResource"}}

	require.True(t, IsHighRiskOperationalPattern(*pattern))
}

func TestPatternClassificationDoesNotMatchEnglishTermSubstrings(t *testing.T) {
	highRisk := NewPattern("statement-builder", "Statement Builder", CategoryDatabase)
	highRisk.SetDescription("The statement builder removes empty clauses before compiling queries.")
	highRisk.EvidenceLocations = []PatternEvidenceLocation{{Path: "internal/query/builder.go", Symbol: "BuildStatement"}}

	naming := NewPattern("username-validator", "Username Validator", CategoryNaming)
	naming.SetDescription("Username validation appears across account DTOs.")
	naming.Frequency = 2
	naming.EvidenceLocations = []PatternEvidenceLocation{
		{Path: "src/auth/user.ts", Symbol: "ValidateUsername"},
		{Path: "src/admin/user.ts", Symbol: "ValidateUsername"},
	}

	require.False(t, IsHighRiskOperationalPattern(*highRisk))
	require.False(t, IsRenderableNamingPattern(*naming))
}

func TestRenderableNamingPatternRequiresNamingEvidence(t *testing.T) {
	weak := NewPattern("login-model", "LoginWithPassword Model", CategoryNaming)
	weak.SetDescription("LoginWithPassword appears near model code.")
	weak.EvidenceLocations = []PatternEvidenceLocation{{Path: "src/auth/login.ts", Symbol: "LoginWithPassword"}}

	stable := NewPattern("component-suffix", "Component Suffix Naming", CategoryNaming)
	stable.SetDescription("Interactive components use a component suffix for routeable UI entries.")
	stable.Frequency = 2
	stable.EvidenceLocations = []PatternEvidenceLocation{
		{Path: "src/user/ProfileComponent.tsx", Symbol: "ProfileComponent"},
		{Path: "src/order/OrderComponent.tsx", Symbol: "OrderComponent"},
	}

	require.False(t, IsRenderableNamingPattern(*weak))
	require.True(t, IsRenderableNamingPattern(*stable))
}

func TestRouteableUtilityFunctionDropsTrivialCacheKeys(t *testing.T) {
	cacheKey := UtilityFunction{
		Name:        "BuildCacheKey",
		File:        "src/cache/key.ts",
		Signature:   "function BuildCacheKey(id: string): string",
		Description: "builds a cache key",
	}
	normalizer := UtilityFunction{
		Name:        "NormalizeResourceID",
		File:        "src/resource/id.ts",
		Signature:   "function NormalizeResourceID(id: string): string",
		Description: "normalizes resource identifiers before protocol calls",
	}

	require.False(t, IsRouteableUtilityFunction(cacheKey))
	require.True(t, IsRouteableUtilityFunction(normalizer))
}
