package analyzer

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestDeltaChangeAnchoredAllowsNewUnitWhenAnchorIsInFocus(t *testing.T) {
	focusByID := map[string]map[string]bool{
		"payment": {"internal/payment/pay.go": true},
	}
	change := domain.KnowledgeChange{
		FocusAction: domain.KnowledgeFocusNew,
		FocusID:     "new-payment-policy",
		Anchors:     []domain.PatternDiffAnchor{{Path: "internal/payment/pay.go"}},
	}

	require.True(t, deltaChangeAnchored(change, focusByID))
}

func TestDeltaChangeAnchoredRejectsNewUnitOutsideFocus(t *testing.T) {
	focusByID := map[string]map[string]bool{
		"payment": {"internal/payment/pay.go": true},
	}
	change := domain.KnowledgeChange{
		FocusAction: domain.KnowledgeFocusNew,
		FocusID:     "new-payment-policy",
		Anchors:     []domain.PatternDiffAnchor{{Path: "internal/order/create.go"}},
	}

	require.False(t, deltaChangeAnchored(change, focusByID))
}

func TestValidateDeltaChangesKeepsScopedNoChangeDecisionWithoutAnchor(t *testing.T) {
	svc := &AnalyzerService{}
	focusByID := map[string]map[string]bool{
		"app-config": {"src/common/init.ts": true},
	}
	changes := []domain.KnowledgeChange{{
		FocusAction:   domain.KnowledgeFocusNoChange,
		FocusID:       "app-config",
		PatternAction: domain.KnowledgePatternNoChange,
		Reason:        "changed files do not reveal reusable knowledge",
	}}

	validated, err := svc.validateDeltaChanges(context.Background(), "/repo", changes, focusByID)

	require.NoError(t, err)
	require.Len(t, validated, 1)
	require.Equal(t, "app-config", validated[0].FocusID)
}

func TestValidateDeltaChangesDropsUnscopedNoChangeDecisionWithoutAnchor(t *testing.T) {
	svc := &AnalyzerService{}
	focusByID := map[string]map[string]bool{
		"app-config": {"src/common/init.ts": true},
	}
	changes := []domain.KnowledgeChange{{
		FocusAction:   domain.KnowledgeFocusNoChange,
		PatternAction: domain.KnowledgePatternNoChange,
		Reason:        "missing focus id",
	}}

	validated, err := svc.validateDeltaChanges(context.Background(), "/repo", changes, focusByID)

	require.NoError(t, err)
	require.Empty(t, validated)
}

func TestAnalyzeCurrentCodebaseBatchPassesSharedContextPath(t *testing.T) {
	session := &sharedContextTestSession{}
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockAgent.AnalyzeCurrentBatchFn = session.AnalyzeCurrentCodebaseBatch
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.AnalyzeCurrentCodebaseBatch(context.Background(), "/repo", "demo", "go", AnalyzeCurrentCodebaseBatchOptions{
		RuntimeLabel:      "batch-001",
		SharedContextPath: "/repo/.skills-seed/runtime/learn-current/shared-context.md",
		RunContext:        &CodebaseRunContext{},
		Focuses: []AnalyzeCurrentEvidenceFocus{{
			EvidenceFocus: domain.EvidenceFocus{ID: "auth", Name: "Auth"},
			FocusAbsPaths: []string{"/repo/internal/auth.go"},
		}},
	})

	require.NoError(t, err)
	require.NotNil(t, session.codebaseReq)
	require.Equal(t, "/repo/.skills-seed/runtime/learn-current/shared-context.md", session.codebaseReq.SharedContextPath)
}

func TestAnalyzeCurrentDeltaBatchPassesSharedContextPath(t *testing.T) {
	session := &sharedContextTestSession{}
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockAgent.AnalyzeCurrentDeltaFn = session.AnalyzeCurrentDeltaBatch
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.AnalyzeCurrentDeltaBatch(context.Background(), "/repo", "demo", "go", AnalyzeCurrentDeltaBatchOptions{
		RuntimeLabel:      "batch-001",
		SharedContextPath: "/repo/.skills-seed/runtime/learn-current/shared-context.md",
		RunContext:        &CodebaseRunContext{},
		Focuses: []AnalyzeCurrentDeltaFocus{{
			EvidenceFocus: domain.EvidenceFocus{ID: "auth", Name: "Auth"},
			FocusAbsPaths: []string{"/repo/internal/auth.go"},
		}},
	})

	require.NoError(t, err)
	require.NotNil(t, session.deltaReq)
	require.Equal(t, "/repo/.skills-seed/runtime/learn-current/shared-context.md", session.deltaReq.SharedContextPath)
}

type sharedContextTestSession struct {
	codebaseReq *agent.AnalyzeCurrentCodebaseBatchRequest
	deltaReq    *agent.AnalyzeCurrentDeltaBatchRequest
}

func (s *sharedContextTestSession) AnalyzeCurrentCodebaseBatch(_ context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	s.codebaseReq = req
	results := make([]agent.AnalyzeCurrentEvidenceResult, 0, len(req.Focuses))
	for _, focus := range req.Focuses {
		results = append(results, agent.AnalyzeCurrentEvidenceResult{
			FocusID:   focus.EvidenceFocus.ID,
			FocusName: focus.EvidenceFocus.Name,
		})
	}
	return &agent.AnalyzeCurrentCodebaseBatchResult{Focuses: results}, nil
}

func (s *sharedContextTestSession) AnalyzeCurrentDeltaBatch(_ context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	s.deltaReq = req
	changes := make([]domain.KnowledgeChange, 0, len(req.Focuses))
	for _, focus := range req.Focuses {
		changes = append(changes, domain.KnowledgeChange{
			FocusID:       focus.EvidenceFocus.ID,
			FocusAction:   domain.KnowledgeFocusNoChange,
			PatternAction: domain.KnowledgePatternNoChange,
		})
	}
	return &agent.AnalyzeCurrentDeltaBatchResult{Changes: changes}, nil
}
