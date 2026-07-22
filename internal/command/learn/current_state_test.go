package learn

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/stretchr/testify/require"
)

func TestBuildStateInputsPreservesAISelectionState(t *testing.T) {
	changes := &fileanalysis.FileChanges{
		Records: []domain.FileAnalysisRecord{
			{Path: "internal/key/create.go", Hash: "key-hash", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
			{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusAISkipped},
		},
		Deleted: []string{"internal/removed.go"},
	}

	inputs := buildStateInputs(changes)

	require.Equal(t, []commandstate.FileInput{
		{Path: "internal/key/create.go", Hash: "key-hash", Status: "present"},
		{Path: "internal/removed.go", Status: "deleted"},
		{Path: "internal/types/types.go", Hash: "types-hash", Status: domain.FileAnalysisStatusAISkipped},
	}, inputs)
}

func TestCommandStatePreservesCommittedArtifactPhase(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []commandstate.FileInput{{Path: "main.go", Hash: "hash", Status: "present"}}, []domain.AnalysisUnit{{ID: "all", EntryPaths: []string{"main.go"}}})
	state.ArtifactsCommitted = true
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	require.NoError(t, repo.Save(context.Background(), state))

	loaded, err := repo.Load(context.Background())
	require.NoError(t, err)
	require.True(t, loaded.ArtifactsCommitted)
}

func TestCommandStatePreservesAnalysisCheckpoint(t *testing.T) {
	pattern := domain.NewPattern("checkpoint", "Checkpoint", domain.CategoryBusiness)
	unit := domain.AnalysisUnit{ID: "auth", Name: "Auth", EntryPaths: []string{"internal/auth.go"}}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []commandstate.FileInput{{Path: "internal/auth.go", Hash: "hash", Status: "present"}}, []domain.AnalysisUnit{unit})
	state.Analysis = &commandstate.AnalysisCheckpoint{
		Complete:             true,
		Patterns:             []domain.Pattern{*pattern},
		CompletedUnits:       []domain.AnalysisUnit{unit},
		ProfileRefreshNeeded: true,
		ProfileRefreshReason: "module boundary changed",
	}
	state.ProfileCommitted = true
	repo := commandstate.NewRepository(t.TempDir(), commandStateLearnCurrent)
	require.NoError(t, repo.Save(context.Background(), state))

	loaded, err := repo.Load(context.Background())

	require.NoError(t, err)
	require.NotNil(t, loaded.Analysis)
	require.True(t, loaded.Analysis.Complete)
	require.Len(t, loaded.Analysis.Patterns, 1)
	require.Equal(t, "checkpoint", loaded.Analysis.Patterns[0].ID)
	require.Equal(t, []domain.AnalysisUnit{unit}, loaded.Analysis.CompletedUnits)
	require.True(t, loaded.Analysis.ProfileRefreshNeeded)
	require.Equal(t, "module boundary changed", loaded.Analysis.ProfileRefreshReason)
	require.True(t, loaded.ProfileCommitted)
}

func TestCurrentStateInputsMatchProjectDetectsChangedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	original := []byte("package main\n")
	require.NoError(t, os.WriteFile(path, original, 0o644))
	sum := md5.Sum(original)
	inputs := []commandstate.FileInput{{Path: "main.go", Hash: hex.EncodeToString(sum[:]), Status: "present"}}

	require.True(t, currentStateInputsMatchProject(root, inputs))
	require.NoError(t, os.WriteFile(path, []byte("package changed\n"), 0o644))
	require.False(t, currentStateInputsMatchProject(root, inputs))
}

func TestCurrentStateInputsMatchProjectChecksDeletedFiles(t *testing.T) {
	root := t.TempDir()
	inputs := []commandstate.FileInput{{Path: "removed.go", Status: "deleted"}}
	require.True(t, currentStateInputsMatchProject(root, inputs))
	require.NoError(t, os.WriteFile(filepath.Join(root, "removed.go"), []byte("package restored\n"), 0o644))
	require.False(t, currentStateInputsMatchProject(root, inputs))
}

func TestCurrentChangesCoveredByStateRejectsUnplannedInput(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []commandstate.FileInput{
		{Path: "main.go", Hash: "main-hash", Status: "present"},
	}, []domain.AnalysisUnit{{ID: "main", EntryPaths: []string{"main.go"}}})
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "main-hash"},
		{Path: "new.go", Hash: "new-hash"},
	}}

	require.False(t, currentChangesCoveredByState(state, changes))
}

func TestCanReuseCurrentStateRequiresExactInputSet(t *testing.T) {
	const invocationHash = "invocation"
	mode := learnCurrentStateMode(string(config.LearningModeNormal), string(config.LearningScopeFlow))
	state := commandstate.NewStateWithMode(commandStateLearnCurrent, "demo", "go", mode, "", []commandstate.FileInput{
		{Path: "main.go", Hash: "main-hash", Status: "present"},
		{Path: "removed.go", Status: "deleted"},
	}, []domain.AnalysisUnit{{ID: "main", EntryPaths: []string{"main.go"}}}).WithInvocationHash(invocationHash)
	changes := &fileanalysis.FileChanges{Records: []domain.FileAnalysisRecord{{Path: "main.go", Hash: "main-hash"}}}

	require.False(t, canReuseCurrentState(state, changes, "demo", "go", mode, "", invocationHash))
}

