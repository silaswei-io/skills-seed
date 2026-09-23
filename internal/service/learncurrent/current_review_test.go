package learncurrent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestReviewLearnedKnowledgeUsesSerialFocusBoundaries(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	focuses := []domain.EvidenceFocus{
		{ID: "auth", Name: "认证", EntryPaths: []string{"internal/auth.go"}},
		{ID: "key", Name: "密钥", EntryPaths: []string{"internal/key.go"}},
	}
	units := []commandstate.FocusKnowledgeCheckpoint{
		{Focus: focuses[0], Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("auth-rule", "Auth", domain.CategoryBusiness, "internal/auth.go")}},
		{Focus: focuses[1], Patterns: []domain.Pattern{
			*admittedLearnCurrentPatternForTest("key-create", "Key Create", domain.CategoryBusiness, "internal/key.go"),
			*admittedLearnCurrentPatternForTest("key-rotate", "Key Rotate", domain.CategoryBusiness, "internal/key.go"),
		}},
	}

	var focusCalls []string
	var candidateCalls [][]string
	mockAgent := &mocks.MockAgent{ReviewKnowledgeFn: func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		focusCalls = append(focusCalls, req.EvidenceFocus.ID)
		ids := make([]string, 0, len(req.Candidates))
		for _, candidate := range req.Candidates {
			ids = append(ids, candidate.ID)
		}
		candidateCalls = append(candidateCalls, ids)
		return acceptKnowledgeCandidates(req.Candidates), nil
	}}
	var progressLabels []string
	run := newKnowledgeReviewTestRun(t, focuses, units, mockAgent, func(label string) {
		progressLabels = append(progressLabels, label)
	})

	require.NoError(t, run.reviewRemainingKnowledge("analysis"))

	require.Equal(t, []string{"auth", "key"}, focusCalls)
	require.Equal(t, [][]string{{"auth-rule"}, {"key-create", "key-rotate"}}, candidateCalls)
	require.Contains(t, strings.Join(progressLabels, "\n"), "焦点 1/2 认证 · 候选 1")
	require.Contains(t, strings.Join(progressLabels, "\n"), "焦点 2/2 密钥 · 候选 2")
	state, err := run.stateRepo.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, state.Analysis.FocusKnowledge, 2)
	require.True(t, state.Analysis.FocusKnowledge[0].Reviewed)
	require.True(t, state.Analysis.FocusKnowledge[1].Reviewed)
}

func TestReviewLearnedKnowledgeResumesOnlyUnreviewedFocus(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	focuses := []domain.EvidenceFocus{
		{ID: "first", Name: "第一焦点", EntryPaths: []string{"first.ext"}},
		{ID: "second", Name: "第二焦点", EntryPaths: []string{"second.ext"}},
	}
	units := []commandstate.FocusKnowledgeCheckpoint{
		{Focus: focuses[0], Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("first-pattern", "First", domain.CategoryBusiness, "first.ext")}},
		{Focus: focuses[1], Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("second-pattern", "Second", domain.CategoryBusiness, "second.ext")}},
	}

	var firstRunCalls []string
	mockAgent := &mocks.MockAgent{ReviewKnowledgeFn: func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		firstRunCalls = append(firstRunCalls, req.EvidenceFocus.ID)
		if req.EvidenceFocus.ID == "second" {
			return nil, errors.New("review failed")
		}
		return acceptKnowledgeCandidates(req.Candidates), nil
	}}
	run := newKnowledgeReviewTestRun(t, focuses, units, mockAgent, nil)

	err := run.reviewRemainingKnowledge("analysis")
	require.ErrorContains(t, err, "review failed")
	require.Equal(t, []string{"first", "second"}, firstRunCalls)
	checkpoint, loadErr := run.stateRepo.Load(context.Background())
	require.NoError(t, loadErr)
	require.True(t, checkpoint.Analysis.FocusKnowledge[0].Reviewed)
	require.False(t, checkpoint.Analysis.FocusKnowledge[1].Reviewed)

	var resumedCalls []string
	mockAgent.ReviewKnowledgeFn = func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		resumedCalls = append(resumedCalls, req.EvidenceFocus.ID)
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
	run.analysisState = checkpoint
	run.focusKnowledge = nil
	run.patterns = nil
	run.restoreAnalysisCheckpoint()

	require.NoError(t, run.reviewRemainingKnowledge("analysis"))
	require.Equal(t, []string{"second"}, resumedCalls)
}

