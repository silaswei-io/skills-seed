package curator

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func hydrateCurateResult(result *proposal, candidates, existing []domain.Pattern) error {
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
				}
			}
		}
		if len(sources) == 0 {
			return fmt.Errorf("curated pattern %q has no merged sources", pattern.ID)
		}

		sources = prioritizeCurrentSources(pattern.ID, sources)
		sourceEvidence := evidenceFromSources(sources)
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
		pattern.Frequency = domain.PatternEvidenceFileCount(pattern.EvidenceLocations)
		if pattern.Confidence <= 0 {
			for _, source := range sources {
				if source.Confidence > pattern.Confidence {
					pattern.Confidence = source.Confidence
				}
			}
		}
	}
	return nil
}

func evidenceFromSources(sources []domain.Pattern) []domain.PatternEvidenceLocation {
	var out []domain.PatternEvidenceLocation
	seen := make(map[string]struct{})
	for _, source := range sources {
		for _, location := range source.EvidenceLocations {
			key := curateEvidenceKey(location)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, location)
		}
	}
	return out
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

func hydrateCurrentProvenance(pattern *domain.Pattern, sources []domain.Pattern) {
	if len(sources) == 0 {
		return
	}
	pattern.Source = sources[0].Source
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
