package learn

import (
	"strconv"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/stretchr/testify/require"
)

func TestBuildStateInputsPreservesAISelectionState(t *testing.T) {
	changes := &incrementalFileChanges{
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
		Changes: &incrementalFileChanges{
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
		Changes: &incrementalFileChanges{
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
	changes := &incrementalFileChanges{SourceFileCount: 12}
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
	changes := &incrementalFileChanges{SourceFileCount: 12}
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
	changes := &incrementalFileChanges{
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
	changes := &incrementalFileChanges{
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
