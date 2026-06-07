package domain

import (
	"sort"
	"time"
)

// StrongestPatterns 按 confidence 和 frequency 排序后取 top N
func StrongestPatterns(patterns []Pattern, limit int) []Pattern {
	filtered := make([]Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.Name == "" {
			continue
		}
		filtered = append(filtered, pattern)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Confidence == filtered[j].Confidence {
			if filtered[i].Frequency == filtered[j].Frequency {
				return filtered[i].Name < filtered[j].Name
			}
			return filtered[i].Frequency > filtered[j].Frequency
		}
		return filtered[i].Confidence > filtered[j].Confidence
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

// PatternNames 提取模式名称列表
func PatternNames(patterns []Pattern) []string {
	names := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern.Name != "" {
			names = append(names, pattern.Name)
		}
	}
	return names
}

// CategoryNamesWithPatterns 返回有模式的分类名（去重排序）
func CategoryNamesWithPatterns(patterns []Pattern) []string {
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		category := string(pattern.Category)
		if category == "" {
			continue
		}
		seen[category] = true
	}

	categories := make([]string, 0, len(seen))
	for category := range seen {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
}

// PatternInsight 记录单个模式在生成阶段的辅助信息
type PatternInsight struct {
	HitCount       int
	LastHitAt      time.Time
	GenerationRank float64
}

// RankPatternsForGeneration 根据洞察分数对模式排序，用于生成优先级
func RankPatternsForGeneration(patterns []Pattern, insights map[string]PatternInsight) []Pattern {
	ranked := append([]Pattern(nil), patterns...)
	maxHits := 0
	for _, insight := range insights {
		if insight.HitCount > maxHits {
			maxHits = insight.HitCount
		}
	}
	for i, pattern := range ranked {
		insight := insights[pattern.ID]
		insight.GenerationRank = patternGenerationRank(pattern, insight.HitCount, maxHits)
		insights[pattern.ID] = insight
		ranked[i] = pattern
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		left := insights[ranked[i].ID].GenerationRank
		right := insights[ranked[j].ID].GenerationRank
		if left != right {
			return left > right
		}
		if ranked[i].Metrics.EffectiveScore != ranked[j].Metrics.EffectiveScore {
			return ranked[i].Metrics.EffectiveScore > ranked[j].Metrics.EffectiveScore
		}
		if insights[ranked[i].ID].HitCount != insights[ranked[j].ID].HitCount {
			return insights[ranked[i].ID].HitCount > insights[ranked[j].ID].HitCount
		}
		if ranked[i].Confidence != ranked[j].Confidence {
			return ranked[i].Confidence > ranked[j].Confidence
		}
		return ranked[i].ID < ranked[j].ID
	})
	return ranked
}

func patternGenerationRank(pattern Pattern, hitCount, maxHits int) float64 {
	normalizedHits := 0.0
	if maxHits > 0 {
		normalizedHits = float64(hitCount) / float64(maxHits)
	}
	return roundFloat(pattern.Metrics.EffectiveScore*0.6 + normalizedHits*0.3 + pattern.Confidence*0.1)
}

func roundFloat(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
