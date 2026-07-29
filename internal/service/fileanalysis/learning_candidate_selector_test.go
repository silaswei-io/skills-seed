package fileanalysis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectLearningCandidatesKeepsRequiredPaths(t *testing.T) {
	result := SelectLearningCandidates(LearningCandidateSelectionOptions{
		Candidates:    []string{"internal/auth/readme.go", "internal/auth/types.go"},
		RequiredPaths: []string{"internal/auth/readme.go"},
	})

	require.Equal(t, []string{"internal/auth/readme.go"}, result.SelectedPaths)
	require.Equal(t, []string{"internal/auth/types.go"}, result.SkippedPaths)
}

func TestSelectLearningCandidatesUsesChangedPathsWhenDiffIsNarrow(t *testing.T) {
	result := SelectLearningCandidates(LearningCandidateSelectionOptions{
		Candidates: []string{"internal/auth/service.go", "internal/auth/types.go", "internal/auth/readme.go"},
		Changes: &FileChanges{
			AddedOrModified: []string{"internal/auth/types.go"},
		},
	})

	require.Equal(t, []string{"internal/auth/service.go", "internal/auth/types.go"}, result.SelectedPaths)
	require.Equal(t, []string{"internal/auth/readme.go"}, result.SkippedPaths)
}

func TestSelectLearningCandidatesDoesNotTreatWholeInitialScanAsChangedSignal(t *testing.T) {
	paths := []string{"internal/auth/service.go", "internal/auth/types.go", "internal/auth/readme.go"}

	result := SelectLearningCandidates(LearningCandidateSelectionOptions{
		Candidates: paths,
		Changes: &FileChanges{
			AddedOrModified: paths,
		},
	})

	require.Equal(t, []string{"internal/auth/service.go"}, result.SelectedPaths)
	require.Equal(t, []string{"internal/auth/readme.go", "internal/auth/types.go"}, result.SkippedPaths)
}

func TestSelectLearningCandidatesKeepsBusinessEntryVocabulary(t *testing.T) {
	result := SelectLearningCandidates(LearningCandidateSelectionOptions{
		Candidates: []string{
			"plugins/cert/internal/action/renew.go",
			"plugins/license/README.md",
			"plugins/license/internal/processor/check.go",
			"plugins/report/internal/readme.go",
		},
	})

	require.Equal(t, []string{
		"plugins/cert/internal/action/renew.go",
		"plugins/license/internal/processor/check.go",
	}, result.SelectedPaths)
	require.Equal(t, []string{"plugins/license/README.md", "plugins/report/internal/readme.go"}, result.SkippedPaths)
}

func TestSelectLearningCandidatesFallsBackToAllWhenNoSignalExists(t *testing.T) {
	result := SelectLearningCandidates(LearningCandidateSelectionOptions{
		Candidates: []string{"a.go", "b.go"},
	})

	require.Equal(t, []string{"a.go", "b.go"}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
}

func TestSelectLearningContextSeedsDoesNotFallbackToAllCandidates(t *testing.T) {
	seeds := SelectLearningContextSeeds(LearningCandidateSelectionOptions{
		Candidates: []string{"a.go", "b.go"},
	})

	require.Empty(t, seeds)
}

func TestSelectLearningContextSeedsUsesRequiredChangedAndHighSignalPaths(t *testing.T) {
	seeds := SelectLearningContextSeeds(LearningCandidateSelectionOptions{
		Candidates: []string{
			"internal/auth/readme.go",
			"internal/auth/service.go",
			"internal/auth/types.go",
			"internal/job/sync.go",
		},
		RequiredPaths: []string{"internal/auth/readme.go"},
		Changes: &FileChanges{
			AddedOrModified: []string{"internal/auth/types.go"},
		},
	})

	require.Equal(t, []string{
		"internal/auth/readme.go",
		"internal/auth/service.go",
		"internal/auth/types.go",
		"internal/job/sync.go",
	}, seeds)
}

func TestSelectLearningContextSeedsDoesNotTreatWholeInitialScanAsChangedSignal(t *testing.T) {
	paths := []string{"internal/auth/service.go", "internal/auth/types.go", "internal/auth/readme.go"}

	seeds := SelectLearningContextSeeds(LearningCandidateSelectionOptions{
		Candidates: paths,
		Changes: &FileChanges{
			AddedOrModified: paths,
		},
	})

	require.Equal(t, []string{"internal/auth/service.go"}, seeds)
}
