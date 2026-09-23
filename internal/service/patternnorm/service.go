package patternnorm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/knowledge/maintained"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

// Service 是模式库唯一的规范入库边界。
type Service struct {
	patternRepo patternStore
	normalizer  agent.PatternNormalizer
	reviewer    agent.KnowledgeReviewer
	admission   AdmissionPolicy
	guidance    maintained.Provider
}

// WithMaintainedGuidance 为审查和规范化调用注入持久化 Rule 与 Workflow。
// 只应在容器构造阶段、服务开始处理请求前调用。
func (s *Service) WithMaintainedGuidance(provider maintained.Provider) *Service {
	s.guidance = provider
	return s
}

// NewServiceWithNormalizer 创建带 AI 合并优化的模式规范化服务。
func NewServiceWithNormalizer(repo patternStore, normalizer agent.PatternNormalizer, admission ...AdmissionPolicy) *Service {
	policy := DefaultAdmissionPolicy()
	if len(admission) > 0 {
		policy = admission[0]
	}
	reviewer, _ := normalizer.(agent.KnowledgeReviewer)
	return &Service{patternRepo: repo, normalizer: normalizer, reviewer: reviewer, admission: policy}
}

// NewService 创建模式规范化入库服务。
func NewService(repo patternStore, admission ...AdmissionPolicy) *Service {
	policy := DefaultAdmissionPolicy()
	if len(admission) > 0 {
		policy = admission[0]
	}
	return &Service{patternRepo: repo, admission: policy}
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
	var candidates []domain.Pattern
	if req.Operation == OperationLearnCurrent {
		candidates = s.prepareCurrentCandidates(req.Candidates)
	} else {
		candidates = validateCandidates(req.Candidates)
	}
	retiredIDs := uniquePatternIDs(req.RetiredPatternIDs)
	if len(candidates) == 0 && len(retiredIDs) == 0 {
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
	retiredIDs = eligibleRetiredPatternIDs(retiredIDs, existing, nil)
	activeExisting := activeNormalizePatterns(existing)
	if len(candidates) == 0 {
		if len(retiredIDs) > 0 {
			if err := s.patternRepo.ApplyPatternMutation(ctx, domain.PatternMutation{DeleteIDs: retiredIDs}); err != nil {
				return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormApplyPatternsFailed"), err)
			}
		}
		return &NormalizeResult{
			RetiredPatternIDs: retiredIDs,
			Summary: Summary{
				TotalCandidates: len(req.Candidates),
				TotalExisting:   len(activeExisting),
			},
		}, nil
	}

	retrieved := retrieveRelatedPatterns(candidates, activeExisting)
	var normalized *proposal
	aiSkipped := false
	if req.Operation == OperationLearnCurrent {
		outcome, normErr := s.normalizeCurrent(ctx, req, candidates, retrieved, hooks)
		if normErr != nil {
			return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormLearnCurrentFailed"), normErr)
		}
		normalized = outcome.proposal
		aiSkipped = outcome.aiSkipped
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
	// A2：入库前证据硬闸，丢弃无法在项目根验证的证据归属。
	gatedPatterns, evidenceDrops := gateEvidenceForPersistence(req.RootPath, normalized.Patterns)
	if len(evidenceDrops) > 0 {
		normalized.Dropped = append(normalized.Dropped, evidenceDrops...)
	}
	retiredIDs = eligibleRetiredPatternIDs(retiredIDs, existing, gatedPatterns)
	written, err := applyNormalizedPatterns(ctx, s.patternRepo, gatedPatterns, normalized.Dropped, retrieved.related, retiredIDs, storeCandidates)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Get("PatternNormApplyPatternsFailed"), err)
	}
	return &NormalizeResult{
		Written:           written,
		RetiredPatternIDs: retiredIDs,
		Dropped:           normalized.Dropped,
		Summary:           summarizeNormalization(len(candidates), len(retrieved.related), written, normalized.Dropped),
		AISkipped:         aiSkipped,
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

func uniquePatternIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func eligibleRetiredPatternIDs(ids []string, existing, outputs []domain.Pattern) []string {
	outputIDs := make(map[string]bool, len(outputs))
	for _, pattern := range outputs {
		outputIDs[pattern.ID] = true
	}
	existingByID := make(map[string]domain.Pattern, len(existing))
	for _, pattern := range existing {
		existingByID[pattern.ID] = pattern
	}

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		pattern, ok := existingByID[id]
		if !ok || outputIDs[id] || !pattern.CanBeRetiredFromCurrentLearning() {
			continue
		}
		out = append(out, id)
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

func normalizationDecisionKey(candidates, related []domain.Pattern, userContext string, guidance maintained.Snapshot) (string, error) {
	data, err := json.Marshal(struct {
		Candidates  []domain.Pattern
		Related     []domain.Pattern
		UserContext string
		Guidance    maintained.Snapshot
	}{candidates, related, userContext, guidance})
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
	logger.Diagnostic(i18n.Get("LoggerPatternNormSanitized"),
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
