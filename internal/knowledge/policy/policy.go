package policy

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type Strength string

const (
	StrengthCore        Strength = "core"
	StrengthConvention  Strength = "convention"
	StrengthLocal       Strength = "local"
	StrengthObservation Strength = "observation"
)

type Decision struct {
	Strength Strength
}

func EvaluatePattern(pattern domain.Pattern) Decision {
	evidenceCount := PatternEvidenceCount(pattern)
	if pattern.Confidence >= 0.90 && pattern.Frequency >= 2 && evidenceCount >= 2 {
		return Decision{Strength: StrengthCore}
	}
	if pattern.Confidence >= 0.85 && (pattern.Frequency >= 2 || evidenceCount >= 2) {
		return Decision{Strength: StrengthConvention}
	}
	if pattern.Confidence >= 0.70 && (evidenceCount >= 1 || strings.TrimSpace(pattern.ScopePath) != "") {
		return Decision{Strength: StrengthLocal}
	}
	return Decision{Strength: StrengthObservation}
}

func PatternEvidenceCount(pattern domain.Pattern) int {
	evidenceCount := len(pattern.EvidenceLocations)
	if pattern.BusinessMethod != nil && strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()) != "" {
		evidenceCount++
	}
	if pattern.Metrics.EvidenceCount > evidenceCount {
		evidenceCount = pattern.Metrics.EvidenceCount
	}
	return evidenceCount
}
