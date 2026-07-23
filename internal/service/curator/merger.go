package curator

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func deterministicCurate(candidates, existing []domain.Pattern) *proposal {
	merger := newDeterministicMerger(existing, len(candidates))

	for _, candidate := range candidates {
		merger.Add(candidate)
	}

	result := &proposal{
		Patterns: merger.CuratedPatterns(),
		Dropped:  []Drop{},
	}
	return result
}

type deterministicMerger struct {
	accepted  []domain.Pattern
	indexByID map[string]int
	output    map[string]domain.Pattern
}

func newDeterministicMerger(existing []domain.Pattern, candidateCount int) *deterministicMerger {
	merger := &deterministicMerger{
		accepted:  make([]domain.Pattern, 0, len(existing)+candidateCount),
		indexByID: make(map[string]int, len(existing)+candidateCount),
		output:    make(map[string]domain.Pattern, candidateCount),
	}
	for _, pattern := range existing {
		merger.upsertAccepted(normalizeCandidate(pattern))
	}
	return merger
}

func (m *deterministicMerger) Add(candidate domain.Pattern) {
	candidate = normalizeCandidate(candidate)
	if index, ok := m.indexByID[candidate.ID]; ok {
		m.recordMerged(index, candidate)
		return
	}
	bestIndex, bestScore := m.bestMatch(candidate)
	if bestIndex >= 0 && bestScore >= deterministicMergeThreshold {
		m.recordMerged(bestIndex, candidate)
		return
	}
	m.appendAccepted(candidate)
	m.output[candidate.ID] = patternWithSources(candidate, []string{candidate.ID})
}

func (m *deterministicMerger) upsertAccepted(pattern domain.Pattern) {
	if index, ok := m.indexByID[pattern.ID]; ok {
		m.replaceAccepted(index, mergeKeepingBestPattern(m.accepted[index], pattern))
		return
	}
	m.appendAccepted(pattern)
}

func (m *deterministicMerger) recordMerged(index int, candidate domain.Pattern) {
	merged := mergeKeepingBestPattern(m.accepted[index], candidate)
	m.replaceAccepted(index, merged)
	m.removeMergedOutputs(merged.MergedFrom, merged.ID)
	m.output[merged.ID] = patternWithSources(merged, merged.MergedFrom)
}

func (m *deterministicMerger) appendAccepted(pattern domain.Pattern) {
	m.indexByID[pattern.ID] = len(m.accepted)
	m.accepted = append(m.accepted, pattern)
}

func (m *deterministicMerger) replaceAccepted(index int, pattern domain.Pattern) {
	previousID := m.accepted[index].ID
	if previousID != pattern.ID {
		delete(m.indexByID, previousID)
	}
	m.accepted[index] = pattern
	m.indexByID[pattern.ID] = index
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

func (m *deterministicMerger) CuratedPatterns() []domain.Pattern {
	result := make([]domain.Pattern, 0, len(m.output))
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
	primary.HistoryEvidence = mergeHistoryEvidence(primary.HistoryEvidence, secondary.HistoryEvidence)
	primary.RefreshMetrics()
	return primary
}

func mergeHistoryEvidence(left, right domain.PatternHistoryEvidence) domain.PatternHistoryEvidence {
	commits := uniqueStrings(append(append([]string(nil), left.CommitHashes...), right.CommitHashes...))
	paths := uniqueStrings(append(append([]string(nil), left.CoChangedPaths...), right.CoChangedPaths...))
	first := left.FirstSeenAt
	if first.IsZero() || !right.FirstSeenAt.IsZero() && right.FirstSeenAt.Before(first) {
		first = right.FirstSeenAt
	}
	last := left.LastSeenAt
	if right.LastSeenAt.After(last) {
		last = right.LastSeenAt
	}
	return domain.PatternHistoryEvidence{CommitHashes: commits, CommitCount: len(commits), FirstSeenAt: first, LastSeenAt: last, CoChangedPaths: paths}
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

func patternWithSources(pattern domain.Pattern, mergedFrom []string) domain.Pattern {
	pattern = normalizeCandidate(pattern)
	pattern.MergedFrom = uniqueStrings(mergedFrom)
	pattern.Merged = len(pattern.MergedFrom) > 1
	pattern.BusinessMethod = cloneBusinessMethod(pattern.BusinessMethod)
	pattern.EvidenceLocations = append([]domain.PatternEvidenceLocation(nil), pattern.EvidenceLocations...)
	return pattern
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
