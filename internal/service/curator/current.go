package curator

import (
	"context"
	"fmt"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

func (s *Service) curateCurrent(ctx context.Context, candidates []domain.Pattern, retrieved retrievalResult, checkpoint DecisionCheckpoint, hooks ProgressHooks) (*proposal, error) {
	var result *proposal
	var err error
	if full, ok := checkpoint.(fullDecisionCheckpoint); ok && full.HasFullDecision() {
		result, err = s.curate(ctx, OperationLearnCurrent, candidates, retrieved.related, false, retrieved.existingByCandidate, checkpoint, hooks)
	} else {
		switch s.backend {
		case config.LearningBackendLocal:
			result = deterministicCurate(candidates, retrieved.related)
		case config.LearningBackendHybrid:
			result, err = s.curateHybridCurrent(ctx, candidates, retrieved, checkpoint, hooks)
		default:
			result, err = s.curate(ctx, OperationLearnCurrent, candidates, retrieved.related, false, retrieved.existingByCandidate, checkpoint, hooks)
		}
	}
	if err != nil {
		return nil, err
	}
	var conflictingDroppedIDs []string
	result, conflictingDroppedIDs = preferCurrentPatternsOverConflictingDrops(result, candidates)
	assessment := assessCuration(result, candidates, retrieved.related)
	assessment.IgnoredConflictingDroppedIDs = conflictingDroppedIDs
	logCurationAssessment(OperationLearnCurrent, assessment)
	if assessment.Coverage.MissingCount() == 0 {
		return assessment.Result, nil
	}
	logger.Warn(i18n.Get("LoggerAgentCuratePatternsCoverageRecovered"),
		"operation", OperationLearnCurrent,
		"missing_count", assessment.Coverage.MissingCount(),
		"candidate_count", assessment.Coverage.CandidateCount,
		"missing_ratio", assessment.Coverage.MissingRatio(),
	)
	result = recoverCurrentCuration(assessment, candidates, retrieved.related)
	assessment = assessCuration(result, candidates, retrieved.related)
	logCurationAssessment(OperationLearnCurrent, assessment)
	if assessment.Coverage.MissingCount() > 0 {
		return nil, fmt.Errorf("recover curation coverage: %d of %d candidates remain unclassified", assessment.Coverage.MissingCount(), assessment.Coverage.CandidateCount)
	}
	return assessment.Result, nil
}

const hybridCurationBatchSize = 25

func (s *Service) curateHybridCurrent(ctx context.Context, candidates []domain.Pattern, retrieved retrievalResult, checkpoint DecisionCheckpoint, hooks ProgressHooks) (*proposal, error) {
	local, ambiguous := partitionCurationCandidates(candidates, retrieved.related)
	result := deterministicCurate(local, retrieved.related)
	for start := 0; start < len(ambiguous); start += hybridCurationBatchSize {
		end := min(start+hybridCurationBatchSize, len(ambiguous))
		batch := ambiguous[start:end]
		batchRetrieved := retrieveRelatedPatterns(batch, retrieved.related, relatedPatternsPerCandidate)
		curated, err := s.curate(ctx, OperationLearnCurrent, batch, batchRetrieved.related, false, batchRetrieved.existingByCandidate, checkpoint, hooks)
		if err != nil {
			return nil, err
		}
		result.Patterns = append(result.Patterns, curated.Patterns...)
		result.Dropped = append(result.Dropped, curated.Dropped...)
	}
	return result, nil
}

func partitionCurationCandidates(candidates, existing []domain.Pattern) ([]domain.Pattern, []domain.Pattern) {
	ambiguous := make(map[string]bool, len(candidates))
	for i, candidate := range candidates {
		if candidate.Confidence < 0.75 || len(candidate.EvidenceLocations) == 0 {
			ambiguous[candidate.ID] = true
		}
		for _, pattern := range existing {
			score := patternSimilarity(candidate, pattern)
			if score >= 0.45 && score < deterministicMergeThreshold {
				ambiguous[candidate.ID] = true
			}
		}
		for j := i + 1; j < len(candidates); j++ {
			score := patternSimilarity(candidate, candidates[j])
			if score >= 0.45 && score < deterministicMergeThreshold {
				ambiguous[candidate.ID] = true
				ambiguous[candidates[j].ID] = true
			}
		}
	}
	local := make([]domain.Pattern, 0, len(candidates))
	uncertain := make([]domain.Pattern, 0, len(ambiguous))
	for _, candidate := range candidates {
		if ambiguous[candidate.ID] {
			uncertain = append(uncertain, candidate)
		} else {
			local = append(local, candidate)
		}
	}
	return local, uncertain
}

func preferCurrentPatternsOverConflictingDrops(result *proposal, candidates []domain.Pattern) (*proposal, []string) {
	result = cloneProposal(result)
	if result == nil || len(result.Dropped) == 0 {
		return result, nil
	}
	candidateIDs := patternIDSet(candidates)
	represented := make(map[string]struct{}, len(candidateIDs))
	for _, pattern := range result.Patterns {
		for _, sourceID := range pattern.MergedFrom {
			if _, ok := candidateIDs[sourceID]; ok {
				represented[sourceID] = struct{}{}
			}
		}
		if len(pattern.MergedFrom) == 0 {
			if _, ok := candidateIDs[pattern.ID]; ok {
				represented[pattern.ID] = struct{}{}
			}
		}
	}

	dropped := result.Dropped[:0]
	var ignored []string
	for _, item := range result.Dropped {
		if _, conflict := represented[item.ID]; conflict {
			ignored = append(ignored, item.ID)
			continue
		}
		dropped = append(dropped, item)
	}
	result.Dropped = dropped
	sort.Strings(ignored)
	return result, ignored
}