func TestLearnCurrentInvocationHashIncludesExecutionOptions(t *testing.T) {
	base := learnCurrentInvocationHash(nil, []string{"internal/auth"}, learnCurrentProfileAuto, false)

	require.NotEqual(t, base, learnCurrentInvocationHash(nil, []string{"internal/key"}, learnCurrentProfileAuto, false))
	require.NotEqual(t, base, learnCurrentInvocationHash(nil, []string{"internal/auth"}, learnCurrentProfileRefresh, false))
	require.NotEqual(t, base, learnCurrentInvocationHash(nil, []string{"internal/auth"}, learnCurrentProfileAuto, true))
}

func TestBuildLearnCurrentResumeSummaryUsesStoredInputMetrics(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []commandstate.FileInput{
		{Path: "internal/key/create.go", Hash: "key-hash", Status: "present"},
		{Path: "internal/types/types.go", Hash: "types-hash", Status: domain.FileAnalysisStatusAISkipped},
	}, []domain.AnalysisUnit{{ID: "key", Name: "Key", EntryPaths: []string{"internal/key/create.go"}}}).
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
	require.Equal(t, "8", summary.AISelectionInputs)
	require.Equal(t, "1", summary.AISelectedFiles)
	require.Equal(t, 1, summary.PendingAnalyzeFiles)
	require.Equal(t, 1, summary.Units)
}

func TestBuildLearnCurrentResumeSummaryFallsBackForLegacyState(t *testing.T) {
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", []commandstate.FileInput{
		{Path: "internal/key/create.go", Hash: "key-hash", Status: "present"},
		{Path: "internal/types/types.go", Hash: "types-hash", Status: domain.FileAnalysisStatusAISkipped},
		{Path: "internal/removed.go", Status: "deleted"},
	}, []domain.AnalysisUnit{{ID: "key", Name: "Key", EntryPaths: []string{"internal/key/create.go"}}})
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
	require.Equal(t, "2", summary.AISelectionInputs)
	require.Equal(t, "1", summary.AISelectedFiles)
	require.Equal(t, 2, summary.PendingAnalyzeFiles)
}

func TestCurrentStateInputSummaryUsesSelectionStages(t *testing.T) {
	changes := &fileanalysis.FileChanges{SourceFileCount: 12}
	selectionPlan := currentFileSelectionPlan{
		Candidates: []string{"a.go", "b.go", "c.go"},
		Eligible:   true,
	}
	selectionSummary := aiFileSelectionSummary{
		Applied:        true,
		CandidateCount: 3,
		SelectedCount:  1,
		SkippedCount:   2,
	}

	summary := currentStateInputSummary(changes, selectionPlan, selectionSummary)

	require.Equal(t, commandstate.InputSummary{
		SourceFiles:         12,
		LocalPlanInputFiles: 3,
		SelectionInputFiles: 3,
		SelectedFiles:       1,
		SkippedFiles:        2,
	}, summary)
}

func TestCurrentStateInputSummaryRecordsAttemptedSelection(t *testing.T) {
	changes := &fileanalysis.FileChanges{SourceFileCount: 12}
	selectionPlan := currentFileSelectionPlan{
		Candidates: []string{"a.go", "b.go", "c.go"},
		Eligible:   true,
	}
	selectionSummary := aiFileSelectionSummary{
		Attempted:      true,
		CandidateCount: 3,
	}

	summary := currentStateInputSummary(changes, selectionPlan, selectionSummary)

	require.Equal(t, commandstate.InputSummary{
		SourceFiles:         12,
		LocalPlanInputFiles: 3,
		SelectionInputFiles: 3,
	}, summary)
}

func TestFilterCompletedStateChangesKeepsOnlyUnfinishedInputs(t *testing.T) {
	changes := &fileanalysis.FileChanges{
		Records: []domain.FileAnalysisRecord{
			{Path: "internal/auth/login.go", Hash: "auth-hash"},
			{Path: "internal/key/create.go", Hash: "key-hash"},
			{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusAISkipped},
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
		{Path: "internal/types/types.go", Hash: "types-hash", AnalysisStatus: domain.FileAnalysisStatusAISkipped},
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

func TestRunningUnitsProgressStartsFromCompletedOriginalPlan(t *testing.T) {
	units := make([]domain.AnalysisUnit, 0, 19)
	for i := 1; i <= 19; i++ {
		units = append(units, domain.AnalysisUnit{
			ID:         "unit-" + strconv.Itoa(i),
			Name:       "单元 " + strconv.Itoa(i),
			EntryPaths: []string{"internal/unit" + strconv.Itoa(i) + ".go"},
		})
	}
	state := commandstate.NewState(commandStateLearnCurrent, "demo", "go", "", nil, units)
	pending := units[17:]

	running := newLearnCurrentRunningUnits(state, pending)

	params := running.progressParams(2)
	require.Equal(t, 18, params["Current"])
	require.Equal(t, 19, params["Total"])
	running.start(0, "单元 18")
	require.Equal(t, 18, running.progressParams(2)["Current"])
	running.finish(0, true)
	require.Equal(t, 19, running.progressParams(2)["Current"])
}
