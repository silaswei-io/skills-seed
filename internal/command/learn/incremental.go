package learn

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
)

type incrementalFileChanges = fileanalysis.FileChanges

func prepareIncrementalFileChanges(ctx context.Context, tracker domain.FileAnalysisTracker, configRepo config.Reader, repoRoot string, scanRoot string, scope domain.FileAnalysisScope, focusAbsPaths []string) (*incrementalFileChanges, error) {
	return fileanalysis.PrepareCurrentChanges(ctx, tracker, configRepo, repoRoot, scanRoot, scope, focusAbsPaths)
}

func commitIncrementalFileChanges(ctx context.Context, tracker domain.FileAnalysisTracker, changes *incrementalFileChanges) error {
	return fileanalysis.CommitCurrentChanges(ctx, tracker, changes)
}
