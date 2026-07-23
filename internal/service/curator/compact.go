package curator

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

	curated, err := s.planCompaction(ctx, patterns, req.UseAI, hooks)
	if err != nil {
		return nil, err
	}
	if req.DryRun {
		written := patternsForSave(curated.Patterns)
		return &CompactResult{
			Written: written,
			Dropped: curated.Dropped,
			Summary: summarizeCuration(len(patterns), len(patterns), written, curated.Dropped),
		}, nil
	}

	written, err := applyCuratedPatterns(ctx, s.patternRepo, curated.Patterns, curated.Dropped, patterns, compactLibrary)
	if err != nil {
		return nil, fmt.Errorf("apply compacted patterns: %w", err)
	}
	return &CompactResult{
		Written: written,
		Dropped: curated.Dropped,
		Summary: summarizeCuration(len(patterns), len(patterns), written, curated.Dropped),
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

func (s *Service) planCompaction(ctx context.Context, patterns []domain.Pattern, useAI bool, hooks ProgressHooks) (*proposal, error) {
	curated := deterministicCurate(patterns, nil)
	if useAI {
		var err error
		curated, err = s.curate(ctx, OperationCompact, patterns, patterns, true, compactRelatedPatterns(patterns), nil, hooks)
		if err != nil {
			return nil, fmt.Errorf("compact patterns with AI: %w", err)
		}
	}
	assessment := assessCuration(curated, patterns, patterns)
	logCurationAssessment(OperationCompact, assessment)
	if assessment.Coverage.MissingCount() > 0 {
		return nil, fmt.Errorf("compact curation left %d of %d patterns unclassified", assessment.Coverage.MissingCount(), assessment.Coverage.CandidateCount)
	}
	if useAI {
		if err := hydrateCurateResult(assessment.Result, patterns, patterns); err != nil {
			return nil, fmt.Errorf("hydrate compact curation: %w", err)
		}
	}
	if err := validateCurateResult(assessment.Result, patterns, patterns); err != nil {
		return nil, fmt.Errorf("validate compact curation: %w", err)
	}
	return assessment.Result, nil
}

func compactRelatedPatterns(patterns []domain.Pattern) map[string][]string {
	byCandidate := make(map[string][]string, len(patterns))
	for _, pattern := range patterns {
		for _, related := range patterns {
			if pattern.ID != related.ID && patternSimilarity(pattern, related) > 0 {
				byCandidate[pattern.ID] = append(byCandidate[pattern.ID], related.ID)
			}
		}
	}
	return byCandidate
}
