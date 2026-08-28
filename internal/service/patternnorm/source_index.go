package patternnorm

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// indexNormalizationSources 为每个来源 ID 选择当前有效的 Pattern。
// 当前候选表示同 ID Pattern 的新版本，必须覆盖模式库中的已有记录。
func indexNormalizationSources(candidates, existing []domain.Pattern) map[string]domain.Pattern {
	indexed := make(map[string]domain.Pattern, len(candidates)+len(existing))
	candidateIDs := patternIDSet(candidates)
	effective := make([]domain.Pattern, 0, len(candidates)+len(existing))
	for _, pattern := range existing {
		indexed[pattern.ID] = pattern
		if _, replaced := candidateIDs[pattern.ID]; !replaced {
			effective = append(effective, pattern)
		}
	}
	for _, pattern := range candidates {
		indexed[pattern.ID] = pattern
		effective = append(effective, pattern)
	}

	// 已水合结果会携带历史输出谱系；仅无歧义谱系可以回指当前有效 Pattern。
	lineage := make(map[string]domain.Pattern)
	ambiguous := make(map[string]struct{})
	for _, pattern := range effective {
		for _, sourceID := range pattern.MergedFrom {
			sourceID = strings.TrimSpace(sourceID)
			if sourceID == "" {
				continue
			}
			if _, direct := indexed[sourceID]; direct {
				continue
			}
			if owner, exists := lineage[sourceID]; exists && owner.ID != pattern.ID {
				delete(lineage, sourceID)
				ambiguous[sourceID] = struct{}{}
				continue
			}
			if _, conflict := ambiguous[sourceID]; !conflict {
				lineage[sourceID] = pattern
			}
		}
	}
	for sourceID, pattern := range lineage {
		indexed[sourceID] = pattern
	}
	return indexed
}
