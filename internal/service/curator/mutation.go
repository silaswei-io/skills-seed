package curator

import (
	"context"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type mutationIntent int

const (
	storeCandidates mutationIntent = iota
	compactLibrary
)

type mutationPlan struct {
	Written  []domain.Pattern
	Mutation domain.PatternMutation
}

func applyCuratedPatterns(ctx context.Context, repo patternStore, curated []domain.Pattern, dropped []Drop, existing []domain.Pattern, intent mutationIntent) ([]domain.Pattern, error) {
	plan := buildMutationPlan(curated, dropped, existing, intent)
	if err := repo.ApplyPatternMutation(ctx, plan.Mutation); err != nil {
		return nil, err
	}
	return plan.Written, nil
}

func buildMutationPlan(curated []domain.Pattern, dropped []Drop, existing []domain.Pattern, intent mutationIntent) mutationPlan {
	plan := mutationPlan{Written: make([]domain.Pattern, 0, len(curated))}
	outputIDs := make(map[string]struct{}, len(curated))
	for _, pattern := range curated {
		outputIDs[pattern.ID] = struct{}{}
	}
	existingIDs := patternIDSet(existing)
	deleteIDs := make([]string, 0)

	if intent == compactLibrary {
		for _, item := range dropped {
			if _, existed := existingIDs[item.ID]; !existed {
				continue
			}
			if _, isOutput := outputIDs[item.ID]; !isOutput {
				deleteIDs = append(deleteIDs, item.ID)
			}
		}
	}

	for _, pattern := range curated {
		for _, sourceID := range pattern.MergedFrom {
			if sourceID == "" || sourceID == pattern.ID {
				continue
			}
			if _, existed := existingIDs[sourceID]; !existed {
				continue
			}
			if _, isOutput := outputIDs[sourceID]; !isOutput {
				deleteIDs = append(deleteIDs, sourceID)
			}
		}
		domainPattern := patternForSave(pattern)
		plan.Written = append(plan.Written, domainPattern)
		plan.Mutation.Save = append(plan.Mutation.Save, &plan.Written[len(plan.Written)-1])
	}
	plan.Mutation.DeleteIDs = uniqueStrings(deleteIDs)
	return plan
}

func patternForSave(pattern domain.Pattern) domain.Pattern {
	pattern = normalizeCandidate(pattern)
	now := time.Now()
	pattern.Merged = len(pattern.MergedFrom) > 1
	pattern.MergedFrom = append([]string(nil), pattern.MergedFrom...)
	pattern.BusinessMethod = cloneBusinessMethod(pattern.BusinessMethod)
	pattern.EvidenceLocations = append([]domain.PatternEvidenceLocation(nil), pattern.EvidenceLocations...)
	pattern.CreatedAt = now
	pattern.UpdatedAt = now
	pattern.RefreshMetrics()
	return pattern
}

func patternsForSave(patterns []domain.Pattern) []domain.Pattern {
	result := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		result = append(result, patternForSave(pattern))
	}
	return result
}
