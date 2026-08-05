package patternnorm

import (
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/knowledge/patternview"
)

type scoredPattern struct {
	pattern domain.Pattern
	score   float64
}

func retrieveRelatedPatterns(candidates, existing []domain.Pattern, limitPerCandidate int) retrievalResult {
	if limitPerCandidate <= 0 {
		limitPerCandidate = relatedPatternsPerCandidate
	}
	byID := make(map[string]domain.Pattern)
	byCandidate := make(map[string][]string, len(candidates))

	for _, candidate := range candidates {
		scored := make([]scoredPattern, 0, len(existing))
		for _, pattern := range existing {
			score := patternview.Similarity(candidate, pattern)
			if score <= 0 {
				continue
			}
			scored = append(scored, scoredPattern{pattern: pattern, score: score})
		}
		sort.SliceStable(scored, func(i, j int) bool {
			if scored[i].score == scored[j].score {
				return scored[i].pattern.ID < scored[j].pattern.ID
			}
			return scored[i].score > scored[j].score
		})
		if len(scored) > limitPerCandidate {
			scored = scored[:limitPerCandidate]
		}
		for _, item := range scored {
			byID[item.pattern.ID] = item.pattern
			byCandidate[candidate.ID] = append(byCandidate[candidate.ID], item.pattern.ID)
		}
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
