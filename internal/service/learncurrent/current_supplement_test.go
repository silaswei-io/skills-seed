package learncurrent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestSemanticGapSelectionUsesImportanceAndChanges(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "shared", Path: "shared.go", Kind: "function"},
		{ID: "a", Path: "a.go", Kind: "function"},
		{ID: "b", Path: "b.go", Kind: "function"},
	}, Calls: []codegraph.Call{{Source: "shared", Target: "a"}, {Source: "shared", Target: "b"}}}
	known := *admittedLearnCurrentPatternForTest("known", "Known", domain.CategoryBusiness, "known.go")
	gaps := selectSemanticGaps(index, map[string]bool{"shared.go": true, "known.go": true, "ordinary.go": true}, []domain.Pattern{known}, true)
	require.Len(t, gaps, 2)
	require.Equal(t, "known.go", gaps[0].Path)
	require.Equal(t, "shared.go", gaps[1].Path)
	require.Empty(t, selectSemanticGaps(index, map[string]bool{}, []domain.Pattern{known}, true))
}

func TestSupplementRejectsOversizeAndEscapingSource(t *testing.T) {
	root := t.TempDir()
	index := &codegraph.Index{Files: []codegraph.File{{Path: "main.go"}}}
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte(strings.Repeat("x", supplementSingleSourceLimit+1)), 0600))
	_, err := supplementEvidence(root, index, []semanticGap{{Path: "main.go"}}, nil)
	require.ErrorIs(t, err, agent.ErrSupplementUnresolved)
	outside := filepath.Join(t.TempDir(), "private.go")
	require.NoError(t, os.WriteFile(outside, []byte("package private"), 0600))
	require.NoError(t, os.Remove(filepath.Join(root, "main.go")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "main.go")))
	_, err = supplementEvidence(root, index, []semanticGap{{Path: "main.go"}}, nil)
	require.ErrorContains(t, err, "leaves project")
}

func TestSemanticGapSelectionIgnoresImportedUtilities(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "flow", Path: "flow.go", Kind: "function"}, {ID: "utility", Path: "utility.go", Kind: "function"},
		{ID: "policy", Path: "policy.go", Kind: "function"}, {ID: "store", Path: "store.go", Kind: "function"},
	}}
	for i := 0; i < 80; i++ {
		index.Dependencies = append(index.Dependencies, codegraph.Dependency{Source: fmt.Sprintf("caller%d.go", i), Target: "utility.go"})
	}
	index.Calls = []codegraph.Call{{Source: "flow", Target: "policy"}, {Source: "flow", Target: "store"}}
	gaps := selectSemanticGaps(index, map[string]bool{"flow.go": true, "utility.go": true}, nil, true)
	require.Len(t, gaps, 1)
	require.Equal(t, "flow.go", gaps[0].Path)
}

func TestSemanticGapSelectionFollowsWrapperAndSkipsDeclaration(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "entry", Path: "entry.go", Kind: "function"},
		{ID: "wrapper", Path: "entry.go", Kind: "function"},
		{ID: "policy", Path: "policy.go", Kind: "function"},
		{ID: "store", Path: "store.go", Kind: "function"},
		{ID: "contract", Path: "contract.go", Kind: "method", DeclarationOnly: true},
	}, Calls: []codegraph.Call{
		{Source: "entry", Target: "wrapper"}, {Source: "wrapper", Target: "policy"},
		{Source: "wrapper", Target: "store"}, {Source: "contract", Target: "policy"},
	}}
	gaps := selectSemanticGaps(index, map[string]bool{"entry.go": true, "contract.go": true}, nil, true)
	require.Len(t, gaps, 1)
	require.Equal(t, "entry.go", gaps[0].Path)
	require.ElementsMatch(t, []string{"policy", "store", "wrapper"}, gaps[0].Related)
}

func TestSemanticGapSelectionKeepsSingleCrossFileBoundary(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "entry", Path: "entry.go", Kind: "function"},
		{ID: "wrapper", Path: "entry.go", Kind: "function"},
		{ID: "policy", Path: "policy.go", Kind: "function"},
	}, Calls: []codegraph.Call{{Source: "entry", Target: "wrapper"}, {Source: "wrapper", Target: "policy"}}}
	gaps := selectSemanticGaps(index, map[string]bool{"entry.go": true}, nil, true)
	require.Len(t, gaps, 1)
	require.ElementsMatch(t, []string{"wrapper", "policy"}, gaps[0].Related)
}

func TestSemanticGapSelectionRechecksChangedCalleeAfterInitialScan(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "entry", Path: "entry.go", Kind: "function"},
		{ID: "decision", Path: "decision.go", Kind: "function"},
	}, Calls: []codegraph.Call{{Source: "entry", Target: "decision"}}}
	changed := map[string]bool{"decision.go": true}
	require.Empty(t, selectSemanticGaps(index, changed, nil, true))
	gaps := selectSemanticGaps(index, changed, nil, false)
	require.Len(t, gaps, 1)
	require.Equal(t, "decision.go", gaps[0].Path)
}

