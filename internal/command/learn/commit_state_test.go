package learn

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	snapshotstore "github.com/silaswei-io/skills-seed/internal/infra/storage/snapshot"
	"github.com/silaswei-io/skills-seed/internal/service/analyzer"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/service/snapshotflow"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCommitCurrentAnalysisDoesNotSaveFingerprintWhenSnapshotFails(t *testing.T) {
	saved := false
	run := &learnCurrentProjectRun{
		cont: &container.Container{FileTracker: &mocks.MockFileAnalysisTracker{
			SaveAnalyzedFilesFn: func(ctx context.Context, records []domain.FileAnalysisRecord) error {
				saved = true
				return nil
			},
		}},
		incrementalChanges: &fileanalysis.FileChanges{
			Records: []domain.FileAnalysisRecord{{Path: "../outside.go"}},
		},
		codebaseRunContext: &analyzer.CodebaseRunContext{SnapshotFlow: &snapshotflow.Result{
			CurrentFiles: map[string]string{"../outside.go": "package outside\n"},
			Repository:   snapshotstore.NewRepository(t.TempDir()),
		}},
	}

	err := run.commitCurrentAnalysis(context.Background())

	require.Error(t, err)
	require.False(t, saved)
}
