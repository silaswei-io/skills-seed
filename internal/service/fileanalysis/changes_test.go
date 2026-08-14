package fileanalysis

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCommitCurrentChangesSkipsEmptyStoreOperations(t *testing.T) {
	saveCalls := 0
	deleteCalls := 0
	tracker := &mocks.MockFileAnalysisTracker{
		SaveAnalyzedFilesFn: func(ctx context.Context, records []domain.FileAnalysisRecord) error {
			saveCalls++
			return nil
		},
		DeleteAnalyzedFilesFn: func(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
			deleteCalls++
			return nil
		},
	}

	require.NoError(t, CommitCurrentChanges(context.Background(), tracker, &FileChanges{}))
	require.Zero(t, saveCalls)
	require.Zero(t, deleteCalls)
}

func TestCommitCurrentChangesRunsOnlyPresentOperations(t *testing.T) {
	tests := []struct {
		name        string
		changes     FileChanges
		wantSaves   int
		wantDeletes int
	}{
		{
			name:      "records only",
			changes:   FileChanges{Records: []domain.FileAnalysisRecord{{Path: "main.go"}}},
			wantSaves: 1,
		},
		{
			name:        "deletions only",
			changes:     FileChanges{Deleted: []string{"obsolete.go"}},
			wantDeletes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveCalls := 0
			deleteCalls := 0
			tracker := &mocks.MockFileAnalysisTracker{
				SaveAnalyzedFilesFn: func(ctx context.Context, records []domain.FileAnalysisRecord) error {
					saveCalls++
					return nil
				},
				DeleteAnalyzedFilesFn: func(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
					deleteCalls++
					return nil
				},
			}

			require.NoError(t, CommitCurrentChanges(context.Background(), tracker, &tt.changes))
			require.Equal(t, tt.wantSaves, saveCalls)
			require.Equal(t, tt.wantDeletes, deleteCalls)
		})
	}
}
