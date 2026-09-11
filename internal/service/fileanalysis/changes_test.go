package fileanalysis

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
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

func TestPrepareCurrentChangesValidatesDependencies(t *testing.T) {
	_, err := PrepareCurrentChanges(context.Background(), nil, nil, "", t.TempDir(), domain.FileAnalysisScope{}, nil)
	require.EqualError(t, err, "file analysis tracker is nil")

	wantErr := errors.New("list failed")
	tracker := &mocks.MockFileAnalysisTracker{
		ListAnalyzedFilesFn: func(context.Context, domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
			return nil, wantErr
		},
	}
	_, err = PrepareCurrentChanges(context.Background(), tracker, nil, "", t.TempDir(), domain.FileAnalysisScope{}, nil)
	require.ErrorIs(t, err, wantErr)

	_, err = PrepareCurrentChanges(context.Background(), &mocks.MockFileAnalysisTracker{}, nil, "", filepath.Join(t.TempDir(), "missing"), domain.FileAnalysisScope{}, nil)
	require.Error(t, err)
}

func TestPrepareCurrentChangesClassifiesFiles(t *testing.T) {
	root := t.TempDir()
	writeSelectionFile(t, root, "unchanged.go", "package demo\n")
	writeSelectionFile(t, root, "changed.go", "package demo\nvar changed = true\n")
	writeSelectionFile(t, root, "added.go", "package demo\nvar added = true\n")
	writeSelectionFile(t, root, "ignored/old.go", "package ignored\n")
	writeSelectionFile(t, root, "legacy_test.go", "package demo\n")

	scope := domain.FileAnalysisScope{ProjectID: "demo", ScopePath: "src"}
	unchanged, err := fingerprintLearnFile(root, scope, "unchanged.go")
	require.NoError(t, err)
	previous := []domain.FileAnalysisRecord{
		unchanged,
		{ProjectID: "demo", ScopePath: "src", Path: "changed.go", Hash: "old"},
		{ProjectID: "demo", ScopePath: "src", Path: "deleted.go", Hash: "old"},
		{ProjectID: "demo", ScopePath: "src", Path: "ignored/old.go", Hash: "old"},
		{ProjectID: "demo", ScopePath: "src", Path: "legacy_test.go", Hash: "old"},
	}
	tracker := &mocks.MockFileAnalysisTracker{
		ListAnalyzedFilesFn: func(context.Context, domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
			return previous, nil
		},
	}
	configRepo := &mocks.MockConfigReader{Exclude: []string{"ignored/**"}}

	changes, err := PrepareCurrentChanges(context.Background(), tracker, configRepo, root, root, scope, nil)

	require.NoError(t, err)
	require.Equal(t, 5, changes.PreviousAnalyzedCount)
	require.ElementsMatch(t, []string{"added.go", "changed.go"}, changes.AddedOrModified)
	require.Equal(t, []string{"unchanged.go"}, changes.Unchanged)
	require.Equal(t, []string{"deleted.go"}, changes.Deleted)
	require.NotContains(t, changes.Deleted, "ignored/old.go")
	require.NotContains(t, changes.Deleted, "legacy_test.go")
	require.True(t, changes.HasChanges())
	require.Equal(t, []string{"added.go", "changed.go", "deleted.go"}, changes.FocusPaths())
	require.Equal(t, 1, changes.SkippedCount(SkipReasonProcedure))
	require.Zero(t, changes.SkippedCount(SkipReasonExcluded))
	require.Len(t, changes.Records, 2)
}

func TestPrepareCurrentChangesSupportsFocusAndForce(t *testing.T) {
	root := t.TempDir()
	writeSelectionFile(t, root, "focus/main.go", "package focus\n")
	writeSelectionFile(t, root, "other/main.go", "package other\n")
	scope := domain.FileAnalysisScope{ProjectID: "demo"}
	previous, err := fingerprintLearnFile(root, scope, "focus/main.go")
	require.NoError(t, err)
	tracker := &mocks.MockFileAnalysisTracker{
		ListAnalyzedFilesFn: func(context.Context, domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
			return []domain.FileAnalysisRecord{previous, {Path: "other/main.go", Hash: "old"}}, nil
		},
	}

	changes, err := PrepareCurrentChangesWithOptions(
		context.Background(), tracker, nil, root, root, scope,
		[]string{filepath.Join(root, "focus")}, CurrentChangeOptions{Force: true},
	)

	require.NoError(t, err)
	require.Equal(t, 1, changes.PreviousAnalyzedCount)
	require.Equal(t, []string{"focus/main.go"}, changes.AddedOrModified)
	require.Empty(t, changes.Unchanged)
	require.Empty(t, changes.Deleted)
}

