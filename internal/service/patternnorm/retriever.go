package patternnorm

import (
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/patternview"
)

func retrieveRelatedPatterns(candidates, existing []domain.Pattern) retrievalResult {
	byID := make(map[string]domain.Pattern)
	byCandidate := make(map[string][]string, len(candidates))

	for _, candidate := range candidates {
		for _, pattern := range existing {
			if !patternview.Relate(candidate, pattern).Exists() {
				continue
			}
			byID[pattern.ID] = pattern
			byCandidate[candidate.ID] = append(byCandidate[candidate.ID], pattern.ID)
		}
		sort.Strings(byCandidate[candidate.ID])
	}

	related := make([]domain.Pattern, 0, len(byID))
	for _, pattern := range byID {
		related = append(related, pattern)
	}
	sort.SliceStable(related, func(i, j int) bool {
		return related[i].ID < related[j].ID
	})
	return retrievalResult{
		related:             related,
		existingByCandidate: byCandidate,
	}
}