func TestSemanticGapSelectionKeepsSameFileHelpersAsEntryEvidence(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "entry", Path: "service.go", Kind: "function", Name: "Process", Exported: true, Line: 2, EndLine: 2},
		{ID: "decision", Path: "service.go", Kind: "function", Name: "decide", Line: 3, EndLine: 3},
	}, Calls: []codegraph.Call{{Source: "entry", Target: "decision"}}}
	gaps := selectSemanticGaps(index, map[string]bool{"service.go": true}, nil, false)
	require.Len(t, gaps, 1)
	require.Equal(t, []string{"entry"}, gaps[0].Symbols)
	require.Equal(t, []string{"decision"}, gaps[0].Related)

	root := t.TempDir()
	writeLearnFile(t, root, "service.go", "package service\nfunc Process() { decide() }\nfunc decide() {}\n")
	index.Files = []codegraph.File{{Path: "service.go"}}
	evidence, err := supplementEvidence(root, index, gaps, nil)
	require.NoError(t, err)
	require.Contains(t, evidence.Evidence, "func decide()")
}

func TestSemanticGapSelectionIncludesPublicBehaviorBesideConstructor(t *testing.T) {
	root := t.TempDir()
	path := "logic/issue.go"
	writeLearnFile(t, root, path, "package logic\ntype Issue struct{}\nfunc NewIssue() *Issue { return &Issue{} }\nfunc (i *Issue) Issue() error { return nil }\n")
	index := &codegraph.Index{Files: []codegraph.File{{Path: path}}, Nodes: []codegraph.Node{
		{ID: "constructor", Path: path, Kind: "function", Name: "NewIssue", Exported: true, Line: 3, EndLine: 3},
		{ID: "behavior", Path: path, Kind: "method", Name: "Issue", Exported: true, Line: 4, EndLine: 4},
		{ID: "caller", Path: "handler.go", Kind: "function", Name: "Handle", Line: 2, EndLine: 2},
	}, Calls: []codegraph.Call{{Source: "caller", Target: "behavior"}}}
	gaps := selectSemanticGaps(index, map[string]bool{path: true}, nil, true)
	require.Len(t, gaps, 1)
	require.ElementsMatch(t, []string{"constructor", "behavior"}, gaps[0].Symbols)
	evidence, err := supplementEvidence(root, index, gaps, nil)
	require.NoError(t, err)
	require.Contains(t, evidence.Evidence, "func (i *Issue) Issue() error")
}

func TestSemanticGapSelectionSkipsLocalOnlyPrivateCallChain(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "entry", Path: "helper.go", Kind: "function"},
		{ID: "decision", Path: "helper.go", Kind: "function"},
	}, Calls: []codegraph.Call{{Source: "entry", Target: "decision"}}}
	require.Empty(t, selectSemanticGaps(index, map[string]bool{"helper.go": true}, nil, false))
}

func TestSemanticScreenRejectsUnknownPathAndProtectsExistingKnowledge(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{{ID: "entry", Path: "entry.go", Kind: "function", Name: "Entry", Exported: true}}}
	called := false
	screener := &mocks.MockAgent{ScreenSemanticGapsFn: func(_ context.Context, req *agent.SemanticScreenRequest) (*agent.SemanticScreenResult, error) {
		called = true
		require.Equal(t, "entry.go", req.Candidates[0].Path)
		return &agent.SemanticScreenResult{Skip: []agent.SemanticScreenDecision{{Path: "invented.go", Reason: "unknown"}}}, nil
	}}
	_, err := screenCandidates(context.Background(), screener, index, []semanticGap{{Path: "entry.go", Symbols: []string{"entry"}}}, 1)
	require.ErrorContains(t, err, "invalid path")
	require.True(t, called)
	called = false
	skipped, err := screenCandidates(context.Background(), screener, index, []semanticGap{{Path: "entry.go", Symbols: []string{"entry"}, Existing: true}}, 1)
	require.NoError(t, err)
	require.Empty(t, skipped)
	require.False(t, called)
}

func TestSemanticScreenUsesConfiguredParallelism(t *testing.T) {
	index := &codegraph.Index{}
	var gaps []semanticGap
	for i := range 4 {
		path := fmt.Sprintf("candidate-%d.go", i)
		id := fmt.Sprintf("symbol-%d", i)
		index.Nodes = append(index.Nodes, codegraph.Node{ID: id, Path: path, Kind: "function", Signature: strings.Repeat("S", 24*1024)})
		gaps = append(gaps, semanticGap{Path: path, Symbols: []string{id}})
	}
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	screener := &mocks.MockAgent{ScreenSemanticGapsFn: func(_ context.Context, req *agent.SemanticScreenRequest) (*agent.SemanticScreenResult, error) {
		started <- struct{}{}
		<-release
		return &agent.SemanticScreenResult{Skip: []agent.SemanticScreenDecision{}}, nil
	}}
	done := make(chan error, 1)
	go func() {
		_, err := screenCandidates(context.Background(), screener, index, gaps, 2)
		done <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("screen batches did not start concurrently")
		}
	}
	close(release)
	require.NoError(t, <-done)
}

func TestSemanticGapSelectionIncludesPublicCapabilityWithoutKnownCaller(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "encrypt", Path: "crypto/sm9.go", Name: "EncryptSM9", Kind: "function", Exported: true, Line: 8, EndLine: 20},
		{ID: "helper", Path: "crypto/helper.go", Name: "normalize", Kind: "function", Line: 8, EndLine: 20},
	}, Calls: []codegraph.Call{{Source: "helper", Target: "encrypt"}}}
	gaps := selectSemanticGaps(index, map[string]bool{"crypto/sm9.go": true, "crypto/helper.go": true}, nil, false)
	require.Len(t, gaps, 2)
	require.ElementsMatch(t, []string{"crypto/sm9.go", "crypto/helper.go"}, []string{gaps[0].Path, gaps[1].Path})
	initial := selectSemanticGaps(&codegraph.Index{Nodes: index.Nodes}, map[string]bool{"crypto/sm9.go": true}, nil, true)
	require.Len(t, initial, 1)
	require.Equal(t, "crypto/sm9.go", initial[0].Path)
}

