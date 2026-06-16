package curator

import (
	"context"
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
	result := &agent.CuratePatternsResult{
		Patterns: make([]agent.CuratedPattern, 0, len(candidates)),
		Dropped:  []agent.CuratedDrop{},
	}
	usedExisting := make(map[string]struct{})

	for _, candidate := range candidates {
		bestIndex := -1
		bestScore := 0.0
		for i, pattern := range existing {
			if _, used := usedExisting[pattern.ID]; used {
				continue
			}
			score := patternSimilarity(candidate, pattern)
			if score > bestScore {
				bestScore = score
				bestIndex = i
			}
		}

		if bestIndex >= 0 && bestScore >= deterministicMergeThreshold {
			existingPattern := existing[bestIndex]
			usedExisting[existingPattern.ID] = struct{}{}
			merged := existingPattern
			merged.Merge(&candidate)
			result.Patterns = append(result.Patterns, domainToCurated(merged, []string{existingPattern.ID, candidate.ID}, bestScore, "deterministic merge"))
			continue
		}
		result.Patterns = append(result.Patterns, domainToCurated(candidate, []string{candidate.ID}, 1, "new candidate"))
	}

	result.Summary = agent.CurateSummary{
		TotalCandidates: len(candidates),
		TotalExisting:   len(existing),
		TotalWritten:    len(result.Patterns),
		TotalDropped:    len(result.Dropped),
	}
	return result
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
