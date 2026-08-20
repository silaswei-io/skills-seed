package generator

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildDevelopmentFocusesProjectsPrimaryRoute(t *testing.T) {
	first := domain.NewPattern("issue", "Issue", domain.CategoryBusiness)
	first.DevelopmentFocus = &domain.DevelopmentFocus{
		ID:         "certificate-lifecycle",
		Name:       "Certificate Lifecycle",
		RouteTerms: []string{"issue"},
		EntryPaths: []string{"internal/certificate/issue.go"},
	}
	first.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/certificate/service.go", Line: 10}}
	second := domain.NewPattern("status", "Status", domain.CategoryAPI)
	second.DevelopmentFocus = &domain.DevelopmentFocus{
		ID:         "certificate-lifecycle",
		Name:       "Certificate Lifecycle",
		RouteTerms: []string{"status"},
		EntryPaths: []string{"internal/certificate/status.go"},
	}
	second.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "desc/certificate.api", Line: 4}}

	views := buildDevelopmentFocuses([]domain.Pattern{*first, *second})

	require.Len(t, views, 1)
	require.Equal(t, []string{"issue", "status"}, views[0].RouteTerms)
	require.Equal(t, "internal/certificate/issue.go", views[0].PrimaryPath)
	require.Equal(t, "./references/patterns/business/certificate-lifecycle.md", views[0].ReferencePath)
}
