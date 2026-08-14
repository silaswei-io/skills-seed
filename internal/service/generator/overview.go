package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func projectOverviewSummary(profile *domain.ProjectProfile, locale string) string {
	if profile == nil {
		return ""
	}
	if hasBroadProfileEvidence(profile) {
		return learnedCoverageSummary(profile, locale)
	}
	summary := strings.TrimSpace(profile.Summary)
	if summary != "" {
		return summary
	}
	return learnedCoverageSummary(profile, locale)
}

func projectArchitectureSummary(profile *domain.ProjectProfile, locale string) string {
	if profile == nil {
		return ""
	}
	if hasBroadProfileEvidence(profile) {
		if len(profile.Layers) == 0 {
			return ""
		}
		return generatorTextWithParams(locale, "GeneratorOverviewArchitectureSummary", map[string]interface{}{
			"Count": len(profile.Layers),
		})
	}
	architecture := strings.TrimSpace(profile.Architecture)
	if architecture != "" {
		return architecture
	}
	if len(profile.Layers) == 0 {
		return ""
	}
	return generatorTextWithParams(locale, "GeneratorOverviewArchitectureSummary", map[string]interface{}{
		"Count": len(profile.Layers),
	})
}

func hasBroadProfileEvidence(profile *domain.ProjectProfile) bool {
	return profile != nil && (len(profile.KeyModules) > 1 || len(profile.BusinessMethods) > 1 || len(profile.Layers) > 1)
}
func learnedCoverageSummary(profile *domain.ProjectProfile, locale string) string {
	total := learnedResponsibilityTotal(profile)
	if total == 0 {
		return generatorText(locale, "GeneratorOverviewCoverageMissing")
	}
	return generatorTextWithParams(locale, "GeneratorOverviewCoverageTotal", map[string]interface{}{
		"Total": total,
	})
}

func learnedResponsibilityTotal(profile *domain.ProjectProfile) int {
	if profile == nil {
		return 0
	}
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		seen[strings.ToLower(value)] = true
	}
	for _, module := range profile.KeyModules {
		add(stringx.FirstNonBlank(module.Name, module.Path))
	}
	for _, method := range profile.BusinessMethods {
		add(method.Name)
	}
	for _, layer := range profile.Layers {
		add(layer.Name)
	}
	return len(seen)
}
