package curator

import (
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type curationCoverage struct {
	CandidateCount int
	MissingIDs     []string
}

func (c curationCoverage) MissingCount() int {
	return len(c.MissingIDs)
}

func (c curationCoverage) MissingRatio() float64 {
	if c.CandidateCount == 0 {
		return 0
	}
	return float64(c.MissingCount()) / float64(c.CandidateCount)
}

type curationAssessment struct {
	Result                       *proposal
	Coverage                     curationCoverage
	IgnoredDroppedIDs            []string
	IgnoredConflictingDroppedIDs []string
	IgnoredMergedFromIDs         []string
	IgnoredPatternIDs            []string
	ResolvedOwnershipIDs         []string
}

func assessCuration(result *proposal, candidates, existing []domain.Pattern) curationAssessment {
	assessment := curationAssessment{Result: cloneProposal(result)}
	if assessment.Result == nil {
		return assessment
	}

	candidateIDs := patternIDSet(candidates)
	allowedIDs := patternIDSet(existing)
	for id := range candidateIDs {
		allowedIDs[id] = struct{}{}
	}

	dropped := assessment.Result.Dropped[:0]
	seenDropped := make(map[string]struct{}, len(assessment.Result.Dropped))
	for _, item := range assessment.Result.Dropped {
		if _, ok := candidateIDs[item.ID]; !ok {
			assessment.IgnoredDroppedIDs = append(assessment.IgnoredDroppedIDs, item.ID)
			continue
		}
		if _, duplicate := seenDropped[item.ID]; duplicate {
			assessment.IgnoredDroppedIDs = append(assessment.IgnoredDroppedIDs, item.ID)
			continue
		}
		seenDropped[item.ID] = struct{}{}
		dropped = append(dropped, item)
	}
	assessment.Result.Dropped = dropped

	unknownMergedFrom := make(map[string]struct{})
	patterns := assessment.Result.Patterns[:0]
	for i := range assessment.Result.Patterns {
		pattern := &assessment.Result.Patterns[i]
		mergedFrom := pattern.MergedFrom[:0]
		for _, id := range pattern.MergedFrom {
			if _, ok := allowedIDs[id]; !ok {
				unknownMergedFrom[id] = struct{}{}
				continue
			}
			mergedFrom = append(mergedFrom, id)
		}
		pattern.MergedFrom = mergedFrom
		if len(pattern.MergedFrom) == 0 {
			if _, ok := allowedIDs[pattern.ID]; ok {
				pattern.MergedFrom = []string{pattern.ID}
			} else {
				assessment.IgnoredPatternIDs = append(assessment.IgnoredPatternIDs, pattern.ID)
				continue
			}
		}
		patterns = append(patterns, *pattern)
	}
	assessment.Result.Patterns = patterns
	assessment.Result.Patterns, assessment.ResolvedOwnershipIDs = normalizeSourceOwnership(assessment.Result.Patterns)

	covered := make(map[string]struct{}, len(candidateIDs))
	for _, pattern := range assessment.Result.Patterns {
		for _, id := range pattern.MergedFrom {
			if _, ok := candidateIDs[id]; ok {
				covered[id] = struct{}{}
			}
		}
	}
	for _, item := range assessment.Result.Dropped {
		covered[item.ID] = struct{}{}
	}

	assessment.Coverage.CandidateCount = len(candidateIDs)
	for id := range candidateIDs {
		if _, ok := covered[id]; !ok {
			assessment.Coverage.MissingIDs = append(assessment.Coverage.MissingIDs, id)
		}
	}
	for id := range unknownMergedFrom {
		assessment.IgnoredMergedFromIDs = append(assessment.IgnoredMergedFromIDs, id)
	}
	sort.Strings(assessment.IgnoredDroppedIDs)
	sort.Strings(assessment.IgnoredMergedFromIDs)
	sort.Strings(assessment.IgnoredPatternIDs)
	sort.Strings(assessment.ResolvedOwnershipIDs)
	sort.Strings(assessment.Coverage.MissingIDs)
	return assessment
}

// normalizeSourceOwnership 将 AI 的输出归一为 source -> canonical pattern 的单一归属。
func normalizeSourceOwnership(patterns []domain.Pattern) ([]domain.Pattern, []string) {
	claims := make(map[string][]int)
	for i := range patterns {
		patterns[i].MergedFrom = uniqueStrings(patterns[i].MergedFrom)
		for _, sourceID := range patterns[i].MergedFrom {
			claims[sourceID] = append(claims[sourceID], i)
		}
	}

	removed := make([]map[string]struct{}, len(patterns))
	var resolved []string
	for sourceID, owners := range claims {
		if len(owners) < 2 {
			continue
		}
		winner, hasWinner := exactSourceOwner(patterns, sourceID, owners)
		for _, owner := range owners {
			if hasWinner && owner == winner {
				continue
			}
			if removed[owner] == nil {
				removed[owner] = make(map[string]struct{})
			}
			removed[owner][sourceID] = struct{}{}
		}
		resolved = append(resolved, sourceID)
	}

	out := make([]domain.Pattern, 0, len(patterns))
	for i := range patterns {
		if len(removed[i]) > 0 {
			kept := patterns[i].MergedFrom[:0]
			for _, sourceID := range patterns[i].MergedFrom {
				if _, drop := removed[i][sourceID]; !drop {
					kept = append(kept, sourceID)
				}
			}
			patterns[i].MergedFrom = kept
		}
		if len(patterns[i].MergedFrom) == 0 {
			continue
		}
		patterns[i].Merged = len(patterns[i].MergedFrom) > 1
		out = append(out, patterns[i])
	}
	return out, resolved
}

func exactSourceOwner(patterns []domain.Pattern, sourceID string, owners []int) (int, bool) {
	for _, owner := range owners {
		if patterns[owner].ID == sourceID {
			return owner, true
		}
	}
	return 0, false
}
