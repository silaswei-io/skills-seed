package patternnorm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
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
		EvidenceFocus: req.Focus,
		Candidates:    ordered,
		UserContext:   req.UserContext,
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
			return nil, fmt.Errorf("knowledge review has empty candidate id")
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
		if decision.Verdict == "reject" && decision.BusinessMethodVerdict != "remove" {
			return nil, fmt.Errorf("rejected knowledge %q must remove its business method", candidate.ID)
		}
		method, err := reviewedBusinessMethod(candidate, decision)
		if err != nil {
			return nil, err
		}
		candidate.BusinessMethod = method
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

func reviewedBusinessMethod(candidate domain.Pattern, decision agent.KnowledgeReviewDecision) (*domain.BusinessMethod, error) {
	switch decision.BusinessMethodVerdict {
	case "remove":
		if decision.BusinessMethod != nil {
			return nil, fmt.Errorf("knowledge review decision %q removes a business method but also returns a replacement", candidate.ID)
		}
		return nil, nil
	case "set":
		if decision.BusinessMethod == nil {
			return nil, fmt.Errorf("knowledge review decision %q sets a business method without a replacement", candidate.ID)
		}
		method := cloneBusinessMethod(decision.BusinessMethod)
		if !domain.IsRouteableBusinessMethod(*method) || (method.Type != "domain" && method.Type != "common") {
			return nil, fmt.Errorf("knowledge review decision %q sets an incomplete business method", candidate.ID)
		}
		if businessMethodWithinCandidateEvidence(candidate, *method) {
			return method, nil
		}
		return nil, fmt.Errorf("knowledge review decision %q sets a business method outside candidate evidence", candidate.ID)
	default:
		return nil, fmt.Errorf("knowledge review decision %q has invalid business method verdict %q", candidate.ID, decision.BusinessMethodVerdict)
	}
}

func businessMethodWithinCandidateEvidence(candidate domain.Pattern, method domain.BusinessMethod) bool {
	location := pathx.CleanEvidenceLocationPath(method.DisplayLocation())
	if location == "" {
		return false
	}
	if candidate.BusinessMethod != nil && location == pathx.CleanEvidenceLocationPath(candidate.BusinessMethod.DisplayLocation()) {
		return true
	}
	for _, evidence := range candidate.EvidenceLocations {
		if location == pathx.CleanEvidenceLocationPath(evidence.Path) {
			return true
		}
	}
	return false
}