func TestFileChangesSelectionHelpers(t *testing.T) {
	changes := &FileChanges{Records: []domain.FileAnalysisRecord{
		{Path: "selected.go", AnalysisStatus: domain.FileAnalysisStatusSelectionSkipped, SelectionReason: "old"},
		{Path: "dir/./skipped.go", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
	}}
	changes.ApplyLearningSelection([]string{"./selected.go"}, "not selected")

	require.Equal(t, domain.FileAnalysisStatusAnalyzed, changes.Records[0].AnalysisStatus)
	require.Empty(t, changes.Records[0].SelectionReason)
	require.Equal(t, domain.FileAnalysisStatusSelectionSkipped, changes.Records[1].AnalysisStatus)
	require.Equal(t, "not selected", changes.Records[1].SelectionReason)

	changes.addSkipped("dir/./file.go", SkipReasonUnreadable)
	require.Contains(t, changes.Skipped, "dir/file.go")
	require.Equal(t, 1, changes.SkippedCount(SkipReasonUnreadable))
	require.False(t, (FileChanges{}).HasChanges())
	require.Empty(t, (FileChanges{}).FocusPaths())

	(*FileChanges)(nil).ApplyLearningSelection(nil, "ignored")
	(*FileChanges)(nil).addSkipped("ignored.go", SkipReasonUnreadable)
}

func TestFingerprintLearnFile(t *testing.T) {
	root := t.TempDir()
	content := []byte("package demo\n")
	writeSelectionFile(t, root, "main.go", string(content))
	scope := domain.FileAnalysisScope{ProjectID: "demo", ScopePath: "backend"}

	record, err := fingerprintLearnFile(root, scope, "main.go")

	require.NoError(t, err)
	sum := md5.Sum(content)
	require.Equal(t, hex.EncodeToString(sum[:]), record.Hash)
	require.Equal(t, domain.FileAnalysisHashMD5, record.HashAlgorithm)
	require.Equal(t, int64(len(content)), record.Size)
	require.Equal(t, scope.ProjectID, record.ProjectID)
	require.Equal(t, scope.ScopePath, record.ScopePath)
	require.Equal(t, domain.FileAnalysisSourceCurrentCode, record.Source)
	require.Equal(t, domain.FileAnalysisStatusAnalyzed, record.AnalysisStatus)
	require.NotEmpty(t, record.ModTime)
	require.NotEmpty(t, record.LastAnalyzedAt)

	_, err = fingerprintLearnFile(root, scope, "missing.go")
	require.Error(t, err)
	outside := filepath.Join(t.TempDir(), "outside.go")
	require.NoError(t, os.WriteFile(outside, content, 0o644))
	_, err = fingerprintLearnFile(root, scope, "../outside.go")
	require.Error(t, err)
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "linked.go")))
	_, err = fingerprintLearnFile(root, scope, "linked.go")
	require.Error(t, err)
}

func TestCommitCurrentChangesPropagatesStoreErrors(t *testing.T) {
	require.NoError(t, CommitCurrentChanges(context.Background(), nil, nil))

	saveErr := errors.New("save failed")
	deleteCalls := 0
	tracker := &mocks.MockFileAnalysisTracker{
		SaveAnalyzedFilesFn: func(context.Context, []domain.FileAnalysisRecord) error { return saveErr },
		DeleteAnalyzedFilesFn: func(context.Context, domain.FileAnalysisScope, []string) error {
			deleteCalls++
			return nil
		},
	}
	err := CommitCurrentChanges(context.Background(), tracker, &FileChanges{
		Records: []domain.FileAnalysisRecord{{Path: "main.go"}},
		Deleted: []string{"old.go"},
	})
	require.ErrorIs(t, err, saveErr)
	require.Zero(t, deleteCalls)

	deleteErr := errors.New("delete failed")
	tracker.SaveAnalyzedFilesFn = func(context.Context, []domain.FileAnalysisRecord) error { return nil }
	tracker.DeleteAnalyzedFilesFn = func(context.Context, domain.FileAnalysisScope, []string) error { return deleteErr }
	err = CommitCurrentChanges(context.Background(), tracker, &FileChanges{
		Records: []domain.FileAnalysisRecord{{Path: "main.go"}},
		Deleted: []string{"old.go"},
	})
	require.ErrorIs(t, err, deleteErr)
}

func TestConfiguredLearnExcludesIncludesGeneratedSkillDirectories(t *testing.T) {
	root := t.TempDir()
	configRepo := &mocks.MockConfigReader{
		Exclude:   []string{"vendor/**"},
		SkillsCfg: config.SkillsConfig{Paths: map[string]string{"codex": ".agents/skills/demo"}},
	}

	require.Contains(t, ConfiguredLearnExcludes(configRepo, root), "vendor/**")
	require.Equal(t, []string{".agents/skills/demo"}, GeneratedSkillExcludeDirs(configRepo, root))
}
