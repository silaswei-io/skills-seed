package patternnorm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

// ReviewCurrentKnowledge 对一个完整证据焦点的候选知识执行独立审查。
// 跨焦点合并留给后续全局规范化，避免审查阶段丢失证据边界。
func (s *Service) ReviewCurrentKnowledge(ctx context.Context, req ReviewRequest) ([]domain.Pattern, error) {
	candidates := validateCandidates(req.Candidates)
	candidates = coalesceCurrentCandidates(s.validateCurrentCandidates(candidates))
	return s.reviewCurrentKnowledge(ctx, req, candidates)
}

func (s *Service) reviewCurrentKnowledge(ctx context.Context, req ReviewRequest, candidates []domain.Pattern) ([]domain.Pattern, error) {
	if len(candidates) == 0 || s.reviewer == nil {
		return candidates, nil
	}
	ordered := append([]domain.Pattern(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})

	result, err := s.reviewer.ReviewKnowledge(ctx, &agent.ReviewKnowledgeRequest{
		ProjectName:   req.ProjectName,
		RootPath:      req.RootPath,
		Language:      req.Language,
		RuntimeLabel:  req.RuntimeLabel,
		EvidenceFocus: req.Focus,
		Candidates:    ordered,
		UserContext:   req.UserContext,
		Conversation:  req.Conversation,
	})
	if err != nil {
		return nil, err
	}
	if err := agent.RequireResult(result, "ReviewKnowledge"); err != nil {
		return nil, err
	}
	return applyKnowledgeReview(ordered, result.Decisions)
}

func applyKnowledgeReview(candidates []domain.Pattern, decisions []agent.KnowledgeReviewDecision) ([]domain.Pattern, error) {
	byID := make(map[string]agent.KnowledgeReviewDecision, len(decisions))
	for _, decision := range decisions {
		id := strings.TrimSpace(decision.CandidateID)
		if id == "" {
			return nil, fmt.Errorf("%s", i18n.Get("PatternReviewEmptyCandidateID"))
		}
		if _, exists := byID[id]; exists {
			return nil, fmt.Errorf("knowledge review repeats candidate %q", id)
		}
		byID[id] = decision
	}

	reviewed := make([]domain.Pattern, 0, len(candidates))
	for _, candidate := range candidates {
		if !domain.ValidKnowledgeFlags(candidate.KnowledgeFlags) {
			return nil, fmt.Errorf("knowledge candidate %q has invalid flags", candidate.ID)
		}
		decision, ok := byID[candidate.ID]
		if !ok {
			return nil, fmt.Errorf("knowledge review has no decision for candidate %q", candidate.ID)
		}
		delete(byID, candidate.ID)
		if strings.TrimSpace(decision.Reason) == "" || strings.TrimSpace(decision.ReasonCode) == "" {
			return nil, fmt.Errorf("knowledge review decision %q has no reason", candidate.ID)
		}
		candidate.BusinessMethod = reviewedBusinessMethod(decision.BusinessMethod)
		switch decision.Verdict {
		case "accept":
			if decision.Revision != nil {
				return nil, fmt.Errorf("accepted knowledge %q unexpectedly contains a revision", candidate.ID)
			}
			reviewed = append(reviewed, candidate)
		case "revise":
			if decision.Revision == nil {
				return nil, fmt.Errorf("revised knowledge %q has no revision", candidate.ID)
			}
			candidate.Name = decision.Revision.Name
			candidate.Category = domain.Category(decision.Revision.Category)
			candidate.Description = decision.Revision.Description
			candidate.Rule = decision.Revision.Rule
			candidate.Confidence = decision.Revision.Confidence
			if !domain.ValidKnowledgeFlags(decision.Revision.KnowledgeFlags) {
				return nil, fmt.Errorf("knowledge review revision %q has invalid flags", candidate.ID)
			}
			candidate.KnowledgeFlags = domain.CanonicalKnowledgeFlags(decision.Revision.KnowledgeFlags)
			if !candidate.IsValid() || !domain.IsValidPatternCategory(candidate.Category) {
				return nil, fmt.Errorf("knowledge review revision %q is invalid", candidate.ID)
			}
			candidate.RefreshMetrics()
			reviewed = append(reviewed, candidate)
		case "reject":
			if decision.Revision != nil {
				return nil, fmt.Errorf("rejected knowledge %q unexpectedly contains a revision", candidate.ID)
			}
		default:
			return nil, fmt.Errorf("knowledge review decision %q has invalid verdict %q", candidate.ID, decision.Verdict)
		}
	}
	if len(byID) > 0 {
		for id := range byID {
			return nil, fmt.Errorf("knowledge review references unknown candidate %q", id)
		}
	}
	return reviewed, nil
}

// reviewedBusinessMethod 只保留可完整投影为能力入口的审查补充。
// 能力入口是可选索引，字段不完整时不能影响已验证知识的准入。
func reviewedBusinessMethod(method *domain.BusinessMethod) *domain.BusinessMethod {
	if method == nil {
		return nil
	}
	method = cloneBusinessMethod(method)
	if !domain.IsRouteableBusinessMethod(*method) || (method.Type != "domain" && method.Type != "common") {
		return nil
	}
	if pathx.CleanEvidenceLocationPath(method.DisplayLocation()) == "" {
		return nil
	}
	return method
}
