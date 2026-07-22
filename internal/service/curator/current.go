package curator

import (
	"context"
	"fmt"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

func (s *Service) curateCurrent(ctx context.Context, candidates []domain.Pattern, retrieved retrievalResult, hooks ProgressHooks) (*proposal, error) {
	result, err := s.curate(ctx, OperationLearnCurrent, candidates, retrieved.related, false, retrieved.existingByCandidate, hooks)
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