func TestSemanticGapSelectionIncludesUsedPublicCapabilityOnInitialScan(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "encrypt", Path: "crypto/sm9.go", Name: "EncryptSM9", Kind: "function", Exported: true, Line: 8, EndLine: 20},
		{ID: "issue", Path: "service/issue.go", Name: "Issue", Kind: "method", Line: 8, EndLine: 20},
	}, Calls: []codegraph.Call{{Source: "issue", Target: "encrypt"}}}
	gaps := selectSemanticGaps(index, map[string]bool{"crypto/sm9.go": true}, nil, true)
	require.Len(t, gaps, 1)
	require.Equal(t, "crypto/sm9.go", gaps[0].Path)
	require.Equal(t, []string{"encrypt"}, gaps[0].Symbols)
}

func TestSemanticGapSelectionUsesCalledMethodsAsEvidence(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "entry", Path: "entry.go", Kind: "function", Exported: true},
		{ID: "leaf", Path: "leaf.go", Kind: "method", Exported: true},
		{ID: "behavior", Path: "behavior.go", Kind: "method", Exported: true},
		{ID: "store", Path: "store.go", Kind: "method"},
	}, Calls: []codegraph.Call{
		{Source: "entry", Target: "leaf"}, {Source: "entry", Target: "behavior"},
		{Source: "behavior", Target: "store"},
	}}
	changed := map[string]bool{"entry.go": true, "leaf.go": true, "behavior.go": true}
	gaps := selectSemanticGaps(index, changed, nil, true)
	require.Len(t, gaps, 1)
	require.Equal(t, "entry.go", gaps[0].Path)
	require.ElementsMatch(t, []string{"leaf", "behavior"}, gaps[0].Related)
	// 增量修改时上游未变，公开叶子仍是独立复核对象。
	gaps = selectSemanticGaps(index, map[string]bool{"leaf.go": true}, nil, false)
	require.Len(t, gaps, 1)
	require.Equal(t, "leaf.go", gaps[0].Path)
}

func TestSemanticGapSelectionKeepsOneQuestionPerPublicCallChain(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{
		{ID: "entry", Path: "entry.go", Kind: "function", Exported: true},
		{ID: "decision", Path: "decision.go", Kind: "function", Exported: true},
		{ID: "store", Path: "store.go", Kind: "function", Exported: true},
		{ID: "independent", Path: "store.go", Kind: "function", Exported: true},
	}, Calls: []codegraph.Call{
		{Source: "entry", Target: "decision"}, {Source: "decision", Target: "store"},
	}}
	changed := map[string]bool{"entry.go": true, "decision.go": true, "store.go": true}
	gaps := selectSemanticGaps(index, changed, nil, true)
	require.ElementsMatch(t, []string{"entry.go", "store.go"}, []string{gaps[0].Path, gaps[1].Path})

	// 修改独立下游实现而上游未变化时，仍须重新检查这个语义边界。
	gaps = selectSemanticGaps(index, map[string]bool{"decision.go": true}, nil, false)
	require.Len(t, gaps, 1)
	require.Equal(t, "decision.go", gaps[0].Path)
}

func TestSupplementUsesFullSymbolFromLargeFile(t *testing.T) {
	root := t.TempDir()
	content := "package domain\n" + strings.Repeat("// unrelated\n", 5000) + "func Decide() { Policy(); Store() }\n"
	writeLearnFile(t, root, "flow.go", content)
	line := strings.Count(content, "\n")
	index := &codegraph.Index{Files: []codegraph.File{{Path: "flow.go"}}, Nodes: []codegraph.Node{{ID: "flow", Path: "flow.go", Name: "Decide", Kind: "function", Line: line, EndLine: line}}}
	input, err := supplementEvidence(root, index, []semanticGap{{Path: "flow.go", Symbols: []string{"flow"}}}, nil)
	require.NoError(t, err)
	require.Contains(t, input.Evidence, "func Decide()")
	require.NotContains(t, input.Evidence, "unrelated")
	require.NotEmpty(t, input.SourceHashes["flow.go"])
}

func TestSupplementAcceptsLargeSingleSymbolWithinAgentBudget(t *testing.T) {
	root := t.TempDir()
	content := "package domain\nfunc Decide() { _ = \"" + strings.Repeat("x", 104*1024) + "\" }\n"
	writeLearnFile(t, root, "flow.go", content)
	index := &codegraph.Index{Files: []codegraph.File{{Path: "flow.go"}}, Nodes: []codegraph.Node{{
		ID: "decision", Path: "flow.go", Name: "Decide", Kind: "function", Line: 2, EndLine: 2,
	}}}
	input, err := supplementEvidence(root, index, []semanticGap{{Path: "flow.go", Symbols: []string{"decision"}}}, nil)
	require.NoError(t, err)
	require.Contains(t, input.Evidence, "func Decide()")
	require.LessOrEqual(t, len(input.Evidence), supplementSingleSourceLimit)
}

