package learncurrent

import (
	"context"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestCommitCurrentAnalysisSavesFingerprintWithoutSourceSnapshot(t *testing.T) {
	saved := false
	run := &learnCurrentProjectRun{
		learnDeps: learnDeps{
			cont: &container.Container{FileTracker: &mocks.MockFileAnalysisTracker{
				SaveAnalyzedFilesFn: func(ctx context.Context, records []domain.FileAnalysisRecord) error {
					saved = true
					return nil
				},
			}},
		},
		learnChangeCtx: learnChangeCtx{
			incrementalChanges: &fileanalysis.FileChanges{
				Records: []domain.FileAnalysisRecord{{Path: "internal/app.go"}},
			},
		},
	}

	err := run.commitCurrentAnalysis(context.Background())

	require.NoError(t, err)
	require.True(t, saved)
}
