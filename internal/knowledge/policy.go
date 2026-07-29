package knowledge

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

type ClaimStrength string

const (
	ClaimStrengthRule        ClaimStrength = "rule"
	ClaimStrengthSolution    ClaimStrength = "solution"
	ClaimStrengthObservation ClaimStrength = "observation"
)

type ClaimDecision struct {
	Strength ClaimStrength
}

func EvaluatePatternClaim(pattern domain.Pattern) ClaimDecision {
	evidenceCount := PatternEvidenceCount(pattern)
	if pattern.AllowsHardConstraint() {
		return ClaimDecision{Strength: ClaimStrengthRule}
	}
	if evidenceCount >= 1 {
		return ClaimDecision{Strength: ClaimStrengthSolution}
	}
	return ClaimDecision{Strength: ClaimStrengthObservation}
}

// DisplayPatternText 对权威来源返回规则，对学习来源返回待复核的观察描述。
func DisplayPatternText(pattern domain.Pattern) string {
	if !pattern.AllowsHardConstraint() {
		return stringx.FirstNonBlank(pattern.Description, pattern.Rule, pattern.Name)
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
