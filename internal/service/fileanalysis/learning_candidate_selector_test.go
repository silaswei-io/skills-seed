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

	require.Equal(t, []string{"internal/auth/readme.go", "internal/auth/types.go"}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
}

func TestSelectLearningCandidatesKeepsAllCandidatesWhenDiffIsNarrow(t *testing.T) {
	result := SelectLearningCandidates(LearningCandidateSelectionOptions{
		Candidates: []string{"internal/auth/service.go", "internal/auth/types.go", "internal/auth/readme.go"},
		Changes: &FileChanges{
			AddedOrModified: []string{"internal/auth/types.go"},
		},
	})

	require.Equal(t, []string{"internal/auth/readme.go", "internal/auth/service.go", "internal/auth/types.go"}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
}

func TestSelectLearningCandidatesKeepsAllInitialScanCandidates(t *testing.T) {
	paths := []string{"internal/auth/service.go", "internal/auth/types.go", "internal/auth/readme.go"}

	result := SelectLearningCandidates(LearningCandidateSelectionOptions{
		Candidates: paths,
		Changes: &FileChanges{
			AddedOrModified: paths,
		},
	})

	require.Equal(t, []string{"internal/auth/readme.go", "internal/auth/service.go", "internal/auth/types.go"}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
}

func TestSelectLearningCandidatesDoesNotUseVocabularyAsDropSignal(t *testing.T) {
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
		"plugins/license/README.md",
		"plugins/license/internal/processor/check.go",
		"plugins/report/internal/readme.go",
	}, result.SelectedPaths)
	require.Empty(t, result.SkippedPaths)
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

	require.Equal(t, []string{"internal/auth/service.go", "internal/auth/types.go"}, seeds)
}

func TestLearningCandidatePathScoreScoresEntryNamesWithoutLanguageSuffix(t *testing.T) {
	require.Positive(t, learningCandidatePathScore("src/bootstrap.rs"))
	require.Positive(t, learningCandidatePathScore("app/index.py"))
	require.Positive(t, learningCandidatePathScore("server/main"))
}
