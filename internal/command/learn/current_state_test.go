package learn

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/stretchr/testify/require"
)

func TestBuildStateFilesPreservesAnalysisMetadata(t *testing.T) {
	changes := &fileanalysis.FileChanges{
		Records: []domain.FileAnalysisRecord{
			{Path: "internal/key/create.go", Hash: "key-hash", HashAlgorithm: domain.FileAnalysisHashMD5, Size: 42, Source: domain.FileAnalysisSourceCurrentCode, AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
			{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusSelectionSkipped, SelectionReason: "low signal"},
		},
		Deleted: []string{"internal/removed.go"},
	}

	files := buildStateFiles(changes)

	require.Equal(t, changes.Records, files)
	require.Equal(t, "low signal", files[1].SelectionReason)
}

func TestChangesFromCurrentStateRestoresCompleteRecords(t *testing.T) {
	record := domain.FileAnalysisRecord{
		ProjectID:       "backend",
		ScopePath:       "services",
		Path:            "internal/key/create.go",
		Hash:            "key-hash",
		HashAlgorithm:   domain.FileAnalysisHashMD5,
		Size:            42,
		ModTime:         "2026-07-23T12:00:00+08:00",
		Source:          domain.FileAnalysisSourceCurrentCode,
		AnalysisStatus:  domain.FileAnalysisStatusSelectionSkipped,
		SelectionReason: "low signal",
		LastAnalyzedAt:  "2026-07-23T12:01:00+08:00",
	}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{record}, []string{"internal/removed.go"}, nil)

	changes := changesFromCurrentState(state)

	require.Equal(t, []domain.FileAnalysisRecord{record}, changes.Records)
	require.Equal(t, []string{"internal/removed.go"}, changes.Deleted)
}

func TestReconcileEvidenceFocusesFiltersInvalidPathsAndNormalizesPolicy(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{
			ID:            "auth",
			Name:          "Auth",
			Attributes:    []string{" Stable ", "stable", "Critical"},
			RiskSignals:   []string{" Review ", "review"},
			AnalysisDepth: "unsupported",
			EntryPaths:    []string{"internal/auth/login.go", "internal/auth", "outside.go", "../escape.go"},
			RelatedPaths:  []string{"internal/auth/login.go", "internal/auth/types.go", "/tmp/escape.go"},
		},
	}
	allowed := []string{
		"internal/auth/login.go",
		"internal/auth/types.go",
		"internal/key/create.go",
	}

	got := reconcileEvidenceFocuses(focuses, allowed)

	require.Equal(t, []domain.EvidenceFocus{
		{
			ID:            "auth",
			Name:          "Auth",
			Attributes:    []string{"critical", "stable"},
			RiskSignals:   []string{"review"},
			AnalysisDepth: domain.EvidenceFocusDepthStandard,
			EntryPaths:    []string{"internal/auth/login.go"},
			RelatedPaths:  []string{"internal/auth/types.go"},
		},
	}, got)
	require.Equal(t, []string{"internal/key/create.go"}, uncoveredAnalysisPaths(got, allowed))
}

func TestReconcileEvidenceFocusesPreservesDistinctAgentFocuses(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{
			ID:         "primary-workflow",
			Name:       "Primary workflow",
			EntryPaths: []string{"area/entry", "area/shared"},
		},
		{
			ID:           "supporting-policy",
			Name:         "Supporting policy",
			RouteTerms:   []string{"policy"},
			EntryPaths:   []string{"area/policy"},
			RelatedPaths: []string{"area/shared"},
		},
	}

	got := reconcileEvidenceFocuses(focuses, []string{
		"area/entry",
		"area/policy",
		"area/shared",
	})

	require.Len(t, got, 2)
	require.Equal(t, "primary-workflow", got[0].ID)
	require.Equal(t, "supporting-policy", got[1].ID)
	require.Equal(t, []string{"area/policy"}, got[1].EntryPaths)
	require.Equal(t, []string{"area/shared"}, got[1].RelatedPaths)
}

