package patternnorm

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestApplyKnowledgeReviewRevisesTextAndPreservesOwnership(t *testing.T) {
	candidate := currentPattern("bounded-behavior", 0.9, "src/behavior.ext")
	candidate.Source = domain.SourceLearnedCurrent
	candidate.ScopePath = "src"
	candidate.BusinessMethod = &domain.BusinessMethod{Name: "Run"}
	candidate.KnowledgeFlags = []string{domain.KnowledgeFlagOperationalRisk}

	result, err := applyKnowledgeReview([]domain.Pattern{candidate}, []agent.KnowledgeReviewDecision{{
		CandidateID: "bounded-behavior", Verdict: "revise", ReasonCode: "overclaimed",
		Reason: "The evidence supports local behavior, not a global guarantee.", BusinessMethodVerdict: "remove",
		Revision: &agent.KnowledgeRevision{
			Name: "Bounded behavior", Category: string(domain.CategoryBusiness),
			Description: "The local implementation performs the observed behavior.",
			Rule:        "Inspect this implementation when extending the same local boundary.", Confidence: 0.86,
			KnowledgeFlags: []string{},
		},
	}})

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, candidate.ID, result[0].ID)
	require.Equal(t, candidate.Source, result[0].Source)
	require.Equal(t, candidate.ScopePath, result[0].ScopePath)
	require.Equal(t, candidate.EvidenceLocations, result[0].EvidenceLocations)
	require.Nil(t, result[0].BusinessMethod)
	require.Empty(t, result[0].KnowledgeFlags)
	require.Equal(t, "Bounded behavior", result[0].Name)
}

func TestApplyKnowledgeReviewSetsVerifiedBusinessMethod(t *testing.T) {
	candidate := currentPattern("state-transition", 0.95, "src/state.ext")
	method := &domain.BusinessMethod{
		Name: "State.Transition", CodeLocation: domain.CodeLocation{CurrentLocation: "src/state.ext:24"},
		Description: "Validates and applies a state transition.", Usage: "Use when changing the resource state.",
		Type: "domain", Function: "Transition(next State) error",
		Prerequisites: "The current and next states must form an allowed transition.",
		Returns:       "Returns nil after applying the transition or a validation error.",
	}

	result, err := applyKnowledgeReview([]domain.Pattern{candidate}, []agent.KnowledgeReviewDecision{{
		CandidateID: candidate.ID, Verdict: "accept", ReasonCode: "accepted",
		Reason: "The source proves a reusable state transition entry.", BusinessMethodVerdict: "set", BusinessMethod: method,
	}})

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, method.Name, result[0].BusinessMethod.Name)
	require.Equal(t, method.Function, result[0].BusinessMethod.Function)
}

func TestApplyKnowledgeReviewAllowsCandidateBusinessMethodLocationOutsideGeneralEvidence(t *testing.T) {
	candidate := currentPattern("state-transition", 0.95, "src/caller.ext")
	candidate.BusinessMethod = &domain.BusinessMethod{
		Name: "State.Transition", CodeLocation: domain.CodeLocation{CurrentLocation: "src/state.ext:24"},
		Description: "Validates and applies a state transition.", Usage: "Use when changing the resource state.",
		Type: "domain", Function: "Transition(next State) error",
		Prerequisites: "The current and next states must form an allowed transition.",
		Returns:       "Returns nil after applying the transition or a validation error.",
	}
	reviewed := *candidate.BusinessMethod
	reviewed.Description = "Validates the requested transition before applying it."

	result, err := applyKnowledgeReview([]domain.Pattern{candidate}, []agent.KnowledgeReviewDecision{{
		CandidateID: candidate.ID, Verdict: "accept", ReasonCode: "accepted",
		Reason: "The candidate capability location is directly verified.", BusinessMethodVerdict: "set", BusinessMethod: &reviewed,
	}})

	require.NoError(t, err)
	require.Equal(t, reviewed.Description, result[0].BusinessMethod.Description)
}

func TestApplyKnowledgeReviewRejectsBusinessMethodOutsideCandidateEvidence(t *testing.T) {
	candidate := currentPattern("state-transition", 0.95, "src/state.ext")
	method := &domain.BusinessMethod{
		Name: "State.Transition", CodeLocation: domain.CodeLocation{CurrentLocation: "src/other.ext:24"},
		Description: "Validates and applies a state transition.", Usage: "Use when changing the resource state.",
		Type: "domain", Function: "Transition(next State) error",
		Prerequisites: "The current and next states must form an allowed transition.",
		Returns:       "Returns nil after applying the transition or a validation error.",
	}

	_, err := applyKnowledgeReview([]domain.Pattern{candidate}, []agent.KnowledgeReviewDecision{{
		CandidateID: candidate.ID, Verdict: "accept", ReasonCode: "accepted",
		Reason: "The source proves a reusable state transition entry.", BusinessMethodVerdict: "set", BusinessMethod: method,
	}})

	require.ErrorContains(t, err, "outside candidate evidence")
}