func TestSupplementUsesAnchorsForExistingKnowledge(t *testing.T) {
	root := t.TempDir()
	content := "package domain\nfunc Decide() { return }\n" + strings.Repeat("// unrelated\n", 8000)
	writeLearnFile(t, root, "flow.go", content)
	index := &codegraph.Index{Files: []codegraph.File{{Path: "flow.go"}}, Nodes: []codegraph.Node{{
		ID: "decision", Path: "flow.go", Name: "Decide", Kind: "function", Line: 2, EndLine: 2,
	}}}
	pattern := admittedLearnCurrentPatternForTest("decision", "Decision", domain.CategoryBusiness, "flow.go")
	pattern.EvidenceLocations[0].Line = 2
	input, err := supplementEvidence(root, index, []semanticGap{{Path: "flow.go", Existing: true}}, []domain.Pattern{*pattern})
	require.NoError(t, err)
	require.Contains(t, input.Evidence, "func Decide()")
	require.NotContains(t, input.Evidence, "unrelated")
}

func TestLocalCoverageIsCheckpointedWithoutSuppressingExplicitSemanticLearning(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	cont.Agent.(*mocks.MockAgent).AvailableVal = false
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, root, "helper.go", "package sample\nfunc helper() {}\n")
	gitAddAll(t, root)
	index := &codegraph.Index{Files: []codegraph.File{{Path: "main.go"}, {Path: "helper.go"}}, Nodes: []codegraph.Node{{ID: "helper", Path: "helper.go", Kind: "function", Name: "helper", Line: 2, EndLine: 2}}}
	run := func(opts learnCurrentProjectOptions) (supplementStatus, learnCurrentProjectResult) {
		t.Helper()
		var result learnCurrentProjectResult
		require.NoError(t, supplementLocalProject(context.Background(), cont, opts, root, index, &result))
		data, err := os.ReadFile(filepath.Join(cont.SeedPath, "runtime", "local-supplement.json"))
		require.NoError(t, err)
		var status supplementStatus
		require.NoError(t, json.Unmarshal(data, &status))
		return status, result
	}
	status, result := run(learnCurrentProjectOptions{})
	require.Empty(t, status.Pending)
	require.Equal(t, 2, result.changedCount)
	records, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	require.Len(t, records, 2)
	for _, record := range records {
		require.Equal(t, domain.FileAnalysisStatusSelectionSkipped, record.AnalysisStatus)
	}
	_, result = run(learnCurrentProjectOptions{})
	require.Zero(t, result.changedCount)
	status, _ = run(learnCurrentProjectOptions{semantic: true, focusPaths: []string{"helper.go"}})
	require.Len(t, status.Pending, 1)
	require.Equal(t, "helper.go", status.Pending[0].Path)
}

func TestSupplementCapacityBlockPersistsForUnchangedSource(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	cont.Agent.(*mocks.MockAgent).AvailableVal = false
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	content := "package domain\nfunc Decide() { _ = \"" + strings.Repeat("x", supplementSingleSourceLimit) + "\" }\n"
	writeLearnFile(t, root, "large.go", content)
	gitAddAll(t, root)
	index := &codegraph.Index{Files: []codegraph.File{{Path: "large.go"}}, Nodes: []codegraph.Node{{
		ID: "decision", Path: "large.go", Name: "Decide", Kind: "function", Line: 2, EndLine: 2, Exported: true,
	}}}
	run := func() supplementStatus {
		t.Helper()
		require.NoError(t, supplementLocalProject(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{}))
		data, err := os.ReadFile(filepath.Join(cont.SeedPath, "runtime", "local-supplement.json"))
		require.NoError(t, err)
		var status supplementStatus
		require.NoError(t, json.Unmarshal(data, &status))
		return status
	}
	first := run()
	require.Len(t, first.Pending, 1)
	require.NotEmpty(t, first.Blocked["large.go"].SourceHash)
	require.NotEmpty(t, first.Blocked["large.go"].InputHash)
	require.Contains(t, first.Blocked["large.go"].Reason, "safety limit")
	second := run()
	require.Equal(t, first.Blocked, second.Blocked)
	require.Len(t, second.Pending, 1)
	index.Nodes[0].Signature = "func Decide()"
	third := run()
	require.NotEqual(t, second.Blocked["large.go"].InputHash, third.Blocked["large.go"].InputHash)
}

func TestSupplementInputHashTracksExistingKnowledge(t *testing.T) {
	index := &codegraph.Index{Nodes: []codegraph.Node{{ID: "decision", Path: "flow.go", Line: 2, EndLine: 4}}}
	gap := semanticGap{Path: "flow.go", Symbols: []string{"decision"}, Existing: true}
	pattern := admittedLearnCurrentPatternForTest("decision", "Decision", domain.CategoryBusiness, "flow.go")
	pattern.EvidenceLocations[0].Line = 2
	initial := supplementInputHash(index, gap, []domain.Pattern{*pattern})
	pattern.EvidenceLocations[0].Line = 3
	require.NotEqual(t, initial, supplementInputHash(index, gap, []domain.Pattern{*pattern}))
}

