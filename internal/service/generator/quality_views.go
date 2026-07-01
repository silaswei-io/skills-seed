package generator

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func patternImportanceGroups(patterns []domain.Pattern, locale string) []PatternImportanceGroup {
	if len(patterns) == 0 {
		return nil
	}
	groups := []PatternImportanceGroup{
		{Title: localizedText(locale, "核心开发路径", "Core Development Paths"), Description: localizedText(locale, "高置信、跨文件或高频出现，改动相关能力时应优先读取。", "High-confidence, cross-file, or frequent patterns to read first for related changes.")},
		{Title: localizedText(locale, "常用项目约定", "Common Project Conventions"), Description: localizedText(locale, "覆盖常见开发动作，但需要结合当前改动范围判断是否适用。", "Common development conventions that still need change-scope judgment.")},
		{Title: localizedText(locale, "局部模块经验", "Local Module Experience"), Description: localizedText(locale, "证据集中在少数模块，适合命中相邻路径时参考。", "Evidence is concentrated in a few modules; use when working in adjacent paths.")},
		{Title: localizedText(locale, "参考观察", "Reference Observations"), Description: localizedText(locale, "证据较弱或偏归纳，只能作为导航线索，不能作为硬约束。", "Weaker or more inferential findings; use as navigation hints, not hard constraints.")},
	}
	for _, pattern := range patterns {
		pattern = patternForTemplate(pattern)
		index := patternImportanceIndex(pattern)
		groups[index].Patterns = append(groups[index].Patterns, pattern)
	}
	result := make([]PatternImportanceGroup, 0, len(groups))
	for _, group := range groups {
		if len(group.Patterns) == 0 {
			continue
		}
		sort.SliceStable(group.Patterns, func(i, j int) bool {
			left := group.Patterns[i]
			right := group.Patterns[j]
			if left.Confidence != right.Confidence {
				return left.Confidence > right.Confidence
			}
			if left.Frequency != right.Frequency {
				return left.Frequency > right.Frequency
			}
			return left.Name < right.Name
		})
		result = append(result, group)
	}
	return result
}

func patternImportanceIndex(pattern domain.Pattern) int {
	evidenceCount := len(pattern.EvidenceLocations)
	if pattern.BusinessMethod != nil && strings.TrimSpace(pattern.BusinessMethod.DisplayLocation()) != "" {
		evidenceCount++
	}
	if pattern.Metrics.EvidenceCount > evidenceCount {
		evidenceCount = pattern.Metrics.EvidenceCount
	}
	if pattern.Confidence >= 0.90 && pattern.Frequency >= 2 && evidenceCount >= 2 {
		return 0
	}
	if pattern.Confidence >= 0.85 && (pattern.Frequency >= 2 || evidenceCount >= 2) {
		return 1
	}
	if pattern.Confidence >= 0.70 && (evidenceCount >= 1 || strings.TrimSpace(pattern.ScopePath) != "") {
		return 2
	}
	return 3
}
