package curator

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func validateCandidates(candidates []domain.Pattern) []domain.Pattern {
	valid := make([]domain.Pattern, 0, len(candidates))
	for _, candidate := range candidates {
		pattern := normalizeCandidate(candidate)
		if pattern.IsValid() && !hasPlaceholderExample(pattern.GoodExample) {
			valid = append(valid, pattern)
		}
	}
	return valid
}

func validateCurrentCandidates(candidates []domain.Pattern) []domain.Pattern {
	valid := make([]domain.Pattern, 0, len(candidates))
	for _, candidate := range candidates {
		if len(candidate.EvidenceLocations) > 0 {
			valid = append(valid, candidate)
		}
	}
	return valid
}

func coalesceCurrentCandidates(candidates []domain.Pattern) []domain.Pattern {
	coalesced := make([]domain.Pattern, 0, len(candidates))
	indexByID := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		index, exists := indexByID[candidate.ID]
		if !exists {
			indexByID[candidate.ID] = len(coalesced)
			coalesced = append(coalesced, candidate)
			continue
		}

		previous := coalesced[index]
		merged := mergeKeepingBestPattern(previous, candidate)
		merged.ProjectID = commonPairValue(previous.ProjectID, candidate.ProjectID)
		merged.ScopePath = commonPairValue(previous.ScopePath, candidate.ScopePath)
		merged.WorkspaceRole = commonPairValue(previous.WorkspaceRole, candidate.WorkspaceRole)
		merged.AnalysisUnitID = commonPairValue(previous.AnalysisUnitID, candidate.AnalysisUnitID)
		merged.AnalysisUnitName = commonPairValue(previous.AnalysisUnitName, candidate.AnalysisUnitName)
		coalesced[index] = merged
	}
	return coalesced
}

func commonPairValue(left, right string) string {
	left = strings.TrimSpace(left)
	if left == strings.TrimSpace(right) {
		return left
	}
	return ""
}

func validateCurateResultForOperation(operation Operation, result *proposal, candidates, existing []domain.Pattern) error {
	if operation == OperationLearnCurrent {
		if err := hydrateCurateResult(result, candidates, existing); err != nil {
			return err
		}
	}
	return validateCurateResult(result, candidates, existing)
}

func validateCurateResult(result *proposal, candidates, existing []domain.Pattern) error {
	if result == nil {
		return fmt.Errorf("curation result is nil")
	}

	candidateIDs := patternIDSet(candidates)
	existingIDs := patternIDSet(existing)
	allIDs := make(map[string]struct{}, len(candidateIDs)+len(existingIDs))
	for id := range candidateIDs {
		allIDs[id] = struct{}{}
	}
	for id := range existingIDs {
		allIDs[id] = struct{}{}
	}

	coveredCandidates := make(map[string]struct{}, len(candidateIDs))
	mergedCandidates := make(map[string]struct{}, len(candidateIDs))
	outputIDs := make(map[string]struct{}, len(result.Patterns))
	for i := range result.Patterns {
		pattern := &result.Patterns[i]
		pattern.Category = domain.NormalizePatternCategory(pattern.Category)
		if strings.TrimSpace(pattern.ID) == "" {
			return fmt.Errorf("curated pattern has empty id")
		}
		if _, exists := outputIDs[pattern.ID]; exists {
			return fmt.Errorf("duplicate curated pattern id %q", pattern.ID)
		}
		outputIDs[pattern.ID] = struct{}{}
		if !domain.IsValidPatternCategory(pattern.Category) {
			return fmt.Errorf("curated pattern %q has invalid category %q", pattern.ID, pattern.Category)
		}
		if strings.TrimSpace(pattern.Name) == "" {
			return fmt.Errorf("curated pattern %q has empty name", pattern.ID)
		}
		if strings.TrimSpace(pattern.Rule) == "" {
			return fmt.Errorf("curated pattern %q has empty rule", pattern.ID)
		}
		if pattern.Confidence < 0 || pattern.Confidence > 1 {
			return fmt.Errorf("curated pattern %q has confidence outside [0,1]", pattern.ID)
		}
		if hasPlaceholderExample(pattern.GoodExample) {
			return fmt.Errorf("curated pattern %q has placeholder good example", pattern.ID)
		}
		for _, id := range pattern.MergedFrom {
			if _, ok := allIDs[id]; !ok {
				return fmt.Errorf("curated pattern %q references unknown merged_from id %q", pattern.ID, id)
			}
			if _, ok := candidateIDs[id]; ok {
				coveredCandidates[id] = struct{}{}
				mergedCandidates[id] = struct{}{}
			}
		}
	}

	droppedIDs := make(map[string]struct{}, len(result.Dropped))
	for _, dropped := range result.Dropped {
		if _, ok := candidateIDs[dropped.ID]; !ok {
			return fmt.Errorf("dropped pattern id %q is not a current candidate id; dropped may only reference current candidates", dropped.ID)
		}
		if _, exists := droppedIDs[dropped.ID]; exists {
			return fmt.Errorf("duplicate dropped candidate id %q", dropped.ID)
		}
		if _, merged := mergedCandidates[dropped.ID]; merged {
			return fmt.Errorf("candidate pattern %q is both merged and dropped", dropped.ID)
		}
		droppedIDs[dropped.ID] = struct{}{}
		coveredCandidates[dropped.ID] = struct{}{}
	}

	for id := range candidateIDs {
		if _, ok := coveredCandidates[id]; !ok {
			return fmt.Errorf("candidate pattern %q is not covered by curated result", id)
		}
	}
	return validateProposalOwnership(result)
}

func patternIDSet(patterns []domain.Pattern) map[string]struct{} {
	result := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		if pattern.ID == "" {
			continue
		}
		result[pattern.ID] = struct{}{}
	}
	return result
}

func normalizeCandidate(pattern domain.Pattern) domain.Pattern {
	pattern.ID = strings.TrimSpace(pattern.ID)
	pattern.Name = strings.TrimSpace(pattern.Name)
	pattern.Description = strings.TrimSpace(pattern.Description)
	pattern.Rule = strings.TrimSpace(pattern.Rule)
	pattern.GoodExample = strings.TrimSpace(pattern.GoodExample)
	pattern.BadExample = strings.TrimSpace(pattern.BadExample)
	pattern.ProjectID = strings.TrimSpace(pattern.ProjectID)
	pattern.ScopePath = strings.TrimSpace(pattern.ScopePath)
	pattern.WorkspaceRole = strings.TrimSpace(pattern.WorkspaceRole)
	pattern.AnalysisUnitID = strings.TrimSpace(pattern.AnalysisUnitID)
	pattern.AnalysisUnitName = strings.TrimSpace(pattern.AnalysisUnitName)
	pattern.Category = domain.NormalizePatternCategory(pattern.Category)
	if pattern.Source == "" {
		pattern.Source = domain.SourceLearned
	}
	if pattern.Frequency <= 0 {
		pattern.Frequency = 1
	}
	return pattern
}

func hasPlaceholderExample(example string) bool {
	trimmed := strings.TrimSpace(example)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "/* ... */") || strings.Contains(trimmed, "{ ... }") {
		return true
	}
	lines := strings.Split(trimmed, "\n")
	placeholderLines := 0
	for _, line := range lines {
		normalized := strings.TrimSpace(line)
		if normalized == "..." || normalized == "// ..." || normalized == "# ..." {
			placeholderLines++
		}
	}
	return placeholderLines > 0 && placeholderLines == len(lines)
}
