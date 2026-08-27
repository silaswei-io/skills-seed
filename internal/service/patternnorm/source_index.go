package patternnorm

import "github.com/silaswei-io/skills-seed/internal/domain"

// indexNormalizationSources 为每个来源 ID 选择当前有效的 Pattern。
// 当前候选表示同 ID Pattern 的新版本，必须覆盖模式库中的已有记录。
func indexNormalizationSources(candidates, existing []domain.Pattern) map[string]domain.Pattern {
	indexed := make(map[string]domain.Pattern, len(candidates)+len(existing))
	for _, pattern := range existing {
		indexed[pattern.ID] = pattern
	}
	for _, pattern := range candidates {
		indexed[pattern.ID] = pattern
	}
	return indexed
}
