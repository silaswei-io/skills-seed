package learncurrent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestAnalyzePlannedBatchesUsesConfiguredParallelismAndCheckpoints(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	config := cont.ConfigRepo.Get()
	config.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(config))
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := config.Project.RootPath
	paths := []string{"alpha.go", "beta.go"}
	focuses := make([]domain.EvidenceFocus, 0, len(paths))
	batches := make([]learnCurrentBatch, 0, len(paths))
	for i, path := range paths {
		require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte("package sample\nfunc Decide() {}\n"), 0600))
		focus := domain.EvidenceFocus{ID: path, Name: path, EntryPaths: []string{path}}
		focuses = append(focuses, focus)
		batches = append(batches, learnCurrentBatch{index: i, focuses: []indexedEvidenceFocus{{index: i, focus: focus}}})
	}
	run := newLearnCurrentProjectRun(context.Background(), cont, learnCurrentProjectOptions{})
	run.projectRoot = root
	run.projectName = config.Project.Name
	run.currentLanguage = "go"
	run.incrementalChanges = &fileanalysis.FileChanges{AddedOrModified: paths, Records: []domain.FileAnalysisRecord{{Path: paths[0]}, {Path: paths[1]}}}
	run.codebaseRunContext = &analyzer.CodebaseRunContext{}
	state := commandstate.NewState(commandStateLearnCurrent, run.projectName, "go", "", nil, nil, focuses)
	run.analysisState = state
	started := make(chan string, len(paths))
	release := make(chan struct{})
	reviewStarted := make(chan struct{})
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		focusID := req.Focuses[0].EvidenceFocus.ID
		started <- focusID
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if focusID == paths[1] {
			select {
			case <-reviewStarted:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return pipelineSourceResult(req), nil
	}
	mock.ReviewKnowledgeFn = func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		if req.EvidenceFocus.ID == paths[0] {
			close(reviewStarted)
		}
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run.ctx = agent.WithCallBudget(ctx, 2)
	type outcome struct {
		completed int
		err       error
	}
	done := make(chan outcome, 1)
	go func() {
		completed, err := run.analyzePlannedBatches("analysis", state, batches, 2)
		done <- outcome{completed: completed, err: err}
	}()
	var observed []string
	for len(observed) < len(paths) {
		select {
		case id := <-started:
			observed = append(observed, id)
		case <-ctx.Done():
			close(release)
			t.Fatal("independent analysis batches did not start concurrently")
		}
	}
	close(release)
	result := <-done
	require.NoError(t, result.err)
	require.Equal(t, len(paths), result.completed)
	require.ElementsMatch(t, paths, observed)
	checkpoint, err := run.stateRepo.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, checkpoint.Analysis.FocusKnowledge, len(paths))
	for _, unit := range checkpoint.Analysis.FocusKnowledge {
		require.True(t, unit.Reviewed)
	}
}

func TestReviewRemainingKnowledgeUsesConfiguredParallelismAndPreservesCheckpoint(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	config := cont.ConfigRepo.Get()
	config.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(config))
	focuses := []domain.EvidenceFocus{
		{ID: "first", Name: "First", EntryPaths: []string{"first.go"}},
		{ID: "second", Name: "Second", EntryPaths: []string{"second.go"}},
	}
	units := make([]commandstate.FocusKnowledgeCheckpoint, 0, len(focuses))
	for _, focus := range focuses {
		units = append(units, commandstate.FocusKnowledgeCheckpoint{
			Focus:    focus,
			Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest(focus.ID+"-rule", focus.Name, domain.CategoryBusiness, focus.EntryPaths[0])},
		})
	}
	started := make(chan string, len(focuses))
	release := make(chan struct{})
	mock := &mocks.MockAgent{ReviewKnowledgeFn: func(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		started <- req.EvidenceFocus.ID
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if req.EvidenceFocus.ID == "first" {
			return nil, context.DeadlineExceeded
		}
		return acceptKnowledgeCandidates(req.Candidates), nil
	}}
	run := newKnowledgeReviewTestRun(t, focuses, units, mock, nil)
	run.cont.ConfigRepo = cont.ConfigRepo
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run.ctx = agent.WithCallBudget(ctx, 2)
	done := make(chan error, 1)
	go func() {
		done <- run.reviewRemainingKnowledge("analysis")
	}()
	for range focuses {
		select {
		case <-started:
		case <-ctx.Done():
			close(release)
			t.Fatal("configured parallelism did not start two reviews")
		}
	}
	close(release)
	require.ErrorIs(t, <-done, context.DeadlineExceeded)
	checkpoint, err := run.stateRepo.Load(context.Background())
	require.NoError(t, err)
	require.False(t, checkpoint.Analysis.FocusKnowledge[0].Reviewed)
	require.True(t, checkpoint.Analysis.FocusKnowledge[1].Reviewed)
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