func TestLocalSupplementAutomaticReviewResumeAndUnchanged(t *testing.T) {
	for _, failure := range []string{"", "analysis", "review", "unavailable"} {
		t.Run(failure, func(t *testing.T) {
			cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
			cfg := cont.ConfigRepo.Get()
			require.NoError(t, cont.ConfigRepo.Update(cfg))
			cont.PatternReader = cont.PatternRepo
			mock := cont.Agent.(*mocks.MockAgent)
			cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
			cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
			root := cfg.Project.RootPath
			writeLearnFile(t, root, "main.go", "package main\nfunc Shared() { A(); B() }\n")
			index := &codegraph.Index{Files: []codegraph.File{{Path: "main.go"}}, Nodes: []codegraph.Node{
				{ID: "shared", Path: "main.go", Kind: "function", Name: "Shared", Line: 2, EndLine: 2},
				{ID: "a", Path: "a.go", Kind: "function", Name: "A", Line: 2, EndLine: 2},
				{ID: "b", Path: "b.go", Kind: "function", Name: "B", Line: 2, EndLine: 2},
			}, Calls: []codegraph.Call{{Source: "shared", Target: "a"}, {Source: "shared", Target: "b"}}, Dependencies: []codegraph.Dependency{
				{Source: "a.go", Target: "main.go"}, {Source: "b.go", Target: "main.go"}, {Source: "c.go", Target: "main.go"},
			}}
			for _, path := range []string{"a.go", "b.go", "c.go"} {
				content := "package main\n"
				if path == "a.go" {
					content += "func A() {}\n"
				} else if path == "b.go" {
					content += "func B() {}\n"
				}
				writeLearnFile(t, root, path, content)
				index.Files = append(index.Files, codegraph.File{Path: path})
			}
			gitAddAll(t, root)
			analyses, reviews := 0, 0
			fail := failure != ""
			mock.RefreshProjectProfileFn = func(context.Context, *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
				t.Fatal("automatic supplement must not refresh whole project")
				return nil, nil
			}
			mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
				analyses++
				require.NotNil(t, agent.CurrentLearningSupplement(ctx))
				require.Contains(t, req.StructuralContext, "func Shared()")
				assessment := "resolved"
				if fail && failure == "analysis" {
					assessment = "unresolved"
				}
				return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: assessment, Focuses: []agent.AnalyzeCurrentEvidenceResult{{
					FocusID: req.Focuses[0].EvidenceFocus.ID, FocusName: req.Focuses[0].EvidenceFocus.Name,
					Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("shared-use", "Shared capability", domain.CategoryBusiness, "main.go")},
				}}}, nil
			}
			mock.ReviewKnowledgeFn = func(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
				reviews++
				if fail && failure == "review" {
					return nil, errors.New("review timeout")
				}
				decisions := make([]agent.KnowledgeReviewDecision, 0, len(req.Candidates))
				for _, p := range req.Candidates {
					decisions = append(decisions, agent.KnowledgeReviewDecision{CandidateID: p.ID, Verdict: "accept", ReasonCode: "accepted", Reason: "source verified"})
				}
				return &agent.ReviewKnowledgeResult{Decisions: decisions}, nil
			}
			mock.AvailableVal = failure != "unavailable"
			run := func() supplementStatus {
				t.Helper()
				err := supplementLocalProject(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{})
				require.NoError(t, err)
				data, err := os.ReadFile(filepath.Join(cont.SeedPath, "runtime", "local-supplement.json"))
				require.NoError(t, err)
				var status supplementStatus
				require.NoError(t, json.Unmarshal(data, &status))
				return status
			}
			status := run()
			require.Equal(t, 4, status.IndexedFiles)
			require.Equal(t, 1, status.SemanticCandidates)
			if failure != "" {
				require.Len(t, status.Pending, 1)
				require.NotEmpty(t, status.LastError)
				records, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
				require.NoError(t, err)
				require.Len(t, records, 3)
				for _, record := range records {
					require.NotEqual(t, "main.go", record.Path)
					require.Equal(t, domain.FileAnalysisStatusSelectionSkipped, record.AnalysisStatus)
				}
				fail = false
				mock.AvailableVal = true
				status = run()
			}
			require.Empty(t, status.Pending, status.LastError)
			require.Positive(t, reviews)
			if failure == "review" {
				require.Equal(t, 1, analyses, "resume only missing review")
			}
			before := analyses
			require.Empty(t, run().Pending)
			require.Equal(t, before, analyses, "unchanged confirmed evidence must not call AI again")
		})
	}
}

