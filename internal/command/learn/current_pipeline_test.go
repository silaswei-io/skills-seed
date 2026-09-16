package learn

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestLearningResumesReviewWithoutRepeatingSourceAnalysis(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	mock := cont.Agent.(*mocks.MockAgent)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	stateRepo := commandstate.NewRepository(cont.SeedPath, commandStateLearnCurrent)
	sourceCalls, reviewCalls := 0, 0
	mock.AnalyzeCurrentBatchFn = func(_ context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		sourceCalls++
		return pipelineSourceResult(req), nil
	}
	mock.ReviewKnowledgeFn = func(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		reviewCalls++
		state, err := stateRepo.Load(ctx)
		if err != nil {
			return nil, err
		}
		if state.Analysis == nil || len(state.Analysis.FocusKnowledge) != 1 || state.Analysis.FocusKnowledge[0].Reviewed {
			return nil, errors.New("source must be checkpointed before review")
		}
		if len(req.Evidence.SourcePaths) == 0 {
			return nil, errors.New("review must receive saved source facts")
		}
		if reviewCalls == 1 {
			return nil, errors.New("review unavailable")
		}
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
	opts := learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip)
	_, err := runLearnCurrent(context.Background(), cont, opts)
	require.ErrorContains(t, err, "review unavailable")
	saved, err := cont.PatternRepo.GetAll(context.Background())
	require.NoError(t, err)
	require.Empty(t, saved)
	_, err = runLearnCurrent(context.Background(), cont, opts)
	require.NoError(t, err)
	require.Equal(t, 1, sourceCalls)
	require.Equal(t, 2, reviewCalls)
	require.FileExists(t, cont.SeedPath+"/cache/snapshots/main.go")
}

func TestLearningDrainsSuccessfulAnalysisAfterSiblingFailure(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, []config.WorkspaceProjectConfig{})
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, root, "second.go", "package main\nfunc second() {}\n")
	gitAddAll(t, root)
	mock := cont.Agent.(*mocks.MockAgent)
	mock.PlanLearningAgendaFn = func(context.Context, *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
		return &agent.PlanLearningAgendaResult{Focuses: []domain.EvidenceFocus{
			{ID: "first", Name: "First", EntryPaths: []string{"main.go"}},
			{ID: "second", Name: "Second", EntryPaths: []string{"second.go"}},
		}}, nil
	}
	started := make(chan struct{})
	failed := make(chan struct{})
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		if req.Focuses[0].EvidenceFocus.ID == "first" {
			close(started)
			select {
			case <-failed:
				return pipelineSourceResult(req), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		<-started
		close(failed)
		return nil, errors.New("second source failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := runLearnCurrent(ctx, cont, learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip))
	require.ErrorContains(t, err, "second source failed")
	state, err := commandstate.NewRepository(cont.SeedPath, commandStateLearnCurrent).Load(context.Background())
	require.NoError(t, err)
	require.Len(t, state.Analysis.FocusKnowledge, 1)
	require.Equal(t, "first", state.Analysis.FocusKnowledge[0].Focus.ID)
	require.NotEmpty(t, state.Analysis.FocusKnowledge[0].Patterns)
}

func TestReviewPipelineIsSerialEvenWithMultipleSlots(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{ID: "one", Name: "One", EntryPaths: []string{"one.go"}},
		{ID: "two", Name: "Two", EntryPaths: []string{"two.go"}},
	}
	units := make([]commandstate.FocusKnowledgeCheckpoint, len(focuses))
	for i, focus := range focuses {
		units[i] = commandstate.FocusKnowledgeCheckpoint{Focus: focus, Patterns: []domain.Pattern{
			*admittedLearnCurrentPatternForTest(focus.ID, focus.Name, domain.CategoryBusiness, focus.EntryPaths[0]),
		}}
	}
	var mu sync.Mutex
	active, maximum := 0, 0
	mock := &mocks.MockAgent{ReviewKnowledgeFn: func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		mu.Lock()
		active++
		maximum = max(maximum, active)
		mu.Unlock()
		defer func() { mu.Lock(); active--; mu.Unlock() }()
		return acceptKnowledgeCandidates(req.Candidates), nil
	}}
	run := newKnowledgeReviewTestRun(t, focuses, units, mock, nil)
	completed, err := run.analyzePlannedBatches("analysis", run.analysisState, nil, 4)
	require.NoError(t, err)
	require.Equal(t, 2, completed)
	require.Equal(t, 1, maximum)
}

