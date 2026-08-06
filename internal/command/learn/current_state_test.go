package learn

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
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

func TestReconcileEvidenceFocusesFiltersInvalidPathsAndCoversEveryCandidate(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{
			ID:           "auth",
			Name:         "Auth",
			EntryPaths:   []string{"internal/auth/login.go", "internal/auth", "outside.go", "../escape.go"},
			RelatedPaths: []string{"internal/auth/login.go", "internal/auth/types.go", "/tmp/escape.go"},
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
			ID:           "auth",
			Name:         "Auth",
			EntryPaths:   []string{"internal/auth/login.go"},
			RelatedPaths: []string{"internal/auth/types.go"},
		},
		fallbackEvidenceFocus([]string{"internal/key/create.go"}),
	}, got)
	require.Empty(t, uncoveredAnalysisPaths(got, allowed))
}

func TestReconcileEvidenceFocusesUsesUniqueFallbackID(t *testing.T) {
	focuses := []domain.EvidenceFocus{{
		ID:         "current-codebase",
		Name:       "Existing",
		EntryPaths: []string{"main.go"},
	}}

	got := reconcileEvidenceFocuses(focuses, []string{"main.go", "other.go"})

	require.Len(t, got, 2)
	require.Equal(t, "current-codebase-2", got[1].ID)
	require.Equal(t, []string{"other.go"}, got[1].EntryPaths)
}

func TestReconcileEvidenceFocusesMergesLowDensitySupportFocus(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{
			ID:           "application-entry-startup-framework",
			Name:         "应用启动入口与框架集成",
			EntryPaths:   []string{"cmd/server.go", "internal/handler/routes.go"},
			RelatedPaths: []string{"etc/etc.yaml"},
		},
		{
			ID:         "version-api",
			Name:       "版本API",
			RouteTerms: []string{"版本"},
			EntryPaths: []string{"desc/api/version.api", "internal/handler/version.go", "internal/logic/version.go"},
		},
	}

	got := reconcileEvidenceFocuses(focuses, []string{
		"cmd/server.go",
		"internal/handler/routes.go",
		"etc/etc.yaml",
		"desc/api/version.api",
		"internal/handler/version.go",
		"internal/logic/version.go",
	})

	require.Len(t, got, 1)
	require.Equal(t, "application-entry-startup-framework", got[0].ID)
	require.Contains(t, got[0].RelatedPaths, "desc/api/version.api")
	require.Contains(t, got[0].RelatedPaths, "internal/handler/version.go")
	require.Contains(t, got[0].RelatedPaths, "internal/logic/version.go")
}

func TestReconcileEvidenceFocusesGroupsMultipleFallbacksBySemanticPath(t *testing.T) {
	got := reconcileEvidenceFocuses(nil, []string{
		"internal/logic/system/admin/login.go",
		"internal/logic/system/admin/logout.go",
		"internal/logic/user/group/create.go",
		"plugins/ca_manage/internal/logic/ca/addcacert.go",
		"plugins/ca_manage/internal/logic/ca/deletecacert.go",
	})

	require.Len(t, got, 3)
	require.Equal(t, "current-codebase-internal-logic-system", got[0].ID)
	require.Equal(t, []string{"internal/logic/system/admin/login.go", "internal/logic/system/admin/logout.go"}, got[0].EntryPaths)
	require.Equal(t, "current-codebase-internal-logic-user", got[1].ID)
	require.Equal(t, []string{"internal/logic/user/group/create.go"}, got[1].EntryPaths)
	require.Equal(t, "current-codebase-plugins-ca-manage-internal-logic-ca", got[2].ID)
	require.Equal(t, []string{"plugins/ca_manage/internal/logic/ca/addcacert.go", "plugins/ca_manage/internal/logic/ca/deletecacert.go"}, got[2].EntryPaths)
}

func TestReconcileEvidenceFocusesKeepsShallowFallbackTogether(t *testing.T) {
	got := reconcileEvidenceFocuses(nil, []string{
		"main.go",
		"internal/logic/create.go",
		"internal/types/types.go",
	})

	require.Len(t, got, 1)
	require.Equal(t, "current-codebase", got[0].ID)
	require.Equal(t, []string{"internal/logic/create.go", "internal/types/types.go", "main.go"}, got[0].EntryPaths)
}

func TestCommandStatePreservesCommittedArtifactPhase(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{{Path: "main.go", Hash: "hash"}}, nil, []domain.EvidenceFocus{{ID: "all", EntryPaths: []string{"main.go"}}})
	state.ArtifactsCommitted = true
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	require.NoError(t, repo.Save(context.Background(), state))

	loaded, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.True(t, loaded.ArtifactsCommitted)
}

