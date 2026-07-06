package learn

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

type currentChangeProfile string

const (
	currentChangeProfileInitial  currentChangeProfile = "initial"
	currentChangeProfileMicro    currentChangeProfile = "micro"
	currentChangeProfileMinor    currentChangeProfile = "minor"
	currentChangeProfileNormal   currentChangeProfile = "normal"
	currentChangeProfileRefactor currentChangeProfile = "refactor"
)

func classifyCurrentChangeProfile(changes *incrementalFileChanges) currentChangeProfile {
	if changes == nil {
		return currentChangeProfileNormal
	}
	changed := len(changes.AddedOrModified)
	deleted := len(changes.Deleted)
	if changed > 0 && deleted == 0 && len(changes.Unchanged) == 0 {
		return currentChangeProfileInitial
	}
	if deleted > 0 {
		return currentChangeProfileRefactor
	}
	switch {
	case changed <= 2:
		return currentChangeProfileMicro
	case changed <= 15:
		return currentChangeProfileMinor
	default:
		return currentChangeProfileNormal
	}
}

func (r *learnCurrentProjectRun) admitLearnedPatterns(patterns []domain.Pattern) []domain.Pattern {
	budget := r.cont.ConfigRepo.GetCurrentLearningConfig().Budget
	limit := admissionLimitForProfile(r.changeProfile, budget)
	if budget.MaxPatternsPerUnit > 0 && (limit == 0 || budget.MaxPatternsPerUnit < limit) {
		limit = budget.MaxPatternsPerUnit
	}
	if budget.MaxPatternsPerRun > 0 {
		remaining := budget.MaxPatternsPerRun - r.savedCount
		if remaining < 0 {
			remaining = 0
		}
		if limit == 0 || remaining < limit {
			limit = remaining
		}
	}
	if limit <= 0 {
		return nil
	}

	admitted := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		pattern.Status = domain.PatternStatusActive
		if !candidatePassesAdmission(pattern, budget, r.changeProfile) {
			continue
		}
		admitted = append(admitted, pattern)
	}
	sort.SliceStable(admitted, func(i, j int) bool {
		return admissionScore(admitted[i]) > admissionScore(admitted[j])
	})
	if len(admitted) > limit {
		admitted = admitted[:limit]
	}
	return admitted
}

func admissionLimitForProfile(profile currentChangeProfile, budget config.LearningBudget) int {
	switch profile {
	case currentChangeProfileMicro:
		return budget.MicroChangeNewPatterns
	case currentChangeProfileMinor, currentChangeProfileRefactor:
		return budget.MinorChangeNewPatterns
	default:
		return budget.MaxPatternsPerUnit
	}
}

func candidatePassesAdmission(pattern domain.Pattern, budget config.LearningBudget, profile currentChangeProfile) bool {
	if budget.MinConfidence > 0 && pattern.Confidence > 0 && pattern.Confidence < budget.MinConfidence {
		return false
	}
	if budget.RequireRouteableEvidence && profile != currentChangeProfileInitial && !hasRouteableEvidence(pattern) {
		return false
	}
	return strings.TrimSpace(pattern.ID) != ""
}

func hasRouteableEvidence(pattern domain.Pattern) bool {
	if len(pattern.EvidenceLocations) > 0 {
		for _, loc := range pattern.EvidenceLocations {
			if strings.TrimSpace(loc.Path) != "" || strings.TrimSpace(loc.Symbol) != "" {
				return true
			}
		}
	}
	if pattern.BusinessMethod != nil {
		return strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()) != "" || strings.TrimSpace(pattern.BusinessMethod.Name) != ""
	}
	return false
}

func admissionScore(pattern domain.Pattern) float64 {
	score := pattern.Confidence
	if score == 0 {
		score = pattern.Metrics.EffectiveScore
	}
	score += float64(pattern.Frequency) * 0.01
	if len(pattern.EvidenceLocations) > 0 {
		score += 0.05
	}
	if pattern.BusinessMethod != nil {
		score += 0.05
	}
	if domain.NormalizePatternCategory(pattern.Category) == domain.CategoryBusiness {
		score += 0.03
	}
	return score
}