func TestReconcileEvidenceFocusesDropsEmptyFocusWithoutInventingFallback(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{
			ID:         "invalid",
			Name:       "Invalid",
			EntryPaths: []string{"outside"},
		},
		{
			ID:         "valid",
			Name:       "Valid",
			EntryPaths: []string{"scope/entry"},
		},
	}

	got := reconcileEvidenceFocuses(focuses, []string{"scope/entry", "scope/unassigned"})

	require.Len(t, got, 1)
	require.Equal(t, "valid", got[0].ID)
	require.Equal(t, []string{"scope/unassigned"}, uncoveredAnalysisPaths(got, []string{"scope/entry", "scope/unassigned"}))
}

func TestCommandStatePreservesCommittedArtifactPhase(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{{Path: "main.go", Hash: "hash"}}, nil, []domain.EvidenceFocus{{ID: "all", EntryPaths: []string{"main.go"}}})
	state.MarkPatternsCommitted(commandstate.PatternCommitSummary{Found: 2, Saved: 1, Retired: 1})
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	require.NoError(t, repo.Save(context.Background(), state))

	loaded, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.True(t, loaded.PatternsCommitComplete())
	require.Equal(t, commandstate.PatternCommitSummary{Found: 2, Saved: 1, Retired: 1}, loaded.CommittedPatternSummary())
}

func TestCommandStatePreservesAnalysisCheckpoint(t *testing.T) {
	pattern := domain.NewPattern("checkpoint", "Checkpoint", domain.CategoryBusiness)
	unit := domain.EvidenceFocus{ID: "auth", Name: "Auth", EntryPaths: []string{"internal/auth.go"}}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{{Path: "internal/auth.go", Hash: "hash"}}, nil, []domain.EvidenceFocus{unit})
	state.Analysis = &commandstate.AnalysisCheckpoint{
		FocusKnowledge: []commandstate.FocusKnowledgeCheckpoint{{
			Focus:             unit,
			Patterns:          []domain.Pattern{*pattern},
			RetiredPatternIDs: []string{"legacy-pattern"},
			Reviewed:          true,
		}},
		ProfileRefreshNeeded: true,
		ProfileRefreshReason: "module boundary changed",
	}
	state.MarkProjectionsCommitted()
	state.Decision = &commandstate.DecisionCheckpoint{
		CandidateHash: "candidate-hash",
		Decision:      json.RawMessage(`{"patterns":[],"dropped":[]}`),
	}
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	require.NoError(t, repo.Save(context.Background(), state))

	loaded, err := repo.Load(context.Background())

	require.NoError(t, err)
	require.NotNil(t, loaded.Analysis)
	require.Len(t, loaded.Analysis.FocusKnowledge, 1)
	require.Equal(t, unit, loaded.Analysis.FocusKnowledge[0].Focus)
	require.Equal(t, "checkpoint", loaded.Analysis.FocusKnowledge[0].Patterns[0].ID)
	require.Equal(t, []string{"legacy-pattern"}, loaded.Analysis.FocusKnowledge[0].RetiredPatternIDs)
	require.True(t, loaded.Analysis.FocusKnowledge[0].Reviewed)
	require.True(t, loaded.Analysis.ProfileRefreshNeeded)
	require.Equal(t, "module boundary changed", loaded.Analysis.ProfileRefreshReason)
	require.True(t, loaded.ProjectionsCommitComplete())
	require.Equal(t, state.Decision.CandidateHash, loaded.Decision.CandidateHash)
	require.JSONEq(t, string(state.Decision.Decision), string(loaded.Decision.Decision))
}

func TestPendingEvidenceFocusesDerivesCompletionFromFocusKnowledge(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{ID: "auth", Name: "Auth", EntryPaths: []string{"internal/auth.go"}},
		{ID: "key", Name: "Key", EntryPaths: []string{"internal/key.go"}},
	}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", nil, nil, focuses)
	state.Analysis = &commandstate.AnalysisCheckpoint{FocusKnowledge: []commandstate.FocusKnowledgeCheckpoint{{Focus: focuses[0]}}}
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{
		{Path: "internal/auth.go"},
		{Path: "internal/key.go"},
	}}

	pending := pendingEvidenceFocuses(state, changes)
	require.Len(t, pending, 1)
	require.Equal(t, "key", pending[0].ID)
}

