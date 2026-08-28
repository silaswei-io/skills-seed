package patternnorm

import (
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func hydrateNormalizeResult(result *proposal, candidates, existing []domain.Pattern) error {
	if result == nil {
		return fmt.Errorf("normalization result is nil")
	}
	inputs := indexNormalizationSources(candidates, existing)
	for i := range result.Patterns {
		pattern := &result.Patterns[i]
		var sources []domain.Pattern
		seenSources := make(map[string]struct{}, len(pattern.MergedFrom))
		allowedEvidence := make(map[string]domain.PatternEvidenceLocation)
		for _, sourceID := range pattern.MergedFrom {
			source, ok := inputs[sourceID]
			if !ok {
				return fmt.Errorf("normalized pattern %q references unknown source %q", pattern.ID, sourceID)
			}
			if _, seen := seenSources[source.ID]; seen {
				continue
			}
			seenSources[source.ID] = struct{}{}
			sources = append(sources, source)
			for _, location := range source.EvidenceLocations {
				key := evidenceKey(location)
				if _, exists := allowedEvidence[key]; !exists {
					allowedEvidence[key] = location
				}
			}
		}
		if len(sources) == 0 {
			return fmt.Errorf("normalized pattern %q has no merged sources", pattern.ID)
		}

		pattern.MergedFrom = expandHydratedSources(pattern.MergedFrom, sources)
		sources = prioritizeCurrentSources(pattern.ID, sources)
		hydrateCurrentPatternFields(pattern, sources)
		pattern.KnowledgeFlags = knowledgeFlagsFromSources(sources)
		sourceEvidence := evidenceFromSources(sources)
		pattern.GoodExample, pattern.BadExample = currentExamples(sources)
		pattern.BusinessMethod = firstCurrentBusinessMethod(sources)
		pattern.DevelopmentFocus = firstCurrentDevelopmentFocus(sources)
		hydrateCurrentProvenance(pattern, sources)

		canonicalEvidence := make([]domain.PatternEvidenceLocation, 0, len(pattern.EvidenceLocations))
		seenEvidence := make(map[string]struct{}, len(pattern.EvidenceLocations))
		for _, location := range pattern.EvidenceLocations {
			key := evidenceKey(location)
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
			return fmt.Errorf("normalized pattern %q has no evidence", pattern.ID)
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

func knowledgeFlagsFromSources(sources []domain.Pattern) []string {
	groups := make([][]string, 0, len(sources))
	for _, source := range sources {
		groups = append(groups, source.KnowledgeFlags)
	}
	return domain.MergeKnowledgeFlags(groups...)
}

func hydrateCurrentPatternFields(pattern *domain.Pattern, sources []domain.Pattern) {
	if strings.TrimSpace(pattern.Name) == "" {
		pattern.Name = firstSourceValue(sources, func(source domain.Pattern) string { return source.Name })
	}
	if pattern.Category == "" {
		pattern.Category = domain.Category(firstSourceValue(sources, func(source domain.Pattern) string { return string(source.Category) }))
	}
	if strings.TrimSpace(pattern.Description) == "" {
		pattern.Description = firstSourceValue(sources, func(source domain.Pattern) string { return source.Description })
	}
	if strings.TrimSpace(pattern.Rule) == "" {
		pattern.Rule = firstSourceValue(sources, func(source domain.Pattern) string { return source.Rule })
	}
}

func firstSourceValue(sources []domain.Pattern, value func(domain.Pattern) string) string {
	for _, source := range sources {
		if text := strings.TrimSpace(value(source)); text != "" {
			return text
		}
	}
	return ""
}

func expandHydratedSources(sourceIDs []string, sources []domain.Pattern) []string {
	expanded := append([]string(nil), sourceIDs...)
	for _, source := range sources {
		expanded = append(expanded, source.MergedFrom...)
	}
	return stringx.UniqueNonEmpty(expanded)
}

func evidenceFromSources(sources []domain.Pattern) []domain.PatternEvidenceLocation {
	var out []domain.PatternEvidenceLocation
	seen := make(map[string]struct{})
	for _, source := range sources {
		for _, location := range source.EvidenceLocations {
			key := evidenceKey(location)
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
	var goodExample, badExample string
	for _, source := range sources {
		if goodExample == "" {
			goodExample = strings.TrimSpace(source.GoodExample)
		}
		if badExample == "" {
			badExample = strings.TrimSpace(source.BadExample)
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

func firstCurrentDevelopmentFocus(sources []domain.Pattern) *domain.DevelopmentFocus {
	for _, source := range sources {
		if source.DevelopmentFocus != nil {
			return source.DevelopmentFocus.Clone()
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

func evidenceKey(location domain.PatternEvidenceLocation) string {
	return strings.TrimSpace(location.Path) + "|" + fmt.Sprint(location.Line) + "|" + strings.TrimSpace(location.Symbol) + "|" + strings.TrimSpace(location.Kind)
}
