package patternnorm

import (
	"context"
	"fmt"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge/patternview"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

func (s *Service) normalizeCurrent(ctx context.Context, req NormalizeRequest, candidates []domain.Pattern, retrieved retrievalResult, hooks ProgressHooks) (*proposal, error) {
	decisionKey, err := normalizationDecisionKey(candidates)
	if err != nil {
		return nil, err
	}
	if result, found, err := loadNormalizationDecision(ctx, req.DecisionCheckpoint, decisionKey, hooks); err != nil || found {
		if err != nil {
			return nil, err
		}
		return finalizeCurrentNormalization(proposalFromDecision(result), candidates, retrieved.related)
	}

	result := s.normalizeCurrentWithAI(ctx, req, candidates, retrieved, hooks)
	if result != nil {
		result, err = finalizeCurrentNormalization(result, candidates, retrieved.related)
		if err == nil {
			err = validateNormalizeResultForOperation(OperationLearnCurrent, result, candidates, retrieved.related)
		}
		if err == nil {
			if err := saveNormalizationDecision(ctx, req.DecisionCheckpoint, decisionKey, decisionFromProposal(result)); err != nil {
				return nil, err
			}
			return result, nil
		}
		logger.Warn(i18n.Get("LoggerPatternNormAIFallback"), "error", err)
	}
	result = normalizeCurrentLocally(candidates, hooks)
	result, err = finalizeCurrentNormalization(result, candidates, retrieved.related)
	if err != nil {
		return nil, err
	}
	if err := saveNormalizationDecision(ctx, req.DecisionCheckpoint, decisionKey, decisionFromProposal(result)); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) normalizeCurrentWithAI(ctx context.Context, req NormalizeRequest, candidates []domain.Pattern, retrieved retrievalResult, hooks ProgressHooks) *proposal {
	if s.normalizer == nil {
		return nil
	}
	label := i18n.Get("ProgressNormalizePatternsAI")
	notifyProgress(hooks.OnStepStart, label)
	result, err := s.normalizer.NormalizePatterns(ctx, &agent.NormalizePatternsRequest{
		ProjectName:     req.ProjectName,
		RootPath:        req.RootPath,
		Language:        req.Language,
		Candidates:      candidates,
		RelatedPatterns: retrieved.related,
		UserContext:     req.UserContext,
	})
	if err != nil {
		logger.Warn(i18n.Get("LoggerPatternNormAIFallback"), "error", err)
		return nil
	}
	notifyProgress(hooks.OnStepComplete, label)
	return proposalFromNormalizePatternsResult(result)
}

func normalizeCurrentLocally(candidates []domain.Pattern, hooks ProgressHooks) *proposal {
	label := i18n.Get("ProgressNormalizePatternsLocal")
	notifyProgress(hooks.OnStepStart, label)
	result := keepCurrentCandidates(candidates)
	if hooks.OnStepComplete != nil {
		hooks.OnStepComplete(label)
	}
	return result
}

func keepCurrentCandidates(candidates []domain.Pattern) *proposal {
	result := &proposal{
		Patterns: make([]domain.Pattern, 0, len(candidates)),
		Dropped:  []Drop{},
	}
	for _, candidate := range candidates {
		result.Patterns = append(result.Patterns, patternview.WithSources(candidate, patternview.SourceIDs(candidate)))
	}
	return result
}

func proposalFromNormalizePatternsResult(result *agent.NormalizePatternsResult) *proposal {
	if result == nil {
		return nil
	}
	out := &proposal{
		Patterns: make([]domain.Pattern, 0, len(result.Patterns)),
		Dropped:  make([]Drop, 0, len(result.Dropped)),
	}
	for _, item := range result.Patterns {
		out.Patterns = append(out.Patterns, domain.Pattern{
			ID:          item.ID,
			Name:        item.Name,
			Category:    domain.Category(item.Category),
			Description: item.Description,
			Rule:        item.Rule,
			Confidence:  item.Confidence,
			Merged:      len(item.SourceIDs) > 1,
			MergedFrom:  append([]string(nil), item.SourceIDs...),
		})
	}
	for _, item := range result.Dropped {
		out.Dropped = append(out.Dropped, Drop{
			ID:         item.ID,
			ReasonCode: DropReasonCode(item.ReasonCode),
			Reason:     item.Reason,
		})
	}
	return out
}

func finalizeCurrentNormalization(result *proposal, candidates, existing []domain.Pattern) (*proposal, error) {
	var conflictingDroppedIDs []string
	result, conflictingDroppedIDs = preferCurrentPatternsOverConflictingDrops(result, candidates)
	assessment := assessNormalization(result, candidates, existing)
	assessment.IgnoredConflictingDroppedIDs = conflictingDroppedIDs
	var recallRecoveredIDs []string
	result, recallRecoveredIDs = recoverRecallProtectedDrops(assessment.Result, candidates)
	if len(recallRecoveredIDs) > 0 {
		logger.Diagnostic(i18n.Get("LoggerPatternNormRecallRecovered"),
			"operation", OperationLearnCurrent,
			"recovered_ids", recallRecoveredIDs,
			"candidate_count", len(candidates),
		)
		assessment = assessNormalization(result, candidates, existing)
		assessment.IgnoredConflictingDroppedIDs = conflictingDroppedIDs
	}
	logNormalizationAssessment(OperationLearnCurrent, assessment)
	if assessment.Coverage.MissingCount() == 0 {
		return assessment.Result, nil
	}
	logger.Warn(i18n.Get("LoggerPatternNormCoverageRecovered"),
		"operation", OperationLearnCurrent,
		"missing_count", assessment.Coverage.MissingCount(),
		"candidate_count", assessment.Coverage.CandidateCount,
		"missing_ratio", assessment.Coverage.MissingRatio(),
	)
	result = recoverCurrentNormalization(assessment, candidates)
	assessment = assessNormalization(result, candidates, existing)
	logNormalizationAssessment(OperationLearnCurrent, assessment)
	if assessment.Coverage.MissingCount() > 0 {
		return nil, fmt.Errorf("recover normalization coverage: %d of %d candidates remain unclassified", assessment.Coverage.MissingCount(), assessment.Coverage.CandidateCount)
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
