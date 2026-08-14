package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanProjectProfileNormalizesBusinessMethodLocation(t *testing.T) {
	profile := &ProjectProfile{
		BusinessMethods: []BusinessMethod{
			{
				Name:         "CreateOrder",
				CodeLocation: CodeLocation{CurrentLocation: "internal/service/order.go:42"},
			},
		},
	}

	cleaned := CleanProjectProfile(profile)

	assert.Len(t, cleaned.BusinessMethods, 1)
	method := cleaned.BusinessMethods[0]
	assert.Equal(t, "internal/service/order.go:42", method.CodeLocation.HistoricalLocation)
	assert.Equal(t, "internal/service/order.go:42", method.CodeLocation.CurrentLocation)
	assert.Equal(t, CodeLocationStatusValid, method.CodeLocation.Status)
	assert.False(t, method.CodeLocation.CreatedAt.IsZero())
	assert.False(t, method.CodeLocation.UpdatedAt.IsZero())
}

func TestCleanProjectProfileDeduplicatesModulesAndFiltersPlaceholders(t *testing.T) {
	profile := &ProjectProfile{
		KeyModules: []ModuleInfo{
			{
				Name:             " home ",
				Path:             " internal/handler/home ",
				Description:      "first",
				Responsibilities: []string{" route ", "TODO confirm"},
				KeyMethods:       []string{"List", "List"},
			},
			{
				Name:             "home",
				Path:             "internal/handler/home",
				Description:      "second",
				Responsibilities: []string{"handler"},
				Dependencies:     []string{"svc", "svc"},
				KeyMethods:       []string{"Create"},
			},
			{
				Name: "event bus",
				Path: "",
			},
		},
	}

	cleaned := CleanProjectProfile(profile)

	require.Len(t, cleaned.KeyModules, 1)
	module := cleaned.KeyModules[0]
	assert.Equal(t, "home", module.Name)
	assert.Equal(t, "internal/handler/home", module.Path)
	assert.Equal(t, "first", module.Description)
	assert.Equal(t, []string{"route", "handler"}, module.Responsibilities)
	assert.Equal(t, []string{"svc"}, module.Dependencies)
	assert.Equal(t, []string{"List", "Create"}, module.KeyMethods)
}

func TestCleanProjectProfileDropsUnconfirmedProfileEntries(t *testing.T) {
	profile := &ProjectProfile{
		KeyModules: []ModuleInfo{
			{Name: "Event Bus Modules", Path: "unconfirmed"},
			{Name: "Workflow Execution Engines", Path: "待确认"},
			{Name: "service", Path: "internal/service"},
		},
		CommonUtils: []UtilityFunction{
			{Name: "SearchHint", File: "unconfirmed", Signature: "func SearchHint()"},
			{Name: "Confirmed", File: "internal/shared/confirmed.go", Signature: "func Confirmed()"},
		},
	}

	cleaned := CleanProjectProfile(profile)

	require.Len(t, cleaned.KeyModules, 1)
	assert.Equal(t, "service", cleaned.KeyModules[0].Name)
	require.Len(t, cleaned.CommonUtils, 1)
	assert.Equal(t, "Confirmed", cleaned.CommonUtils[0].Name)
}

func TestCleanProjectProfileOnlyCollapsesEquivalentPatternText(t *testing.T) {
	profile := &ProjectProfile{
		FrameworkPatterns: []string{
			"Resolve input before invoking the reusable entry",
			"resolve input before invoking the reusable entry",
			"Preserve the existing output boundary",
		},
		ConfigPatterns: []string{
			"Load configuration through the project entry",
			"load configuration through the project entry",
			"Keep defaults at the integration boundary",
		},
	}

	cleaned := CleanProjectProfile(profile)

	assert.Len(t, cleaned.FrameworkPatterns, 2)
	assert.Len(t, cleaned.ConfigPatterns, 2)
}

func TestNewProjectSpecFromProfilePreservesAuthoritativeProfileData(t *testing.T) {
	spec := NewProjectSpecFromProfile(&ProjectProfile{
		ProjectName: "demo",
		Language:    "unknown",
		EngineeringRules: []EngineeringRule{
			{Title: "Boundary", Rule: "Keep changes within the project boundary", Source: "AGENTS.md", Evidence: []string{"AGENTS.md"}},
		},
	}, WorkspaceProjectOverride{})

	require.NotNil(t, spec)
	require.Len(t, spec.EngineeringRules, 1)
	assert.Equal(t, "AGENTS.md", spec.EngineeringRules[0].Source)
}
