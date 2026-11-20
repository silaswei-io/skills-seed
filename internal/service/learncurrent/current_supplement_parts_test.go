package learncurrent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestSupplementRechecksCurrentFileWhenStoredAnchorIsOutdated(t *testing.T) {
	root := t.TempDir()
	content := "package policy\nfunc Current() {}\n"
	writeLearnFile(t, root, "policy.go", content)
	pattern := admittedLearnCurrentPatternForTest("old", "Old", domain.CategoryBusiness, "policy.go")
	pattern.EvidenceLocations[0].Line = 430
	index := &codegraph.Index{Files: []codegraph.File{{Path: "policy.go"}}}
	input, err := supplementEvidence(root, index, []semanticGap{{Path: "policy.go", Existing: true}}, []domain.Pattern{*pattern})
	require.NoError(t, err)
	var evidence struct{ Sources map[string]string }
	require.NoError(t, json.Unmarshal([]byte(input.Evidence), &evidence))
	require.Equal(t, content, evidence.Sources["policy.go"])
	require.Equal(t, []agent.SourceRange{{Start: 1, End: 3}}, input.SourceRanges["policy.go"])

	large := content + strings.Repeat("// unrelated\n", 20000)
	_, _, err = supplementSource("policy.go", []byte(large), nil, []int{30000}, true, supplementSingleSourceLimit, nil)
	require.ErrorIs(t, err, errSupplementEvidenceCapacity)
	require.ErrorContains(t, err, "stored anchor 30000")
	require.ErrorContains(t, err, "20003 lines")
	_, _, err = supplementSource("policy.go", []byte(content), []string{"stale-symbol"}, nil, false, supplementSingleSourceLimit, nil)
	require.ErrorIs(t, err, agent.ErrSupplementUnresolved)
	require.ErrorContains(t, err, `symbol "stale-symbol" has invalid range`)
}

func TestSupplementPartsKeepEachPatternsCompleteEvidence(t *testing.T) {
	root := t.TempDir()
	writeLearnFile(t, root, "flow.go", "package flow\nfunc First() {}\nfunc Second() {}\n")
	index := &codegraph.Index{Files: []codegraph.File{{Path: "flow.go"}}}
	var patterns []domain.Pattern
	for i, path := range []string{"first.go", "second.go"} {
		writeLearnFile(t, root, path, "package flow\n"+strings.Repeat("// complete evidence\n", 4800))
		index.Files = append(index.Files, codegraph.File{Path: path})
		pattern := admittedLearnCurrentPatternForTest(path, path, domain.CategoryBusiness, "flow.go")
		pattern.EvidenceLocations[0].Line = i + 2
		pattern.EvidenceLocations = append(pattern.EvidenceLocations, domain.PatternEvidenceLocation{Path: path, Kind: "file"})
		patterns = append(patterns, *pattern)
	}
	gap := semanticGap{Path: "flow.go", Existing: true}
	_, err := supplementEvidence(root, index, []semanticGap{gap}, patterns)
	require.ErrorIs(t, err, errSupplementEvidenceCapacity)
	parts, err := supplementParts(root, index, gap, patterns)
	require.NoError(t, err)
	require.Len(t, parts, 2)
	for i, input := range parts {
		require.True(t, input.Partial)
		require.Equal(t, []string{patterns[i].ID}, input.PatternIDs)
		require.Equal(t, []string{"flow.go"}, input.Focus.EntryPaths)
		require.Len(t, input.SourceHashes, 2)
		for _, evidence := range patterns[i].EvidenceLocations {
			require.Contains(t, input.SourceHashes, evidence.Path)
		}
		require.LessOrEqual(t, len(input.Evidence), supplementSingleSourceLimit)
	}
	require.NotEqual(t, parts[0].Focus.ID, parts[1].Focus.ID)
}

func TestSupplementPartsRestrictRelatedKnowledge(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	for _, id := range []string{"first", "second"} {
		require.NoError(t, cont.PatternRepo.Save(context.Background(), admittedLearnCurrentPatternForTest(id, id, domain.CategoryBusiness, "main.go")))
	}
	for _, ids := range [][]string{{"first"}, {}} {
		ctx := agent.WithLearningSupplement(context.Background(), &agent.LearningSupplement{Partial: true, PatternIDs: ids})
		run := newLearnCurrentProjectRun(ctx, cont, learnCurrentProjectOptions{})
		run.projectRoot = root
		related, err := run.relatedPatternsByFocus(ctx, []analyzer.AnalyzeCurrentEvidenceFocus{{EvidenceFocus: domain.EvidenceFocus{ID: "part"}, FocusAbsPaths: []string{filepath.Join(root, "main.go")}}})
		require.NoError(t, err)
		require.Len(t, related["part"], len(ids))
		if len(ids) > 0 {
			require.Equal(t, ids[0], related["part"][0].ID)
		}
	}
}

