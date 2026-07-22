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
	for _, item := range assessment.Result.Dropped {
		if _, ok := candidateIDs[item.ID]; !ok {
			assessment.IgnoredDroppedIDs = append(assessment.IgnoredDroppedIDs, item.ID)
			continue
		}
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
	sort.Strings(assessment.Coverage.MissingIDs)
	return assessment
}
