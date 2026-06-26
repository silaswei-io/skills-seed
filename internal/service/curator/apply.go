package curator

import (
	"context"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

func curatedToDomain(pattern agent.CuratedPattern) domain.Pattern {
	source := domain.Source(pattern.Source)
	if source == "" {
		source = domain.SourceLearned
	}
	frequency := pattern.Frequency
	if frequency <= 0 {
		frequency = 1
	}
	now := time.Now()
	result := domain.Pattern{
		ID:                pattern.ID,
		Name:              pattern.Name,
		Category:          domain.NormalizePatternCategory(domain.Category(pattern.Category)),
		Description:       pattern.Description,
		GoodExample:       pattern.GoodExample,
		BadExample:        pattern.BadExample,
		Rule:              pattern.Rule,
		Confidence:        pattern.Confidence,
		Frequency:         frequency,
		Source:            source,
		Merged:            len(pattern.MergedFrom) > 1,
		MergedFrom:        append([]string(nil), pattern.MergedFrom...),
		BusinessMethod:    pattern.BusinessMethod,
		EvidenceLocations: append([]domain.PatternEvidenceLocation(nil), pattern.EvidenceLocations...),
		ProjectID:         pattern.ProjectID,
		ScopePath:         pattern.ScopePath,
		WorkspaceRole:     pattern.WorkspaceRole,
		AnalysisUnitID:    pattern.AnalysisUnitID,
		AnalysisUnitName:  pattern.AnalysisUnitName,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	result.RefreshMetrics()
	return result
}

func applyCuratedPatterns(ctx context.Context, repo domain.PatternRepository, curated []agent.CuratedPattern, existing []domain.Pattern) ([]domain.Pattern, error) {
	written := make([]domain.Pattern, 0, len(curated))
	outputIDs := make(map[string]struct{}, len(curated))
	for _, pattern := range curated {
		outputIDs[pattern.ID] = struct{}{}
	}
	existingIDs := patternIDSet(existing)

	for _, pattern := range curated {
		for _, sourceID := range pattern.MergedFrom {
			if sourceID == "" || sourceID == pattern.ID {
				continue
			}
			if _, existed := existingIDs[sourceID]; !existed {
				continue
			}
			if _, isOutput := outputIDs[sourceID]; isOutput {
				continue
			}
			if err := repo.Delete(ctx, sourceID); err != nil {
				return written, err
			}
		}
		domainPattern := curatedToDomain(pattern)
		if err := repo.Save(ctx, &domainPattern); err != nil {
			return written, err
		}
		written = append(written, domainPattern)
	}
	return written, nil
}

func fallbackCurate(candidates, existing []domain.Pattern) *agent.CuratePatternsResult {
	return deterministicCurate(candidates, existing)
}

func deterministicCurate(candidates, existing []domain.Pattern) *agent.CuratePatternsResult {
	merger := newDeterministicMerger(existing, len(candidates))

	for _, candidate := range candidates {
		merger.Add(candidate)
	}

	result := &agent.CuratePatternsResult{
		Patterns: merger.CuratedPatterns(),
		Dropped:  []agent.CuratedDrop{},
	}

	result.Summary = agent.CurateSummary{
		TotalCandidates: len(candidates),
		TotalExisting:   len(existing),
		TotalWritten:    len(result.Patterns),
		TotalDropped:    len(result.Dropped),
	}
	return result
}

type deterministicMerger struct {
	accepted []domain.Pattern
	output   map[string]agent.CuratedPattern
}

func newDeterministicMerger(existing []domain.Pattern, candidateCount int) *deterministicMerger {
	accepted := make([]domain.Pattern, 0, len(existing)+candidateCount)
	for _, pattern := range existing {
		accepted = append(accepted, normalizeCandidate(pattern))
	}
	return &deterministicMerger{
		accepted: accepted,
		output:   make(map[string]agent.CuratedPattern, candidateCount),
	}
}

func (m *deterministicMerger) Add(candidate domain.Pattern) {
	candidate = normalizeCandidate(candidate)
	bestIndex, bestScore := m.bestMatch(candidate)
	if bestIndex >= 0 && bestScore >= deterministicMergeThreshold {
		merged := mergeKeepingBestPattern(m.accepted[bestIndex], candidate)
		m.accepted[bestIndex] = merged
		m.removeMergedOutputs(merged.MergedFrom, merged.ID)
		m.output[merged.ID] = domainToCurated(merged, merged.MergedFrom, bestScore, "deterministic merge")
		return
	}
	m.accepted = append(m.accepted, candidate)
	m.output[candidate.ID] = domainToCurated(candidate, []string{candidate.ID}, 1, "new candidate")
}

func (m *deterministicMerger) bestMatch(candidate domain.Pattern) (int, float64) {
	bestIndex := -1
	bestScore := 0.0
	for i := range m.accepted {
		if m.accepted[i].ID == candidate.ID {
			continue
		}
		score := patternSimilarity(candidate, m.accepted[i])
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	return bestIndex, bestScore
}

func (m *deterministicMerger) removeMergedOutputs(mergedFrom []string, keepID string) {
	for _, id := range mergedFrom {
		if id != keepID {
			delete(m.output, id)
		}
	}
}

func (m *deterministicMerger) CuratedPatterns() []agent.CuratedPattern {
	result := make([]agent.CuratedPattern, 0, len(m.output))
	for _, pattern := range m.accepted {
		if curated, ok := m.output[pattern.ID]; ok {
			result = append(result, curated)
		}
	}
	return result
}

func mergeKeepingBestPattern(left, right domain.Pattern) domain.Pattern {
	left = normalizeCandidate(left)
	right = normalizeCandidate(right)
	primary, secondary := left, right
	if patternQualityScore(right) > patternQualityScore(left) {
		primary, secondary = right, left
	}
	primary.Merge(&secondary)
	primary.Merged = true
	primary.MergedFrom = mergedPatternSources(left, right)
	if primary.Source == "" {
		primary.Source = secondary.Source
	}
	if primary.ProjectID == "" {
		primary.ProjectID = secondary.ProjectID
	}
	if primary.ScopePath == "" {
		primary.ScopePath = secondary.ScopePath
	}
	if primary.WorkspaceRole == "" {
		primary.WorkspaceRole = secondary.WorkspaceRole
	}
	if primary.AnalysisUnitID == "" {
		primary.AnalysisUnitID = secondary.AnalysisUnitID
	}
	if primary.AnalysisUnitName == "" {
		primary.AnalysisUnitName = secondary.AnalysisUnitName
	}
	if primary.BusinessMethod == nil {
		primary.BusinessMethod = secondary.BusinessMethod
	}
	primary.EvidenceLocations = mergeEvidenceLocations(primary.EvidenceLocations, secondary.EvidenceLocations)
	primary.RefreshMetrics()
	return primary
}

func mergedPatternSources(left, right domain.Pattern) []string {
	values := make([]string, 0, len(left.MergedFrom)+len(right.MergedFrom)+2)
	values = append(values, left.MergedFrom...)
	values = append(values, right.MergedFrom...)
	values = append(values, left.ID, right.ID)
	return uniqueStrings(values)
}

func patternQualityScore(pattern domain.Pattern) float64 {
	pattern.RefreshMetrics()
	score := pattern.Metrics.EffectiveScore
	if score == 0 {
		score = pattern.Confidence
	}
	score += float64(pattern.Frequency) * 0.01
	if pattern.BusinessMethod != nil {
		score += 0.05
	}
	if strings.TrimSpace(pattern.GoodExample) != "" {
		score += 0.04
	}
	if len(pattern.EvidenceLocations) > 0 {
		score += 0.03
	}
	return score
}

func mergeEvidenceLocations(left, right []domain.PatternEvidenceLocation) []domain.PatternEvidenceLocation {
	out := make([]domain.PatternEvidenceLocation, 0, len(left)+len(right))
	seen := map[string]struct{}{}
	add := func(loc domain.PatternEvidenceLocation) {
		key := loc.DisplayLocation() + "|" + loc.Symbol + "|" + loc.Kind
		if key == "||" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, loc)
	}
	for _, loc := range left {
		add(loc)
	}
	for _, loc := range right {
		add(loc)
	}
	return out
}

func domainToCurated(pattern domain.Pattern, mergedFrom []string, similarity float64, reason string) agent.CuratedPattern {
	pattern = normalizeCandidate(pattern)
	return agent.CuratedPattern{
		ID:                pattern.ID,
		Name:              pattern.Name,
		Category:          string(pattern.Category),
		Description:       pattern.Description,
		GoodExample:       pattern.GoodExample,
		BadExample:        pattern.BadExample,
		Rule:              pattern.Rule,
		Confidence:        pattern.Confidence,
		Frequency:         pattern.Frequency,
		MergedFrom:        uniqueStrings(mergedFrom),
		MergeReason:       reason,
		SimilarityScore:   similarity,
		Source:            string(pattern.Source),
		BusinessMethod:    pattern.BusinessMethod,
		EvidenceLocations: append([]domain.PatternEvidenceLocation(nil), pattern.EvidenceLocations...),
		ProjectID:         pattern.ProjectID,
		ScopePath:         pattern.ScopePath,
		WorkspaceRole:     pattern.WorkspaceRole,
		AnalysisUnitID:    pattern.AnalysisUnitID,
		AnalysisUnitName:  pattern.AnalysisUnitName,
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