func TestCheckpointFocusResultsMakesCompletedFocusRecoverable(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{ID: "first", Name: "First", EntryPaths: []string{"internal/first.go"}},
		{ID: "second", Name: "Second", EntryPaths: []string{"internal/second.go"}},
	}
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{
		{Path: "internal/first.go"},
		{Path: "internal/second.go"},
	}}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", changes.Records, nil, focuses)
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	run := &learnCurrentProjectRun{
		ctx:           context.Background(),
		stateRepo:     repo,
		analysisState: state,
	}

	completed, err := run.checkpointFocusResults([]learnCurrentFocusResult{{
		focus:     focuses[0],
		completed: true,
		reviewed:  true,
	}})

	require.NoError(t, err)
	require.Equal(t, 1, completed)
	resumed, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, resumed.Analysis.FocusKnowledge, 1)
	require.Equal(t, "first", resumed.Analysis.FocusKnowledge[0].Focus.ID)
	require.True(t, resumed.Analysis.FocusKnowledge[0].Reviewed)
	pending := pendingEvidenceFocuses(resumed, changes)
	require.Len(t, pending, 1)
	require.Equal(t, "second", pending[0].ID)
}

func TestParallelAnalysisProgressShowsFocusStageAndElapsedTime(t *testing.T) {
	require.NoError(t, i18n.Init("zh-CN"))
	focus := domain.EvidenceFocus{ID: "auth", Name: "认证授权", EntryPaths: []string{"internal/auth.go"}}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", nil, nil, []domain.EvidenceFocus{focus})
	progress := newLearnCurrentParallelAnalysisProgress(&learnCurrentProjectRun{}, "分析", state, []learnCurrentBatch{{
		index:   0,
		focuses: []indexedEvidenceFocus{{index: 0, focus: focus}},
	}}, 1)
	progress.active[0] = learnCurrentParallelFocusStatus{
		base:      "batch-001 焦点 1/1 认证授权",
		stage:     i18n.GetWithParams("LearnCurrentFocusStageKnowledgeReview", map[string]interface{}{"Candidates": 3}),
		startedAt: time.Now().Add(-2 * time.Second),
	}

	lines := progress.activeLines()

	require.Equal(t, 1, len(lines))
	require.Contains(t, lines[0], "• batch-001")
	require.Contains(t, lines[0], "焦点 1/1 认证授权")
	require.Contains(t, lines[0], "独立知识审查 · 候选 3")
	require.Contains(t, lines[0], "(2s)")
}

func TestValidateCompletedAnalysisRequiresEveryPlannedUnit(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{ID: "auth", Name: "Auth", EntryPaths: []string{"internal/shared.go"}},
		{ID: "key", Name: "Key", EntryPaths: []string{"internal/shared.go"}},
	}
	run := &learnCurrentProjectRun{
		analysisState:      commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", nil, nil, focuses),
		incrementalChanges: &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{{Path: "internal/shared.go"}}},
		focusKnowledge:     []commandstate.FocusKnowledgeCheckpoint{{Focus: focuses[0]}},
	}

	err := run.validateCompletedAnalysis()

	require.ErrorContains(t, err, "Key")
}

func TestCompleteAnalysisDoesNotCheckpointIncompletePlan(t *testing.T) {
	unit := domain.EvidenceFocus{ID: "key", Name: "Key", EntryPaths: []string{"internal/key.go"}}
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	run := &learnCurrentProjectRun{
		ctx:                context.Background(),
		stateRepo:          repo,
		analysisState:      commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", nil, nil, []domain.EvidenceFocus{unit}),
		incrementalChanges: &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{{Path: "internal/key.go"}}},
	}

	err := run.completeAnalysis()

	require.ErrorContains(t, err, "Key")
	require.NoFileExists(t, repo.Path())
}

func TestCurrentStateInputsMatchProjectDetectsChangedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	original := []byte("package main\n")
	require.NoError(t, os.WriteFile(path, original, 0o644))
	sum := md5.Sum(original)
	files := []domain.FileAnalysisRecord{{Path: "main.go", Hash: hex.EncodeToString(sum[:])}}

	require.True(t, currentStateInputsMatchProject(root, files, nil))
	require.NoError(t, os.WriteFile(path, []byte("package changed\n"), 0o644))
	require.False(t, currentStateInputsMatchProject(root, files, nil))
}

