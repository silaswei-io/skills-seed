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