func TestSupplementPersistsOnlyReviewedCodeGraphCapability(t *testing.T) {
	for _, reviewLine := range []int{2, 3} {
		t.Run(fmt.Sprintf("review-line-%d", reviewLine), func(t *testing.T) {
			cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
			cont.PatternReader = cont.PatternRepo
			mock := cont.Agent.(*mocks.MockAgent)
			cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
			cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
			root := cont.ConfigRepo.GetProjectConfig().RootPath
			path := "crypto/sm9.go"
			writeLearnFile(t, root, path, "package crypto\nfunc EncryptSM9(key, data []byte) ([]byte, error) { return data, nil }\n")
			gitAddAll(t, root)
			index := &codegraph.Index{Files: []codegraph.File{{Path: path}}, Nodes: []codegraph.Node{{
				ID: "encrypt", Path: path, Kind: "function", Name: "EncryptSM9", Line: 2, EndLine: 2,
				Exported: true, Signature: "func EncryptSM9(key, data []byte) ([]byte, error)",
			}}}
			method := domain.BusinessMethod{
				Name: "EncryptSM9", CodeLocation: domain.CodeLocation{CurrentLocation: path + ":2"},
				Description: "Encrypts data using the existing SM9 implementation.",
				Usage:       "Use for compatible SM9 encryption requests.", Type: "common",
				Function: "invented signature", Prerequisites: "SM9 key", Returns: "ciphertext or error",
			}
			mock.AnalyzeCurrentBatchFn = func(_ context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
				candidate := admittedLearnCurrentPatternForTest("sm9-encryption", "SM9 encryption", domain.CategoryUtils, path)
				candidate.EvidenceLocations[0].Line = 2
				candidate.BusinessMethod = &method
				return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{
					FocusID: req.Focuses[0].EvidenceFocus.ID, Patterns: []domain.Pattern{*candidate},
				}}}, nil
			}
			mock.ReviewKnowledgeFn = func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
				require.Len(t, req.Candidates, 1)
				reviewed := method
				reviewed.CodeLocation.CurrentLocation = fmt.Sprintf("%s:%d", path, reviewLine)
				return &agent.ReviewKnowledgeResult{Decisions: []agent.KnowledgeReviewDecision{{
					CandidateID: req.Candidates[0].ID, Verdict: "accept", ReasonCode: "accepted",
					Reason: "The source confirms this reusable entry.", BusinessMethod: &reviewed,
				}}}, nil
			}
			require.NoError(t, supplementLocalProject(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{}))
			stored, err := cont.PatternRepo.GetAll(context.Background())
			require.NoError(t, err)
			if reviewLine != 2 {
				require.Empty(t, stored)
				return
			}
			require.Len(t, stored, 1)
			require.Equal(t, domain.CategoryUtils, stored[0].Category)
			require.NotNil(t, stored[0].BusinessMethod)
			require.Equal(t, index.Nodes[0].Signature, stored[0].BusinessMethod.Function)
			require.NotEmpty(t, stored[0].EvidenceLocations[0].ContentHash)
		})
	}
}

func TestSupplementBatchGrowsWithEvidenceAndSplitsAtByteLimit(t *testing.T) {
	root := t.TempDir()
	index := &codegraph.Index{}
	var gaps []semanticGap
	for i := 0; i < 12; i++ {
		path := fmt.Sprintf("domain%d/order.go", i)
		writeLearnFile(t, root, path, "package domain\nfunc Decide() {}\n")
		index.Files = append(index.Files, codegraph.File{Path: path})
		gaps = append(gaps, semanticGap{Path: path, Reason: "business decision"})
	}
	input, err := supplementBatch(root, index, gaps, nil)
	require.NoError(t, err)
	require.Len(t, input.Focus.EntryPaths, 12, "related small problems share one call, not a fixed count")
	large := strings.Repeat("// domain evidence\n", 900)
	for _, gap := range gaps {
		writeLearnFile(t, root, gap.Path, "package domain\n"+large)
	}
	input, err = supplementBatch(root, index, gaps, nil)
	require.NoError(t, err)
	require.Greater(t, len(input.Focus.EntryPaths), 1)
	require.Less(t, len(input.Focus.EntryPaths), len(gaps))
	require.LessOrEqual(t, len(input.Evidence), supplementSourceLimit)
}

func TestSupplementBatchKeepsLargeSingleFileSeparate(t *testing.T) {
	root := t.TempDir()
	large := strings.Repeat("x", 80*1024)
	writeLearnFile(t, root, "large.go", large)
	writeLearnFile(t, root, "small.go", "package domain\nfunc Decide() {}\n")
	index := &codegraph.Index{Files: []codegraph.File{{Path: "large.go"}, {Path: "small.go"}}}
	gaps := []semanticGap{{Path: "large.go"}, {Path: "small.go"}}
	input, err := supplementBatch(root, index, gaps, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"large.go"}, input.Focus.EntryPaths)
	require.Greater(t, len(input.Evidence), supplementSourceLimit)
	require.LessOrEqual(t, len(input.Evidence), supplementSingleSourceLimit)
}