func TestCurrentStateInputsMatchProjectChecksDeletedFiles(t *testing.T) {
	root := t.TempDir()
	deleted := []string{"removed.go"}
	require.True(t, currentStateInputsMatchProject(root, nil, deleted))
	require.NoError(t, os.WriteFile(filepath.Join(root, "removed.go"), []byte("package restored\n"), 0o644))
	require.False(t, currentStateInputsMatchProject(root, nil, deleted))
}

func TestCurrentChangesCoveredByStateRejectsUnplannedInput(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "main-hash"},
	}, nil, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}})
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "main-hash"},
		{Path: "new.go", Hash: "new-hash"},
	}}

	require.False(t, currentChangesCoveredByState(state, changes))
}

func TestCanReuseCurrentStateRequiresExactInputSet(t *testing.T) {
	const invocationHash = "invocation"
	mode := string(config.LearningModeNormal)
	state := commandstate.NewStateWithMode(commandStateLearnCurrent, "demo", "go", mode, "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "main-hash"},
	}, []string{"removed.go"}, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}}).WithInvocationHash(invocationHash)
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{{Path: "main.go", Hash: "main-hash"}}}

	require.False(t, canReuseCurrentState(state, changes, "demo", "go", mode, "", invocationHash))
}

func TestRestoreCurrentStateClearsIncompatibleInvocation(t *testing.T) {
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	state := commandstate.NewStateWithMode(
		commandStateLearnCurrent,
		"demo",
		"go",
		"normal",
		"",
		[]domain.FileAnalysisRecord{{Path: "main.go", Hash: "hash"}},
		nil,
		[]domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}},
	).WithInvocationHash("old-invocation")
	require.NoError(t, repo.Save(context.Background(), state))

	session, err := restoreCurrentState(
		context.Background(),
		repo,
		nil,
		"demo",
		"go",
		"normal",
		"",
		"new-invocation",
	)

	require.NoError(t, err)
	require.Nil(t, session)
	require.NoFileExists(t, repo.Path())
}

func TestRestoreCurrentStateReportsUnsupportedSchemaWithoutDeletingIt(t *testing.T) {
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	require.NoError(t, os.MkdirAll(filepath.Dir(repo.Path()), 0o755))
	require.NoError(t, os.WriteFile(repo.Path(), []byte(`{"schema_version":3,"command":"learn-current"}`), 0o600))

	session, err := restoreCurrentState(context.Background(), repo, nil, "demo", "go", "normal", "", "invocation")

	require.ErrorIs(t, err, commandstate.ErrUnsupportedSchemaVersion)
	require.Nil(t, session)
	require.FileExists(t, repo.Path())
}

func TestLearnCurrentInvocationHashIncludesExecutionOptions(t *testing.T) {
	base := learnCurrentInvocationHash([]string{"internal/auth"}, false)

	require.NotEqual(t, base, learnCurrentInvocationHash([]string{"internal/key"}, false))
	require.NotEqual(t, base, learnCurrentInvocationHash([]string{"internal/auth"}, true))
}

func TestBuildLearnCurrentResumeSummaryUsesStoredInputMetrics(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{
		{Path: "internal/key/create.go", Hash: "key-hash", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
		{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusSelectionSkipped},
	}, nil, []domain.EvidenceFocus{{ID: "key", Name: "Key", EntryPaths: []string{"internal/key/create.go"}}}).
		WithInputSummary(commandstate.InputSummary{
			SourceFiles:         10,
			LocalPlanInputFiles: 8,
			SelectionInputFiles: 8,
			SelectedFiles:       1,
			SkippedFiles:        7,
		})
	session := &currentStateSession{
		State: state,
		Changes: &fileanalysis.FileChanges{
			Records:         []domain.FileAnalysisRecord{{Path: "internal/key/create.go", Hash: "key-hash"}},
			AddedOrModified: []string{"internal/key/create.go"},
		},
	}

	summary := buildLearnCurrentResumeSummary(session)

	require.Equal(t, "10", summary.SourceFiles)
	require.Equal(t, 8, summary.LocalPlanInputs)
	require.Equal(t, "8", summary.SelectionInputs)
	require.Equal(t, "1", summary.SelectedFiles)
	require.Equal(t, 1, summary.PendingAnalyzeFiles)
	require.Equal(t, 1, summary.Focuses)
	require.Equal(t, 1, summary.DevelopmentFocuses)
	require.Equal(t, 0, summary.CoverageFocuses)
}

