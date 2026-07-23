package curator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

// Service 是模式库唯一的规范入库边界。
type Service struct {
	agent       curationAgent
	patternRepo patternStore
	backend     config.LearningBackend
}

// NewService 创建模式策展服务。
func NewService(ag curationAgent, repo patternStore) *Service {
	return NewServiceWithBackend(ag, repo, config.LearningBackendAgent)
}

// NewServiceWithBackend 创建使用指定学习后端的模式策展服务。
func NewServiceWithBackend(ag curationAgent, repo patternStore, backend config.LearningBackend) *Service {
	return &Service{
		agent:       ag,
		patternRepo: repo,
		backend:     config.NormalizeLearningBackend(string(backend)),
	}
}

// CurateAndStore 将候选模式策展为规范模式并写入模式库。
func (s *Service) CurateAndStore(ctx context.Context, req CurateRequest) (*CurateResult, error) {
	return s.CurateAndStoreWithHooks(ctx, req, ProgressHooks{})
}

// CurateAndStoreWithHooks 将候选模式策展为规范模式并写入模式库，并向调用方报告进度。
func (s *Service) CurateAndStoreWithHooks(ctx context.Context, req CurateRequest, hooks ProgressHooks) (*CurateResult, error) {
	if !req.Operation.Valid() || req.Operation == OperationCompact {
		return nil, fmt.Errorf("unsupported curate operation %q", req.Operation)
	}
	candidates := validateCandidates(req.Candidates)
	if req.Operation == OperationLearnCurrent {
		candidates = coalesceCurrentCandidates(validateCurrentCandidates(candidates))
	}
	if len(candidates) == 0 {
		return &CurateResult{
			Summary: Summary{
				TotalCandidates: len(req.Candidates),
			},
		}, nil
	}

	existing, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("load existing patterns: %w", err)
	}
	existing = activeCuratorPatterns(existing)
	retrieved := retrieveRelatedPatterns(candidates, existing, relatedPatternsPerCandidate)
	var curated *proposal
	if req.Operation == OperationLearnCurrent {
		curated, err = s.curateCurrent(ctx, candidates, retrieved, req.DecisionCheckpoint, hooks)
		if err != nil {
			return nil, fmt.Errorf("curate current patterns: %w", err)
		}
	} else {
		curated = deterministicCurate(candidates, retrieved.related)
	}
	notifyProgress(hooks.OnValidationStart, i18n.Get("ProgressCuratePatternsValidation"))
	if err := validateCurateResultForOperation(req.Operation, curated, candidates, retrieved.related); err != nil {
		if req.Operation == OperationLearnCurrent {
			return nil, fmt.Errorf("validate current curation: %w", err)
		}
		return nil, fmt.Errorf("validate deterministic curation: %w", err)
	}

	notifyProgress(hooks.OnStoreStart, i18n.Get("ProgressCuratePatternsStore"))
	written, err := applyCuratedPatterns(ctx, s.patternRepo, curated.Patterns, curated.Dropped, retrieved.related, storeCandidates)
	if err != nil {
		return nil, fmt.Errorf("apply curated patterns: %w", err)
	}
	return &CurateResult{
		Written: written,
		Dropped: curated.Dropped,
		Summary: summarizeCuration(len(candidates), len(retrieved.related), written, curated.Dropped),
	}, nil
}

func notifyProgress(callback func(string), label string) {
	if callback != nil {
		callback(label)
	}
}

func activeCuratorPatterns(patterns []domain.Pattern) []domain.Pattern {
	out := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.IsActive() {
			out = append(out, pattern)
		}
	}
	return out
}

func (s *Service) curate(ctx context.Context, operation Operation, candidates, existing []domain.Pattern, allExisting bool, existingByCandidate map[string][]string, checkpoint DecisionCheckpoint, hooks ProgressHooks) (*proposal, error) {
	request := &agent.CuratePatternsRequest{
		Operation:           string(operation),
		CandidatePatterns:   candidates,
		ExistingPatterns:    existing,
		AllExisting:         allExisting,
		ExistingByCandidate: existingByCandidate,
	}
	decisionKey, err := curationDecisionKey(candidates)
	if err != nil {
		return nil, err
	}
	if checkpoint != nil {
		result, found, err := checkpoint.Load(ctx, decisionKey)
		if err != nil {
			return nil, fmt.Errorf("load curation decision: %w", err)
		}
		if found {
			if err := agent.RequireResult(result, "CuratePatterns"); err != nil {
				return nil, fmt.Errorf("load curation decision: %w", err)
			}
			label := i18n.Get("ProgressCuratePatternsReplay")
			notifyProgress(hooks.OnStepStart, label)
			notifyProgress(hooks.OnStepComplete, label)
			return proposalFromAgent(result), nil
		}
	}
	if s.agent == nil {
		return nil, fmt.Errorf("pattern curator agent is not configured")
	}

	retryProgress := agent.NewRetryProgressBinder(hooks.OnStepUpdate)
	ctx = retryProgress.WithContext(ctx)
	label := i18n.Get("ProgressCuratePatternsAI")
	if hooks.OnStepStart != nil {
		hooks.OnStepStart(label)
	}
	retryProgress.StartStep(label)
	result, callErr := s.agent.CuratePatterns(ctx, request)
	retryProgress.FinishStep(label, callErr == nil)
	if hooks.OnStepComplete != nil {
		hooks.OnStepComplete(label)
	}
	if callErr != nil {
		return nil, callErr
	}
	if err := agent.RequireResult(result, "CuratePatterns"); err != nil {
		return nil, err
	}
	if checkpoint != nil {
		if err := checkpoint.Save(ctx, decisionKey, result); err != nil {
			return nil, fmt.Errorf("save curation decision: %w", err)
		}
	}
	return proposalFromAgent(result), nil
}

func curationDecisionKey(candidates []domain.Pattern) (string, error) {
	data, err := json.Marshal(candidates)
	if err != nil {
		return "", fmt.Errorf("hash curation candidates: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func logCurationAssessment(operation Operation, assessment curationAssessment) {
	if len(assessment.IgnoredDroppedIDs) == 0 && len(assessment.IgnoredConflictingDroppedIDs) == 0 && len(assessment.IgnoredMergedFromIDs) == 0 && len(assessment.IgnoredPatternIDs) == 0 && len(assessment.ResolvedOwnershipIDs) == 0 && assessment.Coverage.MissingCount() == 0 {
		return
	}
	logger.Info(i18n.Get("LoggerAgentCuratePatternsSanitized"),
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
