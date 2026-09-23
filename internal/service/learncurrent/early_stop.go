package learncurrent

import "github.com/silaswei-io/skills-seed/internal/domain"

// 早停策略：在知识流水线中决定哪些阶段可以确定性跳过，避免无价值的 Agent 调用。
// 策略纯函数、无 I/O，便于单测与编排层组合。

const (
	// skipReviewEmpty 表示分析后无候选，独立审查可本地完成。
	skipReviewEmpty = "review_empty"
	// skipNormalizeEmpty 表示无候选且无退役，规范化 AI/入库可跳过。
	skipNormalizeEmpty = "normalize_empty"
)

// focusNeedsIndependentReview 判断焦点是否仍需独立 AI 审查。
// 无候选时 patternnorm 本就不会调用 Agent；此处进一步避免进入审查队列与检查点往返。
func focusNeedsIndependentReview(patterns []domain.Pattern) bool {
	return len(patterns) > 0
}

// knowledgeNeedsNormalizeStore 判断是否需要进入规范化入库路径（含仅退役）。
func knowledgeNeedsNormalizeStore(patterns []domain.Pattern, retiredIDs []string) bool {
	return len(patterns) > 0 || len(retiredIDs) > 0
}
