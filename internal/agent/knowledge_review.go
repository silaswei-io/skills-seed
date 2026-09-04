package agent

import (
	"fmt"
	"strings"
)

// ValidKnowledgeReviewVerdict 判断知识评审结论是否属于结构化契约允许的集合。
func ValidKnowledgeReviewVerdict(verdict string) bool {
	switch strings.TrimSpace(verdict) {
	case "accept", "revise", "reject":
		return true
	default:
		return false
	}
}

// ValidKnowledgeReviewReasonCode 判断知识评审原因是否属于结构化契约允许的集合。
func ValidKnowledgeReviewReasonCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "accepted", "unsupported_evidence", "contradictory", "unsafe_guidance", "no_routeable_value", "low_signal_boilerplate", "overclaimed", "incorrect_boundary":
		return true
	default:
		return false
	}
}

// ValidateKnowledgeReviewCandidates 校验评审回执是否完整覆盖本次输入候选。
// 候选身份由调用方提供，避免把模型生成的 ID 当成新的事实来源。
func ValidateKnowledgeReviewCandidates(decisions []KnowledgeReviewDecision, candidateIDs []string) error {
	expected := make(map[string]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			expected[id] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(decisions))
	for _, decision := range decisions {
		id := strings.TrimSpace(decision.CandidateID)
		if id == "" {
			return fmt.Errorf("knowledge review has an empty candidate id")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("knowledge review repeats candidate %q", id)
		}
		if _, ok := expected[id]; !ok {
			return fmt.Errorf("knowledge review references unknown candidate %q", id)
		}
		seen[id] = struct{}{}
	}
	for id := range expected {
		if _, ok := seen[id]; !ok {
			return fmt.Errorf("knowledge review has no decision for candidate %q", id)
		}
	}
	return nil
}
