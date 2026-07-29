package analyzer

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
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
