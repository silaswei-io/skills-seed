package policy

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type Strength string

const (
	StrengthRule        Strength = "rule"
	StrengthSolution    Strength = "solution"
	StrengthObservation Strength = "observation"
)

type Decision struct {
	Strength Strength
}

func EvaluatePattern(pattern domain.Pattern) Decision {
	evidenceCount := PatternEvidenceCount(pattern)
	if pattern.AllowsHardConstraint() {
		return Decision{Strength: StrengthRule}
	}
	if evidenceCount >= 1 {
		return Decision{Strength: StrengthSolution}
	}
	return Decision{Strength: StrengthObservation}
}

// DisplayPatternText 对权威来源返回规则，对学习来源返回待复核的观察描述。
func DisplayPatternText(pattern domain.Pattern) string {
	if !pattern.AllowsHardConstraint() {
		return firstNonEmpty(pattern.Description, pattern.Rule, pattern.Name)
	}
	text := strings.TrimSpace(pattern.Rule)
	if text == "" {
		text = strings.TrimSpace(pattern.Description)
	}
	if text == "" {
		text = strings.TrimSpace(pattern.Name)
	}
	return text
}

func PatternEvidenceCount(pattern domain.Pattern) int {
	evidenceCount := domain.PatternEvidenceFileCount(pattern.EvidenceLocations)
	if evidenceCount == 0 && pattern.BusinessMethod != nil && strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()) != "" {
		evidenceCount = 1
	}
	return evidenceCount
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
