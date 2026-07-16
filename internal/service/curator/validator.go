package curator

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

type sanitizeCurateResultReport struct {
	IgnoredDroppedIDs []string
}

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

func validateCurateResultForOperation(operation string, result *agent.CuratePatternsResult, candidates, existing []domain.Pattern) error {
	if operation == OperationLearnCurrent {
		if err := hydrateCurrentCurateResult(result, candidates, existing); err != nil {
			return err
		}
	}
	return validateCurateResult(result, candidates, existing)
}

func sanitizeCurateResult(result *agent.CuratePatternsResult, candidates []domain.Pattern) sanitizeCurateResultReport {
	if result == nil {
		return sanitizeCurateResultReport{}
	}

	candidateIDs := patternIDSet(candidates)
	dropped := result.Dropped[:0]
	report := sanitizeCurateResultReport{}
	for _, item := range result.Dropped {
		if _, ok := candidateIDs[item.ID]; !ok {
			report.IgnoredDroppedIDs = append(report.IgnoredDroppedIDs, item.ID)
			continue
		}
		dropped = append(dropped, item)
	}
	result.Dropped = dropped
	return report
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
		droppedIDs[dropped.ID] = struct{}{}
		coveredCandidates[dropped.ID] = struct{}{}
	}

	for id := range candidateIDs {
		if _, ok := coveredCandidates[id]; !ok {
			return fmt.Errorf("candidate pattern %q is not covered by curated result", id)
		}
	}
	return nil
}

func hydrateCurrentCurateResult(result *agent.CuratePatternsResult, candidates, existing []domain.Pattern) error {
	if result == nil {
		return fmt.Errorf("curation result is nil")
	}
	inputs := make(map[string][]domain.Pattern, len(candidates)+len(existing))
	for _, pattern := range append(append([]domain.Pattern(nil), candidates...), existing...) {
		inputs[pattern.ID] = append(inputs[pattern.ID], pattern)
	}
	for i := range result.Patterns {
		pattern := &result.Patterns[i]
		var sources []domain.Pattern
		allowedEvidence := make(map[string]domain.PatternEvidenceLocation)
		var sourceEvidence []domain.PatternEvidenceLocation
		for _, sourceID := range pattern.MergedFrom {
			mergedSources, ok := inputs[sourceID]
			if !ok {
				return fmt.Errorf("curated pattern %q references unknown source %q", pattern.ID, sourceID)
			}
			for _, source := range mergedSources {
				sources = append(sources, source)
				for _, location := range source.EvidenceLocations {
					key := curateEvidenceKey(location)
					if _, exists := allowedEvidence[key]; exists {
						continue
					}
					allowedEvidence[key] = location
					sourceEvidence = append(sourceEvidence, location)
				}
			}
		}
		if len(sources) == 0 {
			return fmt.Errorf("curated pattern %q has no merged sources", pattern.ID)
		}

		sources = prioritizeCurrentSources(pattern.ID, sources)
		pattern.GoodExample, pattern.BadExample = currentExamples(sources)
		pattern.BusinessMethod = firstCurrentBusinessMethod(sources)
		hydrateCurrentProvenance(pattern, sources)

		canonicalEvidence := make([]domain.PatternEvidenceLocation, 0, len(pattern.EvidenceLocations))
		seenEvidence := make(map[string]struct{}, len(pattern.EvidenceLocations))
		for _, location := range pattern.EvidenceLocations {
			key := curateEvidenceKey(location)
			canonical, ok := allowedEvidence[key]
			if !ok {
				continue
			}
			if _, exists := seenEvidence[key]; exists {
				continue
			}
			seenEvidence[key] = struct{}{}
			canonicalEvidence = append(canonicalEvidence, canonical)
		}
		if len(canonicalEvidence) == 0 {
			canonicalEvidence = sourceEvidence
		}
		if len(canonicalEvidence) == 0 {
			return fmt.Errorf("curated pattern %q has no evidence", pattern.ID)
		}
		pattern.EvidenceLocations = canonicalEvidence
		pattern.Frequency, pattern.Confidence = domain.PatternEvidenceQuality(pattern.EvidenceLocations)
	}
	return nil
}

func prioritizeCurrentSources(patternID string, sources []domain.Pattern) []domain.Pattern {
	ordered := make([]domain.Pattern, 0, len(sources))
	for _, source := range sources {
		if source.ID == patternID {
			ordered = append(ordered, source)
		}
	}
	for _, source := range sources {
		if source.ID != patternID {
			ordered = append(ordered, source)
		}
	}
	return ordered
}

func currentExamples(sources []domain.Pattern) (string, string) {
	var goodExample string
	var badExample string
	for _, source := range sources {
		if goodExample == "" {
			goodExample = strings.TrimSpace(source.GoodExample)
		}
		if badExample == "" {
			badExample = strings.TrimSpace(source.BadExample)
		}
		if goodExample != "" && badExample != "" {
			break
		}
	}
	return goodExample, badExample
}

func firstCurrentBusinessMethod(sources []domain.Pattern) *domain.BusinessMethod {
	for _, source := range sources {
		if source.BusinessMethod != nil {
			return source.BusinessMethod
		}
	}
	return nil
}

func hydrateCurrentProvenance(pattern *agent.CuratedPattern, sources []domain.Pattern) {
	if len(sources) == 0 {
		return
	}
	pattern.Source = string(sources[0].Source)
	pattern.ProjectID = commonSourceValue(sources, func(source domain.Pattern) string { return source.ProjectID })
	pattern.ScopePath = commonSourceValue(sources, func(source domain.Pattern) string { return source.ScopePath })
	pattern.WorkspaceRole = commonSourceValue(sources, func(source domain.Pattern) string { return source.WorkspaceRole })
	pattern.AnalysisUnitID = commonSourceValue(sources, func(source domain.Pattern) string { return source.AnalysisUnitID })
	pattern.AnalysisUnitName = commonSourceValue(sources, func(source domain.Pattern) string { return source.AnalysisUnitName })
}

func commonSourceValue(sources []domain.Pattern, value func(domain.Pattern) string) string {
	if len(sources) == 0 {
		return ""
	}
	common := strings.TrimSpace(value(sources[0]))
	for _, source := range sources[1:] {
		if strings.TrimSpace(value(source)) != common {
			return ""
		}
	}
	return common
}

func curateEvidenceKey(location domain.PatternEvidenceLocation) string {
	return strings.TrimSpace(location.Path) + "|" + fmt.Sprint(location.Line) + "|" + strings.TrimSpace(location.Symbol) + "|" + strings.TrimSpace(location.Kind)
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