func TestLearningOverlapsReviewWithSourceAnalysisWithinLimit(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, root, "second.go", "package main\nfunc second() {}\n")
	gitAddAll(t, root)
	mock := cont.Agent.(*mocks.MockAgent)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	mock.PlanLearningAgendaFn = func(context.Context, *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
		return &agent.PlanLearningAgendaResult{Focuses: []domain.EvidenceFocus{
			{ID: "first", Name: "First", EntryPaths: []string{"main.go"}},
			{ID: "second", Name: "Second", EntryPaths: []string{"second.go"}},
		}}, nil
	}
	reviewStarted := make(chan struct{})
	secondFinished := make(chan struct{})
	var mu sync.Mutex
	active, maximum := 0, 0
	enter := func() func() {
		mu.Lock()
		active++
		maximum = max(maximum, active)
		mu.Unlock()
		return func() { mu.Lock(); active--; mu.Unlock() }
	}
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		defer enter()()
		if req.Focuses[0].EvidenceFocus.ID == "second" {
			select {
			case <-reviewStarted:
				close(secondFinished)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return pipelineSourceResult(req), nil
	}
	mock.ReviewKnowledgeFn = func(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		defer enter()()
		if req.EvidenceFocus.ID == "first" {
			close(reviewStarted)
			select {
			case <-secondFinished:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := runLearnCurrent(ctx, cont, learnCurrentOptionsForTest("", nil, learnCurrentProfileSkip))
	require.NoError(t, err)
	require.Equal(t, 2, maximum)
}

func TestUnreviewedKnowledgeCannotBeProjectedOrStored(t *testing.T) {
	focus := domain.EvidenceFocus{ID: "pending", Name: "Pending", EntryPaths: []string{"pending.go"}}
	run := newKnowledgeReviewTestRun(t, []domain.EvidenceFocus{focus}, []commandstate.FocusKnowledgeCheckpoint{{
		Focus: focus, Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("candidate", "Candidate", domain.CategoryBusiness, "pending.go")},
		RetiredPatternIDs: []string{"old"},
	}}, &mocks.MockAgent{}, nil)
	require.Empty(t, run.patterns)
	require.Empty(t, run.retiredPatternIDs)
	require.ErrorContains(t, run.completeAnalysis(), "unreviewed")
	require.ErrorContains(t, run.normalizeAndSavePatternsStep(), "before all focuses are reviewed")
}

func TestDeltaAnchorResolvesOwnerInsteadOfRelatedEvidence(t *testing.T) {
	root := t.TempDir()
	owner := domain.EvidenceFocus{ID: "owner", Name: "Owner", EntryPaths: []string{"owned.go"}}
	reader := domain.EvidenceFocus{ID: "reader", Name: "Reader", EntryPaths: []string{"reader.go"}, RelatedPaths: []string{"owned.go"}}
	resolver := newDeltaFocusResolver(root, []analyzer.AnalyzeCurrentEvidenceFocus{{EvidenceFocus: owner}, {EvidenceFocus: reader}})
	focus, ok := resolver.resolve(domain.KnowledgeChange{Anchors: []domain.PatternDiffAnchor{{Path: "owned.go"}}})
	require.True(t, ok)
	require.Equal(t, owner.ID, focus.ID)
}

func pipelineSourceResult(req *agent.AnalyzeCurrentCodebaseBatchRequest) *agent.AnalyzeCurrentCodebaseBatchResult {
	focus := req.Focuses[0]
	return &agent.AnalyzeCurrentCodebaseBatchResult{Focuses: []agent.AnalyzeCurrentEvidenceResult{{
		FocusID: focus.EvidenceFocus.ID, FocusName: focus.EvidenceFocus.Name,
		Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("pattern-"+focus.EvidenceFocus.ID, "Source boundary", domain.CategoryBusiness, focus.FocusPaths[0])},
	}}}
}
