package claim

import (
	"fmt"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge/policy"
)

// maxClaimLocations 限制概要断言展示的证据位置数量，避免 reference 首页过长。
const maxClaimLocations = 4

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
		{Title: i18n.GetForLocale(locale, "KnowledgePolicyGroupRuleTitle"), Description: i18n.GetForLocale(locale, "KnowledgePolicyGroupRuleDescription")},
		{Title: i18n.GetForLocale(locale, "KnowledgePolicyGroupSolutionTitle"), Description: i18n.GetForLocale(locale, "KnowledgePolicyGroupSolutionDescription")},
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
		Text:          claimText(pattern),
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

func claimText(pattern domain.Pattern) string {
	return policy.DisplayPatternText(pattern)
}

func usage(strength policy.Strength, locale string) string {
	switch strength {
	case policy.StrengthRule:
		return i18n.GetForLocale(locale, "KnowledgeClaimUsageRule")
	case policy.StrengthSolution:
		return i18n.GetForLocale(locale, "KnowledgeClaimUsageSolution")
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
		add(evidenceLabel(pattern.BusinessMethod.DisplayLocation(), pattern.BusinessMethod.Name, pattern.BusinessMethod.Description))
	}
	for _, location := range pattern.EvidenceLocations {
		add(evidenceLabel(location.DisplayLocation(), location.Symbol, location.Description))
	}
	return result
}

func evidenceLabel(location, symbol, description string) string {
	location = strings.TrimSpace(location)
	symbol = strings.TrimSpace(symbol)
	description = strings.TrimSpace(description)
	if symbol == "" && description == "" {
		return location
	}
	if description == "" {
		return fmt.Sprintf("%s - %s", location, symbol)
	}
	if symbol == "" {
		return fmt.Sprintf("%s - %s", location, description)
	}
	return fmt.Sprintf("%s - %s: %s", location, symbol, description)
}

func strengthIndex(strength policy.Strength) int {
	switch strength {
	case policy.StrengthRule:
		return 0
	case policy.StrengthSolution:
		return 1
	default:
		return 2
	}
}