func TestReviewLearnedKnowledgeCheckpointsEmptyFocusWithoutAgentCall(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	focus := domain.EvidenceFocus{ID: "empty", Name: "无候选", EntryPaths: []string{"empty.ext"}}
	calls := 0
	mockAgent := &mocks.MockAgent{ReviewKnowledgeFn: func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		calls++
		return acceptKnowledgeCandidates(req.Candidates), nil
	}}
	run := newKnowledgeReviewTestRun(t, []domain.EvidenceFocus{focus}, []commandstate.FocusKnowledgeCheckpoint{{Focus: focus}}, mockAgent, nil)

	require.NoError(t, run.reviewRemainingKnowledge("analysis"))
	require.Zero(t, calls)
	state, err := run.stateRepo.Load(context.Background())
	require.NoError(t, err)
	require.True(t, state.Analysis.FocusKnowledge[0].Reviewed)
}

func TestReviewAnalyzedFocusUsesCheckpointEvidence(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	focus := domain.EvidenceFocus{ID: "auth", Name: "认证", EntryPaths: []string{"internal/auth.go"}}
	evidence := domain.LearningEvidence{SourcePaths: []string{"internal/auth.go"}, StructuralContext: "Auth calls Verify"}
	var received domain.LearningEvidence
	var receivedRuntimeLabel string
	mockAgent := &mocks.MockAgent{ReviewKnowledgeFn: func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		received = req.Evidence
		receivedRuntimeLabel = req.RuntimeLabel
		return acceptKnowledgeCandidates(req.Candidates), nil
	}}
	run := newKnowledgeReviewTestRun(t, []domain.EvidenceFocus{focus}, nil, mockAgent, nil)
	result := buildAnalyzedFocusResult(focus, 0, []domain.Pattern{*admittedLearnCurrentPatternForTest("auth-rule", "Auth", domain.CategoryBusiness, "internal/auth.go")}, agent.ProfileRefreshRecommendation{}, agent.Conversation{})
	result.evidence = evidence
	_, err := run.checkpointFocusResult(result)
	require.NoError(t, err)

	err = run.reviewRemainingKnowledge("analysis")

	require.NoError(t, err)
	require.Equal(t, evidence, received)
	require.Equal(t, "batch-001", receivedRuntimeLabel)
	require.True(t, run.focusKnowledge[0].Reviewed)
	require.Equal(t, domain.DevelopmentFocusFromEvidenceFocus(focus), run.patterns[0].DevelopmentFocus)
}

func TestDerivedKnowledgeFollowsAgendaOrder(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	focuses := []domain.EvidenceFocus{
		{ID: "first", Name: "First", EntryPaths: []string{"first.ext"}},
		{ID: "second", Name: "Second", EntryPaths: []string{"second.ext"}},
	}
	units := []commandstate.FocusKnowledgeCheckpoint{
		{Focus: focuses[1], Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("second-pattern", "Second", domain.CategoryBusiness, "second.ext")}, Reviewed: true},
		{Focus: focuses[0], Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("first-pattern", "First", domain.CategoryBusiness, "first.ext")}, Reviewed: true},
	}
	run := newKnowledgeReviewTestRun(t, focuses, units, &mocks.MockAgent{}, nil)

	require.Equal(t, []string{"first", "second"}, []string{run.focusKnowledge[0].Focus.ID, run.focusKnowledge[1].Focus.ID})
	require.Equal(t, []string{"first-pattern", "second-pattern"}, []string{run.patterns[0].ID, run.patterns[1].ID})
}

func newKnowledgeReviewTestRun(t *testing.T, focuses []domain.EvidenceFocus, units []commandstate.FocusKnowledgeCheckpoint, reviewer *mocks.MockAgent, onUpdate func(string)) *learnCurrentProjectRun {
	t.Helper()
	stateRepo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "mixed", "", nil, nil, focuses)
	state.Analysis = &commandstate.AnalysisCheckpoint{FocusKnowledge: cloneFocusKnowledge(units)}
	require.NoError(t, stateRepo.Save(context.Background(), state))
	service := patternnorm.NewServiceWithNormalizer(&mocks.MockPatternRepository{}, reviewer)
	run := &learnCurrentProjectRun{
		ctx:             context.Background(),
		cont:            &container.Container{PatternNormSvc: service},
		stateRepo:       stateRepo,
		analysisState:   state,
		projectName:     "demo",
		projectRoot:     t.TempDir(),
		currentLanguage: "mixed",
		focusKnowledge:  cloneFocusKnowledge(units),
		steps: commandutil.NewConsoleStepRunner(commandutil.ConsoleStepRunnerOptions{
			TotalSteps:   1,
			OnStepUpdate: onUpdate,
		}),
	}
	run.syncDerivedKnowledge()
	return run
}

func acceptKnowledgeCandidates(candidates []domain.Pattern) *agent.ReviewKnowledgeResult {
	decisions := make([]agent.KnowledgeReviewDecision, 0, len(candidates))
	for _, candidate := range candidates {
		decisions = append(decisions, agent.KnowledgeReviewDecision{
			CandidateID: candidate.ID, Verdict: "accept", ReasonCode: "accepted",
			Reason: "The evidence supports the candidate.",
		})
	}
	return &agent.ReviewKnowledgeResult{Decisions: decisions}
}
