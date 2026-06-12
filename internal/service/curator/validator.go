package curator

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
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

func validateCurateResult(result *agent.CuratePatternsResult, candidates, existing []domain.Pattern) error {
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
	outputIDs := make(map[string]struct{}, len(result.Patterns))
	for i := range result.Patterns {
		pattern := &result.Patterns[i]
		pattern.Category = string(domain.NormalizePatternCategory(domain.Category(pattern.Category)))
		if strings.TrimSpace(pattern.ID) == "" {
			return fmt.Errorf("curated pattern has empty id")
		}
		if _, exists := outputIDs[pattern.ID]; exists {
			return fmt.Errorf("duplicate curated pattern id %q", pattern.ID)
		}
		outputIDs[pattern.ID] = struct{}{}
		if !domain.IsValidPatternCategory(domain.Category(pattern.Category)) {
			return fmt.Errorf("curated pattern %q has invalid category %q", pattern.ID, pattern.Category)
		}
		if strings.TrimSpace(pattern.Name) == "" {
			return fmt.Errorf("curated pattern %q has empty name", pattern.ID)
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
			}
		}
	}

	droppedIDs := make(map[string]struct{}, len(result.Dropped))
	for _, dropped := range result.Dropped {
		if _, ok := candidateIDs[dropped.ID]; !ok {
			return fmt.Errorf("dropped pattern references unknown candidate id %q", dropped.ID)
		}
		if _, exists := droppedIDs[dropped.ID]; exists {
			return fmt.Errorf("duplicate dropped candidate id %q", dropped.ID)
		}
		droppedIDs[dropped.ID] = struct{}{}
		coveredCandidates[dropped.ID] = struct{}{}
	}

	for id := range candidateIDs {
		if _, ok := coveredCandidates[id]; !ok {
			return fmt.Errorf("candidate pattern %q is not covered by curated result", id)
		}
	}
	if result.Summary.TotalCandidates != 0 && result.Summary.TotalCandidates != len(candidates) {
		return fmt.Errorf("summary total_candidates mismatch")
	}
	if result.Summary.TotalExisting != 0 && result.Summary.TotalExisting != len(existing) {
		return fmt.Errorf("summary total_existing mismatch")
	}
	if result.Summary.TotalWritten != 0 && result.Summary.TotalWritten != len(result.Patterns) {
		return fmt.Errorf("summary total_written mismatch")
	}
	if result.Summary.TotalDropped != 0 && result.Summary.TotalDropped != len(result.Dropped) {
		return fmt.Errorf("summary total_dropped mismatch")
	}
	return nil
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