func TestSupplementPartsResumeWithoutCommittingPartialFile(t *testing.T) {
	for _, failure := range []string{"analysis", "review"} {
		t.Run(failure, func(t *testing.T) { testSupplementPartsResume(t, failure) })
	}
}

func testSupplementPartsResume(t *testing.T, failure string) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	content := "package policy\n"
	index := &codegraph.Index{Files: []codegraph.File{{Path: "large.go"}}}
	for i, name := range []string{"First", "Second", "Third"} {
		content += fmt.Sprintf("func %s() { _ = %q }\n", name, strings.Repeat("x", 90*1024))
		index.Nodes = append(index.Nodes, codegraph.Node{ID: name, Name: name, Path: "large.go", Kind: "function", Exported: true, Line: i + 2, EndLine: i + 2})
	}
	writeLearnFile(t, root, "large.go", content)
	gitAddAll(t, root)
	calls := make(map[string]int)
	reviews := 0
	fail := true
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		input := agent.CurrentLearningSupplement(ctx)
		require.True(t, input.Partial)
		calls[input.Focus.ID]++
		second := strings.Contains(input.Evidence, "func Second()")
		if fail && failure == "analysis" && second {
			return nil, errors.New("analysis unavailable")
		}
		var patterns []domain.Pattern
		if failure == "review" && second {
			pattern := admittedLearnCurrentPatternForTest("second-policy", "Second policy", domain.CategoryBusiness, "large.go")
			pattern.EvidenceLocations[0].Line = 3
			patterns = append(patterns, *pattern)
		}
		return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{FocusID: req.Focuses[0].EvidenceFocus.ID, Patterns: patterns}}}, nil
	}
	mock.ReviewKnowledgeFn = func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		reviews++
		if fail {
			return nil, errors.New("review unavailable")
		}
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
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
	require.Equal(t, "learning", first.Pending[0].Failure.Stage)
	require.Contains(t, first.Pending[0].Failure.Reason, failure+" unavailable")
	tracked, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	for _, record := range tracked {
		require.NotEqual(t, "large.go", record.Path, "one successful part must not advance the file baseline")
	}
	gap := selectSemanticGaps(index, map[string]bool{"large.go": true}, nil, false)[0]
	parts, err := supplementParts(root, index, gap, nil)
	require.NoError(t, err)
	require.Len(t, parts, 3)
	fail = false
	require.Empty(t, run().Pending)
	require.Equal(t, 1, calls[parts[0].Focus.ID], "reuse the completed part checkpoint")
	if failure == "analysis" {
		require.Equal(t, 2, calls[parts[1].Focus.ID])
	} else {
		require.Equal(t, 1, calls[parts[1].Focus.ID], "resume only the unfinished review")
		require.Equal(t, 2, reviews)
	}
	require.Equal(t, 1, calls[parts[2].Focus.ID])
	tracked, err = cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	confirmed := false
	for _, record := range tracked {
		if record.Path == "large.go" {
			confirmed = true
			require.Equal(t, domain.FileAnalysisStatusAnalyzed, record.AnalysisStatus)
		}
	}
	require.True(t, confirmed)
	for _, input := range parts {
		_, err := learnCurrentStateRepo(cont.SeedPath, supplementStateScope(learnCurrentProjectOptions{}, input)).Load(context.Background())
		require.ErrorIs(t, err, commandstate.ErrStateNotFound)
	}
	require.Empty(t, run().Pending)
	require.Len(t, calls, 3)
}

func TestSupplementPartsRejectChangedEvidenceBeforeFileCompletion(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	input, err := supplementEvidence(root, &codegraph.Index{Files: []codegraph.File{{Path: "main.go"}}}, []semanticGap{{Path: "main.go"}}, nil)
	require.NoError(t, err)
	writeLearnFile(t, root, "main.go", "package main\nfunc Updated() {}\n")
	err = commitSupplementParts(context.Background(), cont, root, &fileanalysis.FileChanges{}, supplementJob{input: input, parts: []*agent.LearningSupplement{input}})
	require.ErrorIs(t, err, agent.ErrSupplementUnresolved)
	require.ErrorContains(t, err, "main.go changed before file completion")
}

