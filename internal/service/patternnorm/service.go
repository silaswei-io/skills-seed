package patternnorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

// Service 是模式库唯一的规范入库边界。
type Service struct {
	patternRepo patternStore
}

// NewService 创建模式规范化入库服务。
func NewService(repo patternStore) *Service {
	return &Service{
		patternRepo: repo,
	}
}

// NormalizeAndStore 将候选模式规范化为可入库模式并写入模式库。
func (s *Service) NormalizeAndStore(ctx context.Context, req NormalizeRequest) (*NormalizeResult, error) {
	return s.NormalizeAndStoreWithHooks(ctx, req, ProgressHooks{})
}

// NormalizeAndStoreWithHooks 将候选模式规范化为可入库模式并写入模式库，并向调用方报告进度。
func (s *Service) NormalizeAndStoreWithHooks(ctx context.Context, req NormalizeRequest, hooks ProgressHooks) (*NormalizeResult, error) {
	if !req.Operation.Valid() || req.Operation == OperationCompact {
		return nil, fmt.Errorf("%s", i18n.GetWithParams("PatternNormUnsupportedOperation", map[string]interface{}{"Operation": req.Operation}))
	}
	candidates := validateCandidates(req.Candidates)
	if req.Operation == OperationLearnCurrent {
		candidates = coalesceCurrentCandidates(validateCurrentCandidates(candidates))
	}
	if len(candidates) == 0 {
		return &NormalizeResult{
			Summary: Summary{
				TotalCandidates: len(req.Candidates),
			},
		}, nil
	}

	existing, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormLoadExistingPatternsFailed"), err)
	}
	existing = activeNormalizePatterns(existing)
	retrieved := retrieveRelatedPatterns(candidates, existing, relatedPatternsPerCandidate)
	var normalized *proposal
	if req.Operation == OperationLearnCurrent {
		normalized, err = s.normalizeCurrent(ctx, candidates, retrieved, req.DecisionCheckpoint, hooks)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormLearnCurrentFailed"), err)
		}
	} else {
		normalized = deterministicNormalize(candidates, retrieved.related)
	}
	notifyProgress(hooks.OnValidationStart, i18n.Get("ProgressNormalizePatternsValidation"))
	if err := validateNormalizeResultForOperation(req.Operation, normalized, candidates, retrieved.related); err != nil {
		if req.Operation == OperationLearnCurrent {
			return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormValidateCurrentFailed"), err)
		}
		return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormValidateDeterministicFailed"), err)
	}

	notifyProgress(hooks.OnStoreStart, i18n.Get("ProgressNormalizePatternsStore"))
	written, err := applyNormalizedPatterns(ctx, s.patternRepo, normalized.Patterns, normalized.Dropped, retrieved.related, storeCandidates)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormApplyPatternsFailed"), err)
	}
	return &NormalizeResult{
		Written: written,
		Dropped: normalized.Dropped,
		Summary: summarizeNormalization(len(candidates), len(retrieved.related), written, normalized.Dropped),
	}, nil
}

func notifyProgress(callback func(string), label string) {
	if callback != nil {
		callback(label)
	}
}

func activeNormalizePatterns(patterns []domain.Pattern) []domain.Pattern {
	out := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.IsActive() {
			out = append(out, pattern)
		}
	}
	return out
}

func loadNormalizationDecision(ctx context.Context, checkpoint DecisionCheckpoint, decisionKey string, hooks ProgressHooks) (*Decision, bool, error) {
	if checkpoint == nil {
		return nil, false, nil
	}
	result, found, err := checkpoint.Load(ctx, decisionKey)
	if err != nil {
		return nil, false, fmt.Errorf("load normalization decision: %w", err)
	}
	if !found {
		return nil, false, nil
	}
	label := i18n.Get("ProgressNormalizePatternsReplay")
	notifyProgress(hooks.OnStepStart, label)
	notifyProgress(hooks.OnStepComplete, label)
	return result, true, nil
}

func saveNormalizationDecision(ctx context.Context, checkpoint DecisionCheckpoint, decisionKey string, result *Decision) error {
	if checkpoint == nil {
		return nil
	}
	if err := checkpoint.Save(ctx, decisionKey, result); err != nil {
		return fmt.Errorf("save normalization decision: %w", err)
	}
	return nil
}

func normalizationDecisionKey(candidates []domain.Pattern) (string, error) {
	data, err := json.Marshal(candidates)
	if err != nil {
		return "", fmt.Errorf("hash normalization candidates: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func logNormalizationAssessment(operation Operation, assessment normalizationAssessment) {
	if len(assessment.IgnoredDroppedIDs) == 0 && len(assessment.IgnoredConflictingDroppedIDs) == 0 && len(assessment.IgnoredMergedFromIDs) == 0 && len(assessment.IgnoredPatternIDs) == 0 && len(assessment.ResolvedOwnershipIDs) == 0 && assessment.Coverage.MissingCount() == 0 {
		return
	}
	logger.Info(i18n.Get("LoggerPatternNormSanitized"),
		"operation", operation,
		"ignored_dropped_ids", assessment.IgnoredDroppedIDs,
		"ignored_conflicting_dropped_ids", assessment.IgnoredConflictingDroppedIDs,
		"ignored_merged_from_ids", assessment.IgnoredMergedFromIDs,
		"ignored_pattern_ids", assessment.IgnoredPatternIDs,
		"resolved_ownership_ids", assessment.ResolvedOwnershipIDs,
		"unclassified_ids", assessment.Coverage.MissingIDs,
		"coverage_ratio", 1-assessment.Coverage.MissingRatio(),
		"reason", "references may only use current candidate or retrieved existing pattern ids",
	)
}
