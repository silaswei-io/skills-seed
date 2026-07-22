package generator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
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
	const previewLimit = 8
	domains := learnedDomainNames(profile, previewLimit)
	total := learnedDomainTotal(profile)
	switch {
	case len(domains) > 0:
		if total > len(domains) {
			return generatorTextWithParams(locale, "GeneratorOverviewCoveragePreview", map[string]interface{}{
				"Total":   total,
				"Preview": len(domains),
				"Domains": generatorListJoin(locale, domains),
			})
		}
		return generatorTextWithParams(locale, "GeneratorOverviewCoverageFull", map[string]interface{}{
			"Total":   total,
			"Domains": generatorListJoin(locale, domains),
		})
	default:
		return generatorText(locale, "GeneratorOverviewCoverageMissing")
	}
}

func learnedDomainTotal(profile *domain.ProjectProfile) int {
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
		add(firstNonEmptyString(module.Name, module.Path))
	}
	for _, method := range profile.BusinessMethods {
		add(method.Name)
	}
	for _, layer := range profile.Layers {
		add(layer.Name)
	}
	return len(seen)
}

func learnedDomainNames(profile *domain.ProjectProfile, limit int) []string {
	if profile == nil || limit <= 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, limit)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, value)
	}
	for _, module := range profile.KeyModules {
		add(firstNonEmptyString(module.Name, module.Path))
		if len(out) >= limit {
			return out
		}
	}
	for _, method := range profile.BusinessMethods {
		add(method.Name)
		if len(out) >= limit {
			return out
		}
	}
	for _, layer := range profile.Layers {
		add(layer.Name)
		if len(out) >= limit {
			return out
		}
	}
	return out
}