func TestSupplementCancellationStopsBeforeSemanticCalls(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, root, "main.go", "package main\nfunc Decide() {}\n")
	index := &codegraph.Index{Files: []codegraph.File{{Path: "main.go"}}, Nodes: []codegraph.Node{{Path: "main.go", Kind: "function"}}, Dependencies: []codegraph.Dependency{{Source: "caller.go", Target: "main.go"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := supplementLocalProject(ctx, cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestSupplementCancellationResumesOnlyUnfinishedBatch(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cfg := cont.ConfigRepo.Get()
	cfg.Agent.Parallelism = 1
	require.NoError(t, cont.ConfigRepo.Update(cfg))
	cont.PatternReader = cont.PatternRepo
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	paths := []string{"billing/create.go", "orders/create.go"}
	index := &codegraph.Index{}
	for _, path := range paths {
		content := "package domain\nfunc Decide() {\n" + strings.Repeat("// business evidence\n", 2200) + "Support()\n}\n"
		writeLearnFile(t, root, path, content)
		neighbor := filepath.ToSlash(filepath.Join(filepath.Dir(path), "support.go"))
		writeLearnFile(t, root, neighbor, "package domain\nfunc Support() {}\n")
		index.Files = append(index.Files, codegraph.File{Path: path}, codegraph.File{Path: neighbor})
		index.Nodes = append(index.Nodes, codegraph.Node{ID: path, Path: path, Kind: "function", Name: "Decide", Line: 2, EndLine: strings.Count(content, "\n")})
		index.Nodes = append(index.Nodes, codegraph.Node{ID: neighbor, Path: neighbor, Kind: "function", Name: "Support", Line: 2, EndLine: 2})
		index.Calls = append(index.Calls, codegraph.Call{Source: path, Target: neighbor})
	}
	gitAddAll(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	calls := make(map[string]int)
	interrupt := true
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		path := req.Focuses[0].FocusPaths[0]
		calls[path]++
		if interrupt && path == paths[1] {
			cancel()
			return nil, ctx.Err()
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{FocusID: req.Focuses[0].EvidenceFocus.ID}}}, nil
	}
	err := supplementLocalProject(ctx, cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{})
	require.ErrorIs(t, err, context.Canceled)
	data, err := os.ReadFile(filepath.Join(cont.SeedPath, "runtime", "local-supplement.json"))
	require.NoError(t, err)
	var status supplementStatus
	require.NoError(t, json.Unmarshal(data, &status))
	require.Len(t, status.Pending, 1)
	require.Equal(t, paths[1], status.Pending[0].Path)
	require.Equal(t, 1, calls[paths[0]])

	interrupt = false
	require.NoError(t, supplementLocalProject(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{}))
	require.Equal(t, 1, calls[paths[0]])
	require.Equal(t, 2, calls[paths[1]])
}

func TestSupplementFailureDoesNotBlockIndependentBatches(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	paths := []string{"billing/create.go", "orders/create.go"}
	index := &codegraph.Index{}
	for _, path := range paths {
		content := "package domain\nfunc Decide() {\n" + strings.Repeat("// business evidence\n", 2200) + "Policy(); Store()\n}\n"
		writeLearnFile(t, root, path, content)
		index.Files = append(index.Files, codegraph.File{Path: path})
		index.Nodes = append(index.Nodes, codegraph.Node{ID: path, Path: path, Kind: "function", Name: "Decide", Line: 2, EndLine: strings.Count(content, "\n")})
		for _, target := range []string{"policy.go", "store.go"} {
			neighbor := filepath.ToSlash(filepath.Join(filepath.Dir(path), target))
			writeLearnFile(t, root, neighbor, "package domain\nfunc Support() {}\n")
			index.Files = append(index.Files, codegraph.File{Path: neighbor})
			index.Nodes = append(index.Nodes, codegraph.Node{ID: neighbor, Path: neighbor, Kind: "function", Name: "Support", Line: 2, EndLine: 2})
			index.Calls = append(index.Calls, codegraph.Call{Source: path, Target: neighbor})
		}
	}
	gitAddAll(t, root)

	var analyzed []string
	var analyzedMu sync.Mutex
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		path := req.Focuses[0].FocusPaths[0]
		analyzedMu.Lock()
		analyzed = append(analyzed, path)
		analyzedMu.Unlock()
		if path == paths[0] {
			return nil, errors.New("first semantic batch failed")
		}
		focus := req.Focuses[0]
		return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{
			FocusID:  focus.EvidenceFocus.ID,
			Patterns: []domain.Pattern{*admittedLearnCurrentPatternForTest("order-decision", "Order decision", domain.CategoryBusiness, path)},
		}}}, nil
	}
	mock.ReviewKnowledgeFn = func(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		return acceptKnowledgeCandidates(req.Candidates), nil
	}

	var result learnCurrentProjectResult
	var started, completed, details []string
	steps := commandutil.NewConsoleStepRunner(commandutil.ConsoleStepRunnerOptions{
		TotalSteps:     learnCurrentProjectStepTotal,
		OnStepStart:    func(label string) { started = append(started, label) },
		OnStepComplete: func(label string) { completed = append(completed, label) },
		OnStepUpdate:   func(label string) { details = append(details, label) },
	})
	require.NoError(t, steps.Run(i18n.Get("LearnLocalIndex"), func() error { return nil }))
	require.NoError(t, supplementLocalProjectWithSteps(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &result, steps))
	require.Equal(t, []string{
		i18n.Get("LearnLocalIndex"), i18n.Get("ProgressLearnCurrentDetectChanges"),
		i18n.Get("LearnLocalPlan"), i18n.Get("LearnLocalCommitCoverage"),
		i18n.Get("LearnLocalSupplement"), i18n.Get("LearnLocalFinalize"),
	}, started)
	require.Equal(t, started, completed)
	require.NotContains(t, strings.Join(details, "\n"), paths[0])
	require.NotContains(t, strings.Join(details, "\n"), paths[1])
	require.Contains(t, strings.Join(details, "\n"), "1/2")
	require.NotContains(t, strings.Join(details, "\n"), "焦点 1/1")
	require.NotContains(t, strings.Join(details, "\n"), "focus 1/1")
	require.Len(t, analyzed, 2)
	require.Equal(t, 1, result.pendingCount)
	require.Equal(t, 1, result.savedCount)
	require.ElementsMatch(t, paths, analyzed)

	data, err := os.ReadFile(filepath.Join(cont.SeedPath, "runtime", "local-supplement.json"))
	require.NoError(t, err)
	var status supplementStatus
	require.NoError(t, json.Unmarshal(data, &status))
	require.Len(t, status.Pending, 1)
	require.Equal(t, paths[0], status.Pending[0].Path)
	require.Contains(t, status.LastError, "first semantic batch failed")
	stored, err := cont.PatternRepo.GetAll(context.Background())
	require.NoError(t, err)
	require.Len(t, stored, 1)
}

