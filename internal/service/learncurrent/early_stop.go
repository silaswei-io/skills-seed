package learncurrent

import (
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// 早停与审查路由策略：纯函数、无 I/O。
// 编排层只消费路由结果，不在此写库或调用 Agent。

const (
	skipReviewEmpty         = "review_empty"
	skipReviewLocalStandard = "review_local_standard"
	skipNormalizeEmpty      = "normalize_empty"
	// skipNormalizeNoRelation 表示候选互无关联且无既有可合并对象，规范化 AI 已短路。
	skipNormalizeNoRelation = "normalize_no_relation"
	// skipProfileUnchanged 表示画像/权威无需刷新。
	skipProfileUnchanged = "profile_unchanged"
)

// reviewRoute 描述焦点审查应走的路径。
type reviewRoute string

const (
	// reviewRouteNone 无候选，本地空审查收口。
	reviewRouteNone reviewRoute = "none"
	// reviewRouteLocalStandard standard 深度且通过本地硬闸，本地接受。
	reviewRouteLocalStandard reviewRoute = "local_standard"
	// reviewRouteAI 需要独立 AI 审查。
	reviewRouteAI reviewRoute = "ai"
)

// routeFocusReview 决定焦点审查路径。
// 失败默认走 AI（fail closed），避免本地硬闸误伤高价值候选。
func routeFocusReview(focus domain.EvidenceFocus, patterns []domain.Pattern) reviewRoute {
	if !focusNeedsIndependentReview(patterns) {
		return reviewRouteNone
	}
	if requiresAIReview(focus, patterns) {
		return reviewRouteAI
	}
	if passLocalStandardReview(patterns) {
		return reviewRouteLocalStandard
	}
	return reviewRouteAI
}

// focusNeedsIndependentReview 判断是否仍存在待审查候选。
func focusNeedsIndependentReview(patterns []domain.Pattern) bool {
	return len(patterns) > 0
}

// knowledgeNeedsNormalizeStore 判断是否需要规范化入库路径（含仅退役）。
func knowledgeNeedsNormalizeStore(patterns []domain.Pattern, retiredIDs []string) bool {
	return len(patterns) > 0 || len(retiredIDs) > 0
}

func requiresAIReview(focus domain.EvidenceFocus, patterns []domain.Pattern) bool {
	switch focus.EffectiveAnalysisDepth() {
	case domain.EvidenceFocusDepthCareful, domain.EvidenceFocusDepthCritical:
		return true
	}
	for _, pattern := range patterns {
		if hasOperationalRisk(pattern) {
			return true
		}
	}
	return false
}

func hasOperationalRisk(pattern domain.Pattern) bool {
	for _, flag := range pattern.KnowledgeFlags {
		if strings.TrimSpace(flag) == domain.KnowledgeFlagOperationalRisk {
			return true
		}
	}
	return false
}

// passLocalStandardReview 对 standard 焦点做保守本地硬闸。
// 只证明“结构完整、有证据路径、标志合法”，不替代 careful/critical 的语义审查。
func passLocalStandardReview(patterns []domain.Pattern) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if !localStandardCandidateOK(pattern) {
			return false
		}
	}
	return true
}

func localStandardCandidateOK(pattern domain.Pattern) bool {
	if strings.TrimSpace(pattern.ID) == "" || strings.TrimSpace(pattern.Name) == "" {
		return false
	}
	if strings.TrimSpace(pattern.Rule) == "" && strings.TrimSpace(pattern.Description) == "" {
		return false
	}
	if !domain.IsValidPatternCategory(pattern.Category) {
		return false
	}
	if !domain.ValidKnowledgeFlags(pattern.KnowledgeFlags) {
		return false
	}
	if hasOperationalRisk(pattern) {
		return false
	}
	if domain.PatternEvidenceFileCount(pattern.EvidenceLocations) < 1 {
		return false
	}
	for _, location := range pattern.EvidenceLocations {
		path := strings.TrimSpace(location.Path)
		if path == "" || strings.Contains(path, "..") {
			return false
		}
	}
	if pattern.Confidence < 0 || pattern.Confidence > 1 {
		return false
	}
	return true
}
