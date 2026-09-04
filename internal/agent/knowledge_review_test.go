package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidKnowledgeReviewVerdict(t *testing.T) {
	for _, verdict := range []string{"accept", "revise", "reject", " accept "} {
		require.True(t, ValidKnowledgeReviewVerdict(verdict), verdict)
	}
	for _, verdict := range []string{"", "approve", "ACCEPT"} {
		require.False(t, ValidKnowledgeReviewVerdict(verdict), verdict)
	}
}

func TestValidKnowledgeReviewReasonCode(t *testing.T) {
	for _, code := range []string{
		"accepted", "unsupported_evidence", "contradictory", "unsafe_guidance",
		"no_routeable_value", "low_signal_boilerplate", "overclaimed", "incorrect_boundary", " accepted ",
	} {
		require.True(t, ValidKnowledgeReviewReasonCode(code), code)
	}
	for _, code := range []string{"", "unknown", "ACCEPTED"} {
		require.False(t, ValidKnowledgeReviewReasonCode(code), code)
	}
}

func TestValidateKnowledgeReviewCandidates(t *testing.T) {
	valid := []KnowledgeReviewDecision{{CandidateID: "a"}, {CandidateID: "b"}}
	require.NoError(t, ValidateKnowledgeReviewCandidates(valid, []string{"a", "b"}))
	require.NoError(t, ValidateKnowledgeReviewCandidates(nil, []string{}))

	tests := []struct {
		name      string
		decisions []KnowledgeReviewDecision
		ids       []string
		want      string
	}{
		{name: "empty id", decisions: []KnowledgeReviewDecision{{}}, ids: []string{"a"}, want: "empty candidate id"},
		{name: "duplicate", decisions: []KnowledgeReviewDecision{{CandidateID: "a"}, {CandidateID: "a"}}, ids: []string{"a"}, want: "repeats"},
		{name: "unknown", decisions: []KnowledgeReviewDecision{{CandidateID: "x"}}, ids: []string{"a"}, want: "unknown"},
		{name: "missing", decisions: []KnowledgeReviewDecision{{CandidateID: "a"}}, ids: []string{"a", "b"}, want: "no decision"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorContains(t, ValidateKnowledgeReviewCandidates(test.decisions, test.ids), test.want)
		})
	}
}
