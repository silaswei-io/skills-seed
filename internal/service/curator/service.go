package curator

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

// Service 是模式库唯一的规范入库边界。
type Service struct {
	agent       agent.Agent
	patternRepo domain.PatternRepository
}

// NewService 创建模式策展服务。
func NewService(ag agent.Agent, repo domain.PatternRepository) *Service {
	return &Service{
		agent:       ag,
		patternRepo: repo,
	}
}

// CurateAndStore 将候选模式策展为规范模式并写入模式库。
func (s *Service) CurateAndStore(ctx context.Context, req CurateRequest) (*CurateResult, error) {
	return s.CurateAndStoreWithHooks(ctx, req, ProgressHooks{})
}

// CurateAndStoreWithHooks 将候选模式策展为规范模式并写入模式库，并向调用方报告进度。
func (s *Service) CurateAndStoreWithHooks(ctx context.Context, req CurateRequest, hooks ProgressHooks) (*CurateResult, error) {
	candidates := validateCandidates(req.Candidates)
	if len(candidates) == 0 {
		return &CurateResult{
			Summary: agent.CurateSummary{
				TotalCandidates: len(req.Candidates),
			},
		}, nil
	}

	existing, err := s.patternRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("load existing patterns: %w", err)
	}
	retrieved := retrieveRelatedPatterns(candidates, existing, relatedPatternsPerCandidate)
	curated := s.curate(ctx, req.Operation, candidates, retrieved.related, false, retrieved.existingByCandidate, hooks)
	if err := validateCurateResult(curated, candidates, retrieved.related); err != nil {
		logger.Warn(i18n.Get("LoggerAgentCuratePatternsValidationFallback"),
			"operation", req.Operation,
			"error", err,
		)
		curated = fallbackCurate(candidates, retrieved.related)
	}

	written, err := applyCuratedPatterns(ctx, s.patternRepo, curated.Patterns, retrieved.related)
	if err != nil {
		return nil, fmt.Errorf("apply curated patterns: %w", err)
	}
	curated.Summary.TotalCandidates = len(candidates)
	curated.Summary.TotalExisting = len(retrieved.related)
	curated.Summary.TotalWritten = len(written)
	curated.Summary.TotalDropped = len(curated.Dropped)
	return &CurateResult{
		Written: written,
		Dropped: curated.Dropped,
		Summary: curated.Summary,
	}, nil
}

// Compact 整理已有模式库。它是显式维护操作，不属于 skills 生成阶段。
func (s *Service) Compact(ctx context.Context, req CompactRequest) (*CompactResult, error) {
	return s.CompactWithHooks(ctx, req, ProgressHooks{})
}

// CompactWithHooks 整理已有模式库，并向调用方报告进度。
func (s *Service) CompactWithHooks(ctx context.Context, req CompactRequest, hooks ProgressHooks) (*CompactResult, error) {
	var patterns []domain.Pattern
	var err error
	if req.Category != "" {
		patterns, err = s.patternRepo.GetByCategory(ctx, domain.Category(req.Category))
	} else {
		patterns, err = s.patternRepo.GetAll(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("load patterns: %w", err)
	}
	patterns = validateCandidates(patterns)
	if len(patterns) == 0 {
		return &CompactResult{Summary: agent.CurateSummary{}}, nil
	}

	byCandidate := make(map[string][]string, len(patterns))
	for _, pattern := range patterns {
		for _, related := range patterns {
			if pattern.ID == related.ID {
				continue
			}
			if patternSimilarity(pattern, related) > 0 {
				byCandidate[pattern.ID] = append(byCandidate[pattern.ID], related.ID)
			}
		}
	}
	curated := s.curate(ctx, OperationCompact, patterns, patterns, true, byCandidate, hooks)
	if err := validateCurateResult(curated, patterns, patterns); err != nil {
		logger.Warn(i18n.Get("LoggerAgentCuratePatternsValidationFallback"),
			"operation", OperationCompact,
			"error", err,
		)
		curated = fallbackCurate(patterns, patterns)
	}
	curated.Summary.TotalCandidates = len(patterns)
	curated.Summary.TotalExisting = len(patterns)
	curated.Summary.TotalWritten = len(curated.Patterns)
	curated.Summary.TotalDropped = len(curated.Dropped)

	if req.DryRun {
		return &CompactResult{
			Written: curatedPatternsToDomain(curated.Patterns),
			Dropped: curated.Dropped,
			Summary: curated.Summary,
		}, nil
	}

	written, err := applyCuratedPatterns(ctx, s.patternRepo, curated.Patterns, patterns)
	if err != nil {
		return nil, fmt.Errorf("apply compacted patterns: %w", err)
	}
	curated.Summary.TotalWritten = len(written)
	return &CompactResult{
		Written: written,
		Dropped: curated.Dropped,
		Summary: curated.Summary,
	}, nil
}

func (s *Service) curate(ctx context.Context, operation string, candidates, existing []domain.Pattern, allExisting bool, existingByCandidate map[string][]string, hooks ProgressHooks) *agent.CuratePatternsResult {
	if s.agent == nil {
		return fallbackCurate(candidates, existing)
	}

	var result *agent.CuratePatternsResult
	retryProgress := agent.NewRetryProgressBinder(hooks.OnStepUpdate)
	ctx = retryProgress.WithContext(ctx)
	label := i18n.Get("ProgressCuratePatternsAI")
	if hooks.OnStepStart != nil {
		hooks.OnStepStart(label)
	}
	retryProgress.StartStep(label)
	var callErr error
	result, callErr = s.agent.CuratePatterns(ctx, &agent.CuratePatternsRequest{
		Operation:           operation,
		CandidatePatterns:   candidates,
		ExistingPatterns:    existing,
		AllExisting:         allExisting,
		ExistingByCandidate: existingByCandidate,
	})
	retryProgress.FinishStep(label, callErr == nil)
	if hooks.OnStepComplete != nil {
		hooks.OnStepComplete(label)
	}
	if callErr != nil {
		logger.Warn(i18n.Get("LoggerAgentCuratePatternsParseFallback"),
			"operation", operation,
			"error", callErr,
		)
		return fallbackCurate(candidates, existing)
	}
	return result
}

func curatedPatternsToDomain(patterns []agent.CuratedPattern) []domain.Pattern {
	result := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		result = append(result, curatedToDomain(pattern))
	}
	return result
}