func TestCommandStatePreservesAnalysisCheckpoint(t *testing.T) {
	pattern := domain.NewPattern("checkpoint", "Checkpoint", domain.CategoryBusiness)
	unit := domain.EvidenceFocus{ID: "auth", Name: "Auth", EntryPaths: []string{"internal/auth.go"}}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []domain.FileAnalysisRecord{{Path: "internal/auth.go", Hash: "hash"}}, nil, []domain.EvidenceFocus{unit})
	state.Analysis = &commandstate.AnalysisCheckpoint{
		Patterns:             []domain.Pattern{*pattern},
		CompletedFocuses:     []domain.EvidenceFocus{unit},
		ProfileRefreshNeeded: true,
		ProfileRefreshReason: "module boundary changed",
	}
	state.ProfileCommitted = true
	state.Decision = &commandstate.DecisionCheckpoint{
		CandidateHash: "candidate-hash",
		Decision:      json.RawMessage(`{"patterns":[],"dropped":[]}`),
	}
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	require.NoError(t, repo.Save(context.Background(), state))

	loaded, err := repo.Load(context.Background())

	require.NoError(t, err)
	require.NotNil(t, loaded.Analysis)
	require.Len(t, loaded.Analysis.Patterns, 1)
	require.Equal(t, "checkpoint", loaded.Analysis.Patterns[0].ID)
	require.Equal(t, []domain.EvidenceFocus{unit}, loaded.Analysis.CompletedFocuses)
	require.True(t, loaded.Analysis.ProfileRefreshNeeded)
	require.Equal(t, "module boundary changed", loaded.Analysis.ProfileRefreshReason)
	require.True(t, loaded.ProfileCommitted)
	require.Equal(t, state.Decision.CandidateHash, loaded.Decision.CandidateHash)
	require.JSONEq(t, string(state.Decision.Decision), string(loaded.Decision.Decision))
}

func TestPendingEvidenceFocusesDerivesCompletionFromCompletedFocuses(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{ID: "auth", Name: "Auth", EntryPaths: []string{"internal/auth.go"}},
		{ID: "key", Name: "Key", EntryPaths: []string{"internal/key.go"}},
	}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", nil, nil, focuses)
	state.Analysis = &commandstate.AnalysisCheckpoint{CompletedFocuses: focuses[:1]}
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{
		{Path: "internal/auth.go"},
		{Path: "internal/key.go"},
	}}

	pending := pendingEvidenceFocuses(state, changes)
	require.Len(t, pending, 1)
	require.Equal(t, "key", pending[0].ID)
}

func TestValidateCompletedAnalysisRequiresEveryPlannedUnit(t *testing.T) {
	focuses := []domain.EvidenceFocus{
		{ID: "auth", Name: "Auth", EntryPaths: []string{"internal/shared.go"}},
		{ID: "key", Name: "Key", EntryPaths: []string{"internal/shared.go"}},
	}
	run := &learnCurrentProjectRun{
		analysisState:            commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", nil, nil, focuses),
		incrementalChanges:       &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{{Path: "internal/shared.go"}}},
		completedEvidenceFocuses: focuses[:1],
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
	mode := learnCurrentStateMode(string(config.LearningModeNormal), string(config.LearningScopeFlow))
	state := commandstate.NewStateWithMode(commandStateLearnCurrent, "demo", "go", mode, "", []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "main-hash"},
	}, []string{"removed.go"}, []domain.EvidenceFocus{{ID: "main", EntryPaths: []string{"main.go"}}}).WithInvocationHash(invocationHash)
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{{Path: "main.go", Hash: "main-hash"}}}

	require.False(t, canReuseCurrentState(state, changes, "demo", "go", mode, "", invocationHash))
}

func TestRestoreCurrentStateClearsIncompatibleCheckpoint(t *testing.T) {
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	state := commandstate.NewStateWithMode(
		commandStateLearnCurrent,
		"demo",
		"go",
		"normal|scope=flow",
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
		"normal|scope=flow",
		"",
		"new-invocation",
	)

	require.NoError(t, err)
	require.Nil(t, session)
	_, err = repo.Load(context.Background())
	require.ErrorIs(t, err, commandstate.ErrStateNotFound)
}

func TestLearnCurrentInvocationHashIncludesExecutionOptions(t *testing.T) {
	base := learnCurrentInvocationHash(nil, []string{"internal/auth"}, learnCurrentProfileAuto, false)

	require.NotEqual(t, base, learnCurrentInvocationHash(nil, []string{"internal/key"}, learnCurrentProfileAuto, false))
	require.NotEqual(t, base, learnCurrentInvocationHash(nil, []string{"internal/auth"}, learnCurrentProfileRefresh, false))
	require.NotEqual(t, base, learnCurrentInvocationHash(nil, []string{"internal/auth"}, learnCurrentProfileAuto, true))
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