func TestSupplementUsesConfiguredAgentParallelism(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	config := cont.ConfigRepo.Get()
	config.Agent.Parallelism = 2
	require.NoError(t, cont.ConfigRepo.Update(config))
	cont.PatternReader = cont.PatternRepo
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := config.Project.RootPath
	index := &codegraph.Index{}
	paths := []string{"billing/create.go", "orders/create.go", "shipping/create.go"}
	for _, path := range paths {
		content := "package domain\nfunc Decide() {\n" + strings.Repeat("// evidence\n", 3500) + "Support()\n}\n"
		writeLearnFile(t, root, path, content)
		neighbor := filepath.ToSlash(filepath.Join(filepath.Dir(path), "support.go"))
		writeLearnFile(t, root, neighbor, "package domain\nfunc Support() {}\n")
		index.Files = append(index.Files, codegraph.File{Path: path}, codegraph.File{Path: neighbor})
		index.Nodes = append(index.Nodes, codegraph.Node{ID: path, Path: path, Kind: "function", Name: "Decide", Line: 2, EndLine: strings.Count(content, "\n")})
		index.Nodes = append(index.Nodes, codegraph.Node{ID: neighbor, Path: neighbor, Kind: "function", Name: "Support", Line: 2, EndLine: 2})
		index.Calls = append(index.Calls, codegraph.Call{Source: path, Target: neighbor})
	}
	gitAddAll(t, root)
	started := make(chan string, len(paths))
	release := make(chan struct{})
	var activeMu sync.Mutex
	active, peak := 0, 0
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		activeMu.Lock()
		active++
		peak = max(peak, active)
		activeMu.Unlock()
		defer func() {
			activeMu.Lock()
			active--
			activeMu.Unlock()
		}()
		started <- req.Focuses[0].FocusPaths[0]
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		focus := req.Focuses[0]
		return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{FocusID: focus.EvidenceFocus.ID}}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- supplementLocalProject(ctx, cont, learnCurrentProjectOptions{force: true}, root, index, &learnCurrentProjectResult{})
	}()
	var observed []string
	for len(observed) < 2 {
		select {
		case path := <-started:
			observed = append(observed, path)
		case <-ctx.Done():
			close(release)
			require.NoError(t, <-done)
			t.Fatal("independent batches did not start concurrently")
		}
	}
	close(release)
	for len(observed) < len(paths) {
		select {
		case path := <-started:
			observed = append(observed, path)
		case <-ctx.Done():
			t.Fatal("remaining batch did not complete")
		}
	}
	require.ElementsMatch(t, paths, observed)
	require.NoError(t, <-done)
	require.Equal(t, 2, peak)
}

func TestSupplementCombinesBusinessCandidatesWithoutAINormalization(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	index := &codegraph.Index{}
	for _, path := range []string{"orders/create.go", "orders/confirm.go", "orders/cancel.go"} {
		writeLearnFile(t, root, path, "package orders\nfunc Decide() { Policy(); Store() }\n")
		index.Files = append(index.Files, codegraph.File{Path: path})
		index.Nodes = append(index.Nodes, codegraph.Node{ID: path, Path: path, Kind: "function", Name: "Decide", Line: 2, EndLine: 2})
		index.Calls = append(index.Calls, codegraph.Call{Source: path, Target: "policy"}, codegraph.Call{Source: path, Target: "store"})
	}
	for _, target := range []struct{ id, path, name string }{{"policy", "orders/policy.go", "Policy"}, {"store", "orders/store.go", "Store"}} {
		writeLearnFile(t, root, target.path, "package orders\nfunc "+target.name+"() {}\n")
		index.Files = append(index.Files, codegraph.File{Path: target.path})
		index.Nodes = append(index.Nodes, codegraph.Node{ID: target.id, Path: target.path, Kind: "function", Name: target.name, Line: 2, EndLine: 2})
	}
	analyses, reviews := 0, 0
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		analyses++
		require.Len(t, req.Focuses, 1)
		require.Len(t, req.Focuses[0].FocusPaths, 3)
		return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{FocusID: req.Focuses[0].EvidenceFocus.ID, Patterns: []domain.Pattern{
			*admittedLearnCurrentPatternForTest("order-create", "Order creation decision", domain.CategoryBusiness, "orders/create.go"),
			*admittedLearnCurrentPatternForTest("order-cancel", "Order cancellation decision", domain.CategoryBusiness, "orders/cancel.go"),
			*admittedLearnCurrentPatternForTest("generic-error", "Generic error handling", domain.CategoryError, "orders/create.go"),
		}}}}, nil
	}
	mock.ReviewKnowledgeFn = func(ctx context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		reviews++
		require.Len(t, req.Candidates, 2)
		require.ElementsMatch(t, []string{"order-create", "order-cancel"}, []string{req.Candidates[0].ID, req.Candidates[1].ID})
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
	mock.NormalizePatternsFn = func(context.Context, *agent.NormalizePatternsRequest) (*agent.NormalizePatternsResult, error) {
		t.Fatal("no extra AI merge")
		return nil, nil
	}
	require.NoError(t, supplementLocalProject(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{}))
	require.Equal(t, 1, analyses)
	require.Equal(t, 1, reviews)
	stored, err := cont.PatternRepo.GetAll(context.Background())
	require.NoError(t, err)
	require.Len(t, stored, 2)
	for _, pattern := range stored {
		require.NotEmpty(t, pattern.EvidenceLocations[0].ContentHash)
	}
	require.NoError(t, supplementLocalProject(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{}))
	require.Equal(t, 1, analyses)
}