func TestSupplementPartsRetainReviewedWritesAfterLaterFailure(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	mock := cont.Agent.(*mocks.MockAgent)
	cont.AnalyzerSvc = analyzer.NewAnalyzerService(mock, cont.ConfigRepo)
	cont.PatternNormSvc = patternnorm.NewServiceWithNormalizer(cont.PatternRepo, mock)
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, root, "main.go", "package main\nfunc First() {}\nfunc Second() {}\n")
	gitAddAll(t, root)
	var parts []*agent.LearningSupplement
	for i, name := range []string{"First", "Second"} {
		index := &codegraph.Index{Files: []codegraph.File{{Path: "main.go"}}, Nodes: []codegraph.Node{{ID: name, Name: name, Path: "main.go", Kind: "function", Line: i + 2, EndLine: i + 2}}}
		input, err := supplementEvidence(root, index, []semanticGap{{Path: "main.go", Symbols: []string{name}}}, nil)
		require.NoError(t, err)
		input.Focus.ID, input.Partial, input.PatternIDs = name, true, []string{}
		parts = append(parts, input)
	}
	mock.AnalyzeCurrentBatchFn = func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
		if agent.CurrentLearningSupplement(ctx).Focus.ID == "Second" {
			return nil, errors.New("second part unavailable")
		}
		pattern := admittedLearnCurrentPatternForTest("first-policy", "First policy", domain.CategoryBusiness, "main.go")
		pattern.EvidenceLocations[0].Line = 2
		return &agent.AnalyzeCurrentCodebaseBatchResult{Assessment: "resolved", Focuses: []agent.AnalyzeCurrentEvidenceResult{{FocusID: req.Focuses[0].EvidenceFocus.ID, Patterns: []domain.Pattern{*pattern}}}}, nil
	}
	mock.ReviewKnowledgeFn = func(_ context.Context, req *agent.ReviewKnowledgeRequest) (*agent.ReviewKnowledgeResult, error) {
		return acceptKnowledgeCandidates(req.Candidates), nil
	}
	result, err := runSupplementParts(context.Background(), cont, learnCurrentProjectOptions{}, supplementJob{input: parts[0], parts: parts})
	require.ErrorContains(t, err, "second part unavailable")
	require.Equal(t, 1, result.savedCount)
	stored, err := cont.PatternRepo.GetAll(context.Background())
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Equal(t, "first-policy", stored[0].ID)
	tracked, err := cont.FileTracker.ListAnalyzedFiles(context.Background(), domain.FileAnalysisScope{})
	require.NoError(t, err)
	require.Empty(t, tracked)
}

func TestSupplementReportsEachCandidatesFailure(t *testing.T) {
	cont := newLearnCurrentTestContainer(t, domain.ModeProject, nil)
	cont.PatternReader = cont.PatternRepo
	cont.Agent.(*mocks.MockAgent).AvailableVal = false
	root := cont.ConfigRepo.GetProjectConfig().RootPath
	writeLearnFile(t, root, "large.go", strings.Repeat("x", supplementSingleSourceLimit+1))
	writeLearnFile(t, root, "small.go", "package main\nfunc Small() {}\n")
	gitAddAll(t, root)
	index := &codegraph.Index{Files: []codegraph.File{{Path: "large.go"}, {Path: "small.go"}}, Nodes: []codegraph.Node{
		{ID: "large", Path: "large.go", Kind: "function", Exported: true, Line: 1, EndLine: 1},
		{ID: "small", Path: "small.go", Kind: "function", Exported: true, Line: 2, EndLine: 2},
	}}
	require.NoError(t, supplementLocalProject(context.Background(), cont, learnCurrentProjectOptions{}, root, index, &learnCurrentProjectResult{}))
	data, err := os.ReadFile(filepath.Join(cont.SeedPath, "runtime", "local-supplement.json"))
	require.NoError(t, err)
	var status supplementStatus
	require.NoError(t, json.Unmarshal(data, &status))
	require.Len(t, status.Pending, 2)
	require.Equal(t, "evidence", status.Pending[0].Failure.Stage)
	require.Contains(t, status.Pending[0].Failure.Reason, "safety limit")
	require.Equal(t, "agent", status.Pending[1].Failure.Stage)
	require.Contains(t, status.Pending[1].Failure.Reason, "mock")
}
