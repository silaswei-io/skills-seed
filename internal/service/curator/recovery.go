package curator

import "github.com/silaswei-io/skills-seed/internal/domain"

func recoverCurrentCuration(assessment curationAssessment, candidates, existing []domain.Pattern) *proposal {
	result := cloneProposal(assessment.Result)
	if result == nil {
		result = &proposal{}
	}
	missing := patternsByID(candidates, assessment.Coverage.MissingIDs)
	if len(missing) == 0 {
		return result
	}

	known := append(append([]domain.Pattern(nil), existing...), result.Patterns...)
	recovered := deterministicCurate(missing, known)
	mergeRecoveredPatterns(result, recovered.Patterns)
	return result
}

func patternsByID(patterns []domain.Pattern, ids []string) []domain.Pattern {
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	out := make([]domain.Pattern, 0, len(ids))
	for _, pattern := range patterns {
		if _, ok := wanted[pattern.ID]; ok {
			out = append(out, pattern)
		}
	}
	return out
}

func mergeRecoveredPatterns(result *proposal, recovered []domain.Pattern) {
	indexByID := make(map[string]int, len(result.Patterns))
	for i := range result.Patterns {
		indexByID[result.Patterns[i].ID] = i
	}
	for _, pattern := range recovered {
		if index, ok := indexByID[pattern.ID]; ok {
			current := &result.Patterns[index]
			current.MergedFrom = uniqueStrings(append(current.MergedFrom, pattern.MergedFrom...))
			current.Merged = len(current.MergedFrom) > 1
			continue
		}
		indexByID[pattern.ID] = len(result.Patterns)
		result.Patterns = append(result.Patterns, pattern)
	}
}
