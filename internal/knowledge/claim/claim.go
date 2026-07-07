package claim

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge/policy"
)

// maxClaimLocations 限制概要断言展示的证据位置数量，避免 reference 首页过长。
const maxClaimLocations = 3

type Claim struct {
	Title         string
	Text          string
	Usage         string
	Strength      policy.Strength
	Category      domain.Category
	Confidence    float64
	Frequency     int
	EvidenceCount int
	Source        domain.Source
	Scope         string
	Locations     []string
}

type Group struct {
	Title       string
	Description string
	Claims      []Claim
}

func Groups(patterns []domain.Pattern, locale string) []Group {
	if len(patterns) == 0 {
		return nil
	}
	groups := []Group{
		{Title: i18n.GetForLocale(locale, "KnowledgePolicyGroupCoreTitle"), Description: i18n.GetForLocale(locale, "KnowledgePolicyGroupCoreDescription")},
		{Title: i18n.GetForLocale(locale, "KnowledgePolicyGroupConventionTitle"), Description: i18n.GetForLocale(locale, "KnowledgePolicyGroupConventionDescription")},
		{Title: i18n.GetForLocale(locale, "KnowledgePolicyGroupLocalTitle"), Description: i18n.GetForLocale(locale, "KnowledgePolicyGroupLocalDescription")},
		{Title: i18n.GetForLocale(locale, "KnowledgePolicyGroupObservationTitle"), Description: i18n.GetForLocale(locale, "KnowledgePolicyGroupObservationDescription")},
	}
	for _, pattern := range patterns {
		item := FromPattern(pattern, locale)
		groups[strengthIndex(item.Strength)].Claims = append(groups[strengthIndex(item.Strength)].Claims, item)
	}
	result := make([]Group, 0, len(groups))
	for _, group := range groups {
		if len(group.Claims) == 0 {
			continue
		}
		sort.SliceStable(group.Claims, func(i, j int) bool {
			left := group.Claims[i]
			right := group.Claims[j]
			if left.Confidence != right.Confidence {
				return left.Confidence > right.Confidence
			}
			if left.Frequency != right.Frequency {
				return left.Frequency > right.Frequency
			}
			if left.EvidenceCount != right.EvidenceCount {
				return left.EvidenceCount > right.EvidenceCount
			}
			return left.Title < right.Title
		})
		result = append(result, group)
	}
	return result
}

func FromPattern(pattern domain.Pattern, locale string) Claim {
	decision := policy.EvaluatePattern(pattern)
	return Claim{
		Title:         strings.TrimSpace(pattern.Name),
		Text:          claimText(pattern, locale),
		Usage:         usage(decision.Strength, locale),
		Strength:      decision.Strength,
		Category:      pattern.Category,
		Confidence:    pattern.Confidence,
		Frequency:     pattern.Frequency,
		EvidenceCount: policy.PatternEvidenceCount(pattern),
		Source:        pattern.Source,
		Scope:         scope(pattern),
		Locations:     locations(pattern),
	}
}

func claimText(pattern domain.Pattern, locale string) string {
	return policy.DisplayPatternText(pattern, locale)
}

func usage(strength policy.Strength, locale string) string {
	switch strength {
	case policy.StrengthCore:
		return i18n.GetForLocale(locale, "KnowledgeClaimUsageCore")
	case policy.StrengthConvention:
		return i18n.GetForLocale(locale, "KnowledgeClaimUsageConvention")
	case policy.StrengthLocal:
		return i18n.GetForLocale(locale, "KnowledgeClaimUsageLocal")
	default:
		return i18n.GetForLocale(locale, "KnowledgeClaimUsageObservation")
	}
}

func scope(pattern domain.Pattern) string {
	if value := strings.TrimSpace(pattern.ScopePath); value != "" {
		return value
	}
	if pattern.BusinessMethod != nil {
		if value := strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()); value != "" {
			return value
		}
	}
	for _, location := range pattern.EvidenceLocations {
		if value := strings.TrimSpace(location.DisplayLocation()); value != "" {
			return value
		}
	}
	return ""
}

func locations(pattern domain.Pattern) []string {
	seen := map[string]bool{}
	result := make([]string, 0, maxClaimLocations)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] || len(result) >= maxClaimLocations {
			return
		}
		seen[value] = true
		result = append(result, value)
	}
	if pattern.BusinessMethod != nil {
		add(pattern.BusinessMethod.DisplayLocation())
	}
	for _, location := range pattern.EvidenceLocations {
		add(location.DisplayLocation())
	}
	return result
}

func strengthIndex(strength policy.Strength) int {
	switch strength {
	case policy.StrengthCore:
		return 0
	case policy.StrengthConvention:
		return 1
	case policy.StrengthLocal:
		return 2
	default:
		return 3
	}
}
