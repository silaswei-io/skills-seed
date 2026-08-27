package analyzer

import (
	"context"
	"os"
	"path/filepath"
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

	validated, err := svc.validateDeltaChanges(context.Background(), "/repo", changes, focusByID, nil)

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

	validated, err := svc.validateDeltaChanges(context.Background(), "/repo", changes, focusByID, nil)

	require.NoError(t, err)
	require.Empty(t, validated)
}

func TestValidateDeltaChangesRetiresOnlyEvidenceThatNoLongerExists(t *testing.T) {
	root := t.TempDir()
	svc := NewAnalyzerService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, nil)
	focusByID := map[string]map[string]bool{
		"auth": {"internal/auth/login.go": true},
	}
	changes := []domain.KnowledgeChange{
		{
			FocusID:       "auth",
			PatternAction: domain.KnowledgePatternRetire,
			PatternID:     "legacy-auth-flow",
			Anchors:       []domain.PatternDiffAnchor{{Path: "internal/auth/login.go"}},
		},
		{
			FocusID:       "auth",
			PatternAction: domain.KnowledgePatternRetire,
			PatternID:     "missing-anchor",
		},
		{
			FocusID:       "auth",
			PatternAction: domain.KnowledgePatternRetire,
			Anchors:       []domain.PatternDiffAnchor{{Path: "internal/auth/login.go"}},
		},
	}

	legacy := domain.NewPattern("legacy-auth-flow", "Legacy Auth Flow", domain.CategoryBusiness)
	legacy.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "internal/auth/login.go",
		Symbol: "Login",
		Kind:   "function",
	}}
	validated, err := svc.validateDeltaChanges(context.Background(), root, changes, focusByID, map[string]map[string]domain.Pattern{
		"auth": {legacy.ID: *legacy},
	})

	require.NoError(t, err)
	require.Len(t, validated, 1)
	require.Equal(t, "legacy-auth-flow", validated[0].PatternID)
}

func TestValidateDeltaChangesKeepsPatternWhenItsEvidenceStillExists(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "auth", "login.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("package auth\n\nfunc Login() error { return nil }\n"), 0o644))

	svc := NewAnalyzerService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, nil)
	change := domain.KnowledgeChange{
		FocusID:       "auth",
		PatternAction: domain.KnowledgePatternRetire,
		PatternID:     "legacy-auth-flow",
		Anchors:       []domain.PatternDiffAnchor{{Path: "internal/auth/login.go"}},
	}
	legacy := domain.NewPattern("legacy-auth-flow", "Legacy Auth Flow", domain.CategoryBusiness)
	legacy.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "internal/auth/login.go",
		Symbol: "Login",
		Kind:   "function",
	}}

	validated, err := svc.validateDeltaChanges(context.Background(), root, []domain.KnowledgeChange{change}, map[string]map[string]bool{
		"auth": {"internal/auth/login.go": true},
	}, map[string]map[string]domain.Pattern{
		"auth": {legacy.ID: *legacy},
	})

	require.NoError(t, err)
	require.Len(t, validated, 1)
	require.Equal(t, domain.KnowledgePatternNoChange, validated[0].PatternAction)
	require.Empty(t, validated[0].PatternID)
}

func TestValidateDeltaChangesKeepsFocusDecisionWhenProposalFailsAdmission(t *testing.T) {
	root := t.TempDir()
	svc := NewAnalyzerService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, nil)
	proposal := domain.NewPattern("api-contract", "API Contract", domain.CategoryAPI)
	change := domain.KnowledgeChange{
		FocusAction:   domain.KnowledgeFocusExisting,
		FocusID:       "api-contract-design",
		PatternAction: domain.KnowledgePatternAdd,
		Proposal:      proposal,
		Anchors:       []domain.PatternDiffAnchor{{Path: "internal/api/types.go"}},
		Reason:        "The diff appears to change a reusable contract.",
	}

	second := change
	second.PatternAction = domain.KnowledgePatternUpdate
	second.PatternID = "missing-api-contract"
	second.Proposal = domain.NewPattern("missing-api-contract", "Missing API Contract", domain.CategoryAPI)

	validated, err := svc.validateDeltaChanges(context.Background(), root, []domain.KnowledgeChange{change, second}, map[string]map[string]bool{
		"api-contract-design": {"internal/api/types.go": true},
	}, nil)

	require.NoError(t, err)
	require.Len(t, validated, 1)
	require.Equal(t, "api-contract-design", validated[0].FocusID)
	require.Equal(t, domain.KnowledgePatternNoChange, validated[0].PatternAction)
	require.Nil(t, validated[0].Proposal)
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
