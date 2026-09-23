package learncurrent

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestFocusNeedsIndependentReview(t *testing.T) {
	require.False(t, focusNeedsIndependentReview(nil))
	require.False(t, focusNeedsIndependentReview([]domain.Pattern{}))
	require.True(t, focusNeedsIndependentReview([]domain.Pattern{{ID: "p1"}}))
}

func TestKnowledgeNeedsNormalizeStore(t *testing.T) {
	require.False(t, knowledgeNeedsNormalizeStore(nil, nil))
	require.False(t, knowledgeNeedsNormalizeStore([]domain.Pattern{}, nil))
	require.True(t, knowledgeNeedsNormalizeStore([]domain.Pattern{{ID: "p1"}}, nil))
	require.True(t, knowledgeNeedsNormalizeStore(nil, []string{"old"}))
}

func TestRouteFocusReview(t *testing.T) {
	standardFocus := domain.EvidenceFocus{ID: "f1", AnalysisDepth: domain.EvidenceFocusDepthStandard}
	carefulFocus := domain.EvidenceFocus{ID: "f2", AnalysisDepth: domain.EvidenceFocusDepthCareful}
	okPattern := domain.Pattern{
		ID: "p1", Name: "Name", Rule: "when x do y", Category: domain.CategoryAPI,
		Confidence:        0.9,
		EvidenceLocations: []domain.PatternEvidenceLocation{{Path: "internal/api/handler.go", Line: 10}},
	}
	riskPattern := okPattern
	riskPattern.ID = "p2"
	riskPattern.KnowledgeFlags = []string{domain.KnowledgeFlagOperationalRisk}
	weakPattern := okPattern
	weakPattern.ID = "p3"
	weakPattern.EvidenceLocations = nil

	require.Equal(t, reviewRouteNone, routeFocusReview(standardFocus, nil))
	require.Equal(t, reviewRouteLocalStandard, routeFocusReview(standardFocus, []domain.Pattern{okPattern}))
	require.Equal(t, reviewRouteAI, routeFocusReview(carefulFocus, []domain.Pattern{okPattern}))
	require.Equal(t, reviewRouteAI, routeFocusReview(standardFocus, []domain.Pattern{riskPattern}))
	require.Equal(t, reviewRouteAI, routeFocusReview(standardFocus, []domain.Pattern{weakPattern}))
}
