package analyzer

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestReconcileLearningAgendaNormalizesAndDeduplicatesPathDecisions(t *testing.T) {
	agenda, err := reconcileLearningAgenda(
		[]string{"src/entry.ext", "src/related.ext", "src/support.ext"},
		[]domain.EvidenceFocus{{
			ID:           " primary ",
			Name:         " Primary evidence ",
			EntryPaths:   []string{"./src/entry.ext", "src/entry.ext", "outside.ext"},
			RelatedPaths: []string{"src/related.ext", "src/entry.ext"},
		}},
		[]agent.LearningPathSkip{
			{Path: "src/support.ext", Reason: " First receipt. "},
			{Path: "./src/support.ext", Reason: "Duplicate receipt."},
			{Path: "context-only.ext", Reason: "Not an input file."},
		},
	)

	require.NoError(t, err)
	require.Equal(t, []domain.EvidenceFocus{{
		ID:           "primary",
		Name:         "Primary evidence",
		EntryPaths:   []string{"src/entry.ext"},
		RelatedPaths: []string{"src/related.ext"},
	}}, agenda.Focuses)
	require.Equal(t, []agent.LearningPathSkip{{Path: "src/support.ext", Reason: "First receipt."}}, agenda.Skipped)
}

func TestReconcileLearningAgendaPrefersFirstFocusedDecision(t *testing.T) {
	agenda, err := reconcileLearningAgenda(
		[]string{"src/shared.ext", "src/second.ext", "src/skipped.ext"},
		[]domain.EvidenceFocus{
			{ID: "first", Name: "First", EntryPaths: []string{"src/shared.ext"}},
			{ID: "second", Name: "Second", EntryPaths: []string{"src/shared.ext", "src/second.ext"}},
		},
		[]agent.LearningPathSkip{
			{Path: "src/shared.ext", Reason: "Conflicts with focus."},
			{Path: "src/skipped.ext", Reason: "No durable decision value."},
		},
	)

	require.NoError(t, err)
	require.Equal(t, []domain.EvidenceFocus{
		{ID: "first", Name: "First", EntryPaths: []string{"src/shared.ext"}},
		{ID: "second", Name: "Second", EntryPaths: []string{"src/second.ext"}},
	}, agenda.Focuses)
	require.Equal(t, []agent.LearningPathSkip{{Path: "src/skipped.ext", Reason: "No durable decision value."}}, agenda.Skipped)
}

func TestReconcileLearningAgendaAddsOneFallbackFocusForOmittedInputs(t *testing.T) {
	require.NoError(t, i18n.Init(i18n.LocaleEnglish))
	t.Cleanup(func() { require.NoError(t, i18n.Init(i18n.DefaultLocale)) })

	agenda, err := reconcileLearningAgenda(
		[]string{"src/entry.ext", "src/omitted-a.ext", "src/omitted-b.ext", "src/skipped.ext"},
		[]domain.EvidenceFocus{{ID: "existing", Name: "Existing", EntryPaths: []string{"src/entry.ext"}}},
		[]agent.LearningPathSkip{{Path: "src/skipped.ext", Reason: "No durable decision value."}},
	)

	require.NoError(t, err)
	require.Len(t, agenda.Focuses, 2)
	require.Equal(t, "unassigned-evidence", agenda.Focuses[1].ID)
	require.Equal(t, i18n.Get("LearnCurrentUnassignedEvidenceFocusName"), agenda.Focuses[1].Name)
	require.Equal(t, i18n.Get("LearnCurrentUnassignedEvidenceFocusReason"), agenda.Focuses[1].ScopeReason)
	require.Equal(t, []string{"src/omitted-a.ext", "src/omitted-b.ext"}, agenda.Focuses[1].EntryPaths)
	require.Equal(t, domain.EvidenceFocusPurposeCoverage, agenda.Focuses[1].Purpose)
	require.Empty(t, agenda.Focuses[1].RouteTerms)
	require.Empty(t, agenda.Focuses[1].Attributes)
	require.Empty(t, agenda.Focuses[1].RiskSignals)
}

func TestReconcileLearningAgendaRejectsInvalidDurableDecisions(t *testing.T) {
	tests := []struct {
		name    string
		focuses []domain.EvidenceFocus
		skipped []agent.LearningPathSkip
		want    string
	}{
		{
			name:    "missing focus id",
			focuses: []domain.EvidenceFocus{{Name: "Named", EntryPaths: []string{"src/entry.ext"}}},
			want:    "has no id",
		},
		{
			name:    "missing focus name",
			focuses: []domain.EvidenceFocus{{ID: "entry", EntryPaths: []string{"src/entry.ext"}}},
			want:    "has no name",
		},
		{
			name: "duplicate focus id",
			focuses: []domain.EvidenceFocus{
				{ID: "entry", Name: "First", EntryPaths: []string{"src/entry.ext"}},
				{ID: "entry", Name: "Second", EntryPaths: []string{"src/second.ext"}},
			},
			want: "repeats focus id",
		},
		{
			name:    "missing skip reason",
			skipped: []agent.LearningPathSkip{{Path: "src/entry.ext"}},
			want:    "has no reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := reconcileLearningAgenda([]string{"src/entry.ext", "src/second.ext"}, tt.focuses, tt.skipped)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestPlanLearningAgendaReconcilesRepeatedSkipReceipts(t *testing.T) {
	svc := NewAnalyzerService(&mocks.MockAgent{
		PlanLearningAgendaFn: func(context.Context, *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
			return &agent.PlanLearningAgendaResult{
				Focuses: []domain.EvidenceFocus{{
					ID:         "entry",
					Name:       "Entry",
					EntryPaths: []string{"src/entry.ext"},
				}},
				SkippedPaths: []agent.LearningPathSkip{
					{Path: "./src/support.ext", Reason: "First receipt."},
					{Path: "src/support.ext", Reason: "Duplicate receipt."},
				},
				Reason: "Source evidence is grouped by responsibility.",
			}, nil
		},
	}, nil)

	plan, err := svc.PlanLearningAgenda(context.Background(), &PlanLearningAgendaRequest{
		FocusPaths:        []string{"src/entry.ext", "src/support.ext"},
		StructuralContext: "provided by the caller",
	})

	require.NoError(t, err)
	require.Equal(t, []agent.LearningPathSkip{{Path: "src/support.ext", Reason: "First receipt."}}, plan.SkippedPaths)
}