func TestBuildLearnCurrentResumeSummaryDerivesMissingMetrics(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{
		{Path: "internal/key/create.go", Hash: "key-hash", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
		{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusSelectionSkipped},
	}, []string{"internal/removed.go"}, []domain.EvidenceFocus{{ID: "key", Name: "Key", EntryPaths: []string{"internal/key/create.go"}}})
	session := &currentStateSession{
		State: state,
		Changes: &fileanalysis.FileChanges{
			Records:         []domain.FileAnalysisRecord{{Path: "internal/key/create.go", Hash: "key-hash"}},
			AddedOrModified: []string{"internal/key/create.go"},
			Deleted:         []string{"internal/removed.go"},
		},
	}

	summary := buildLearnCurrentResumeSummary(session)

	require.Equal(t, "-", summary.SourceFiles)
	require.Equal(t, 3, summary.LocalPlanInputs)
	require.Equal(t, "2", summary.SelectionInputs)
	require.Equal(t, "1", summary.SelectedFiles)
	require.Equal(t, 2, summary.PendingAnalyzeFiles)
}

func TestCurrentStateInputSummaryUsesSelectionStages(t *testing.T) {
	changes := &fileanalysis.FileChanges{SourceFileCount: 12}
	selectionPlan := currentFileSelectionPlan{
		Candidates: []string{"a.go", "b.go", "c.go"},
	}
	selectionSummary := fileSelectionSummary{
		Applied:        true,
		CandidateCount: 3,
		SelectedCount:  1,
		SkippedCount:   2,
	}

	summary := currentStateInputSummary(changes, selectionPlan, selectionSummary)

	require.Equal(t, commandstate.InputSummary{
		SourceFiles:         12,
		LocalPlanInputFiles: 1,
		SelectionInputFiles: 3,
		SelectedFiles:       1,
		SkippedFiles:        2,
	}, summary)
}

func TestCurrentStateInputSummaryRecordsSkippedSelection(t *testing.T) {
	changes := &fileanalysis.FileChanges{SourceFileCount: 12}
	selectionPlan := currentFileSelectionPlan{
		Candidates: []string{"a.go", "b.go", "c.go"},
	}
	selectionSummary := fileSelectionSummary{}

	summary := currentStateInputSummary(changes, selectionPlan, selectionSummary)

	require.Equal(t, commandstate.InputSummary{
		SourceFiles:         12,
		LocalPlanInputFiles: 3,
	}, summary)
}

func TestFilterCompletedStateChangesKeepsOnlyUnfinishedInputs(t *testing.T) {
	changes := &fileanalysis.FileChanges{
		Records: []domain.FileAnalysisRecord{
			{Path: "internal/auth/login.go", Hash: "auth-hash"},
			{Path: "internal/key/create.go", Hash: "key-hash"},
			{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusSelectionSkipped},
		},
		AddedOrModified: []string{
			"internal/auth/login.go",
			"internal/key/create.go",
			"internal/types/types.go",
		},
		Deleted: []string{"internal/removed.go"},
	}
	analyzed := []domain.FileAnalysisRecord{
		{Path: "internal/auth/login.go", Hash: "auth-hash", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
		{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusSelectionSkipped},
	}

	filtered := filterCompletedStateChanges(changes, analyzed)

	require.Equal(t, []string{"internal/key/create.go"}, filtered.AddedOrModified)
	require.Equal(t, []string{"internal/removed.go"}, filtered.Deleted)
	require.Equal(t, []domain.FileAnalysisRecord{
		{Path: "internal/key/create.go", Hash: "key-hash"},
	}, filtered.Records)
}

func TestFilterCompletedStateChangesKeepsChangedHashPending(t *testing.T) {
	changes := &fileanalysis.FileChanges{
		Records:         []domain.FileAnalysisRecord{{Path: "internal/key/create.go", Hash: "new-hash"}},
		AddedOrModified: []string{"internal/key/create.go"},
	}
	analyzed := []domain.FileAnalysisRecord{
		{Path: "internal/key/create.go", Hash: "old-hash", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
	}

	filtered := filterCompletedStateChanges(changes, analyzed)

	require.Equal(t, []string{"internal/key/create.go"}, filtered.AddedOrModified)
	require.Len(t, filtered.Records, 1)
}
