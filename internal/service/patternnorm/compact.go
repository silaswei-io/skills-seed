package patternnorm

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// Compact 整理已有模式库。它是显式维护操作，不属于 skills 生成阶段。
func (s *Service) Compact(ctx context.Context, req CompactRequest) (*CompactResult, error) {
	return s.CompactWithHooks(ctx, req, ProgressHooks{})
}

// CompactWithHooks 整理已有模式库，并向调用方报告进度。
func (s *Service) CompactWithHooks(ctx context.Context, req CompactRequest, hooks ProgressHooks) (*CompactResult, error) {
	patterns, err := s.loadCompactPatterns(ctx, req.Category)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return &CompactResult{Summary: Summary{}}, nil
	}

	normalized, err := s.planCompaction(patterns)
	if err != nil {
		return nil, err
	}
	if req.DryRun {
		written := patternsForSave(normalized.Patterns)
		return &CompactResult{
			Written: written,
			Dropped: normalized.Dropped,
			Summary: summarizeNormalization(len(patterns), len(patterns), written, normalized.Dropped),
		}, nil
	}

	written, err := applyNormalizedPatterns(ctx, s.patternRepo, normalized.Patterns, normalized.Dropped, patterns, compactLibrary)
	if err != nil {
		return nil, fmt.Errorf("apply compacted patterns: %w", err)
	}
	return &CompactResult{
		Written: written,
		Dropped: normalized.Dropped,
		Summary: summarizeNormalization(len(patterns), len(patterns), written, normalized.Dropped),
	}, nil
}

func (s *Service) loadCompactPatterns(ctx context.Context, categoryValue string) ([]domain.Pattern, error) {
	var (
		patterns []domain.Pattern
		err      error
	)
	if categoryValue == "" {
		patterns, err = s.patternRepo.GetAll(ctx)
	} else {
		category := domain.NormalizePatternCategory(domain.Category(categoryValue))
		if !domain.IsValidPatternCategory(category) {
			return nil, fmt.Errorf("invalid compact category %q", categoryValue)
		}
		patterns, err = s.patternRepo.GetByCategory(ctx, category)
	}
	if err != nil {
		return nil, fmt.Errorf("load patterns: %w", err)
	}
	return validateCandidates(patterns), nil
}

func (s *Service) planCompaction(patterns []domain.Pattern) (*proposal, error) {
	normalized := deterministicNormalize(patterns, nil)
	assessment := assessNormalization(normalized, patterns, patterns)
	logNormalizationAssessment(OperationCompact, assessment)
	if assessment.Coverage.MissingCount() > 0 {
		return nil, fmt.Errorf("compact normalization left %d of %d patterns unclassified", assessment.Coverage.MissingCount(), assessment.Coverage.CandidateCount)
	}
	if err := validateNormalizeResult(assessment.Result, patterns, patterns); err != nil {
		return nil, fmt.Errorf("validate compact normalization: %w", err)
	}
	return assessment.Result, nil
}
