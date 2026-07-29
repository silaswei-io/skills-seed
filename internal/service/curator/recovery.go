package curator

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

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

func recoverRecallProtectedDrops(result *proposal, candidates, existing []domain.Pattern) (*proposal, []string) {
	result = cloneProposal(result)
	if result == nil || len(result.Dropped) == 0 {
		return result, nil
	}

	candidatesByID := make(map[string]domain.Pattern, len(candidates))
	for _, candidate := range candidates {
		candidatesByID[candidate.ID] = candidate
	}

	protected := make([]domain.Pattern, 0)
	dropped := result.Dropped[:0]
	for _, item := range result.Dropped {
		candidate, ok := candidatesByID[item.ID]
		if ok && shouldRecoverDroppedCurrentCandidate(candidate, item) {
			protected = append(protected, candidate)
			continue
		}
		dropped = append(dropped, item)
	}
	result.Dropped = dropped
	if len(protected) == 0 {
		return result, nil
	}

	known := append(append([]domain.Pattern(nil), existing...), result.Patterns...)
	recovered := deterministicCurate(protected, known)
	mergeRecoveredPatterns(result, recovered.Patterns)
	recoveredIDs := make([]string, 0, len(protected))
	for _, pattern := range protected {
		recoveredIDs = append(recoveredIDs, pattern.ID)
	}
	sort.Strings(recoveredIDs)
	return result, recoveredIDs
}

func shouldRecoverDroppedCurrentCandidate(candidate domain.Pattern, dropped Drop) bool {
	if !currentCandidateHasReusableEvidence(candidate) {
		return false
	}
	return dropped.ReasonCode == agent.CuratedDropOverfilteredSourceBacked
}

func currentCandidateHasReusableEvidence(candidate domain.Pattern) bool {
	if !candidate.IsValid() || hasPlaceholderExample(candidate.GoodExample) || len(candidate.EvidenceLocations) == 0 {
		return false
	}
	text := strings.TrimSpace(candidate.Name + " " + candidate.Description + " " + candidate.Rule)
	return len([]rune(text)) >= 48
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
			current.MergedFrom = stringx.UniqueNonEmpty(append(current.MergedFrom, pattern.MergedFrom...))
			current.Merged = len(current.MergedFrom) > 1
			continue
		}
		indexByID[pattern.ID] = len(result.Patterns)
		result.Patterns = append(result.Patterns, pattern)
	}
}
