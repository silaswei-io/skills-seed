package generator

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func (s *GeneratorService) patternGenerationInsights(ctx context.Context, patterns []domain.Pattern) (map[string]domain.PatternInsight, error) {
	insights := make(map[string]domain.PatternInsight, len(patterns))
	for _, pattern := range patterns {
		insights[pattern.ID] = domain.PatternInsight{}
	}
	if s.patternStatsRepo == nil {
		return insights, nil
	}

	stats, err := s.patternStatsRepo.GetPatternHitStats(ctx)
	if err != nil {
		return nil, err
	}
	for _, stat := range stats {
		if _, ok := insights[stat.Pattern.ID]; !ok {
			continue
		}
		insights[stat.Pattern.ID] = domain.PatternInsight{
			HitCount:  stat.HitCount,
			LastHitAt: stat.LastHitAt,
		}
	}
	return insights, nil
}

func rankPatternsForGeneration(patterns []domain.Pattern, insights map[string]domain.PatternInsight) []domain.Pattern {
	return domain.RankPatternsForGeneration(patterns, insights)
}

// calculateStats 计算统计信息
func (s *GeneratorService) calculateStats(patterns []domain.Pattern) *Stats {
	stats := &Stats{
		Total:      len(patterns),
		ByCategory: make(map[string][]domain.Pattern),
	}

	if len(patterns) == 0 {
		return stats
	}

	var totalConfidence float64
	for _, p := range patterns {
		totalConfidence += p.Confidence
	}
	stats.AvgConfidence = totalConfidence / float64(len(patterns))

	for _, p := range patterns {
		category := string(p.Category)
		stats.ByCategory[category] = append(stats.ByCategory[category], p)
	}

	for _, p := range patterns {
		if p.Confidence > 0.8 {
			stats.HighConfidence = append(stats.HighConfidence, p)
		}
	}

	for _, p := range patterns {
		if p.Frequency > 3 {
			stats.Frequent = append(stats.Frequent, p)
		}
	}

	return stats
}
