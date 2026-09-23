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