func TestReviewCurrentKnowledgeKeepsCompleteFocusInOneRequest(t *testing.T) {
	candidates := []domain.Pattern{
		currentPattern("candidate-02", 0.9, "src/02.ext"),
		currentPattern("candidate-00", 0.9, "src/00.ext"),
		currentPattern("candidate-01", 0.9, "src/01.ext"),
	}
	focus := domain.EvidenceFocus{ID: "lifecycle", Name: "Lifecycle", EntryPaths: []string{"src"}}

	var calls int
	var receivedFocus domain.EvidenceFocus
	var receivedIDs []string
	service := NewService(&mocks.MockPatternRepository{})
	service.reviewer = &mocks.MockAgent{ReviewKnowledgeFn: func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		calls++
		receivedFocus = req.EvidenceFocus
		decisions := make([]agent.KnowledgeReviewDecision, 0, len(req.Candidates))
		for _, candidate := range req.Candidates {
			receivedIDs = append(receivedIDs, candidate.ID)
			decisions = append(decisions, agent.KnowledgeReviewDecision{
				CandidateID: candidate.ID, Verdict: "accept", ReasonCode: "accepted",
				Reason: "The evidence supports the candidate.", BusinessMethodVerdict: "remove",
			})
		}
		return &agent.ReviewKnowledgeResult{Decisions: decisions}, nil
	}}

	reviewed, err := service.ReviewCurrentKnowledge(context.Background(), ReviewRequest{
		Focus:      focus,
		Candidates: candidates,
	})

	require.NoError(t, err)
	require.Len(t, reviewed, len(candidates))
	require.Equal(t, 1, calls)
	require.Equal(t, focus, receivedFocus)
	require.Equal(t, []string{"candidate-00", "candidate-01", "candidate-02"}, receivedIDs)
}

func TestApplyKnowledgeReviewRejectsUnknownFlag(t *testing.T) {
	candidate := currentPattern("bounded-behavior", 0.9, "src/behavior.ext")
	candidate.KnowledgeFlags = []string{"invented"}

	_, err := applyKnowledgeReview([]domain.Pattern{candidate}, []agent.KnowledgeReviewDecision{{
		CandidateID: candidate.ID, Verdict: "accept", ReasonCode: "accepted",
		Reason: "The evidence supports the candidate.", BusinessMethodVerdict: "remove",
	}})

	require.ErrorContains(t, err, "invalid flags")
}

func TestApplyKnowledgeReviewRejectsUnknownRevisionFlag(t *testing.T) {
	candidate := currentPattern("bounded-behavior", 0.9, "src/behavior.ext")

	_, err := applyKnowledgeReview([]domain.Pattern{candidate}, []agent.KnowledgeReviewDecision{{
		CandidateID: candidate.ID, Verdict: "revise", ReasonCode: "overclaimed",
		Reason: "The wording needs revision.", BusinessMethodVerdict: "remove",
		Revision: &agent.KnowledgeRevision{
			Name: candidate.Name, Category: string(candidate.Category), Description: candidate.Description,
			Rule: candidate.Rule, Confidence: candidate.Confidence, KnowledgeFlags: []string{"invented"},
		},
	}})

	require.ErrorContains(t, err, "invalid flags")
}

func TestApplyKnowledgeReviewRejectsCandidate(t *testing.T) {
	candidate := currentPattern("boilerplate", 0.9, "src/wrapper.ext")

	result, err := applyKnowledgeReview([]domain.Pattern{candidate}, []agent.KnowledgeReviewDecision{{
		CandidateID: candidate.ID, Verdict: "reject", ReasonCode: "low_signal_boilerplate",
		Reason: "The evidence is a thin forwarding wrapper.", BusinessMethodVerdict: "remove",
	}})

	require.NoError(t, err)
	require.Empty(t, result)
}

func TestApplyKnowledgeReviewRequiresCompleteReceipt(t *testing.T) {
	candidate := currentPattern("missing", 0.9, "src/missing.ext")

	_, err := applyKnowledgeReview([]domain.Pattern{candidate}, nil)

	require.ErrorContains(t, err, "no decision")
}
