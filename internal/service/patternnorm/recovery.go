package patternnorm

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func recoverCurrentNormalization(assessment normalizationAssessment, candidates []domain.Pattern) *proposal {
	result := cloneProposal(assessment.Result)
	if result == nil {
		result = &proposal{}
	}
	mergeRecoveredPatterns(result, keepCurrentCandidates(patternsByID(candidates, assessment.Coverage.MissingIDs)).Patterns)
	return result
}

func recoverRecallProtectedDrops(result *proposal, candidates []domain.Pattern) (*proposal, []string) {
	result = cloneProposal(result)
	if result == nil || len(result.Dropped) == 0 {
		return result, nil
	}
	byID := make(map[string]domain.Pattern, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	protected := make([]domain.Pattern, 0)
	dropped := result.Dropped[:0]
	for _, item := range result.Dropped {
		candidate, ok := byID[item.ID]
		if ok && shouldRecoverDroppedCurrentCandidate(candidate, item) {
			protected = append(protected, candidate)
			continue
		}
		dropped = append(dropped, item)
	}
	result.Dropped = dropped
	mergeRecoveredPatterns(result, keepCurrentCandidates(protected).Patterns)
	ids := make([]string, 0, len(protected))
	for _, candidate := range protected {
		ids = append(ids, candidate.ID)
	}
	sort.Strings(ids)
	return result, ids
}

func shouldRecoverDroppedCurrentCandidate(candidate domain.Pattern, dropped Drop) bool {
	if !currentCandidateHasReusableEvidence(candidate) {
		return false
	}
	return dropped.ReasonCode == DropOverfilteredSourceBacked || dropped.ReasonCode == DropNoRouteableValue
}

func currentCandidateHasReusableEvidence(candidate domain.Pattern) bool {
	if !candidate.IsValid() || hasPlaceholderExample(candidate.GoodExample) || len(candidate.EvidenceLocations) == 0 {
		return false
	}
	return len([]rune(strings.TrimSpace(candidate.Name+" "+candidate.Description+" "+candidate.Rule))) >= 48
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
			result.Patterns[index].MergedFrom = stringx.UniqueNonEmpty(append(result.Patterns[index].MergedFrom, pattern.MergedFrom...))
			result.Patterns[index].Merged = len(result.Patterns[index].MergedFrom) > 1
			continue
		}
		indexByID[pattern.ID] = len(result.Patterns)
		result.Patterns = append(result.Patterns, pattern)
	}
}
