package domain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareInputFingerprintSkipsUnchangedInput(t *testing.T) {
	ctx := context.Background()
	scope := FileAnalysisScope{ProjectID: "backend", ScopePath: "backend"}
	tracker := &fingerprintTracker{}

	first, err := PrepareInputFingerprint(ctx, tracker, scope, "workspace.json", map[string]string{"name": "demo"})
	require.NoError(t, err)
	require.False(t, first.ShouldSkip())
	require.NoError(t, first.Commit(ctx, tracker))
	require.Len(t, tracker.saved, 1)
	require.Equal(t, FileAnalysisSourceInputDigest, tracker.saved[0].Source)

	second, err := PrepareInputFingerprint(ctx, tracker, scope, "workspace.json", map[string]string{"name": "demo"})
	require.NoError(t, err)
	require.True(t, second.ShouldSkip())

	third, err := PrepareInputFingerprint(ctx, tracker, scope, "workspace.json", map[string]string{"name": "changed"})
	require.NoError(t, err)
	require.False(t, third.ShouldSkip())
}

type fingerprintTracker struct {
	records map[string]FileAnalysisRecord
	saved   []FileAnalysisRecord
}

func (t *fingerprintTracker) GetAnalyzedFile(ctx context.Context, scope FileAnalysisScope, path string) (*FileAnalysisRecord, error) {
	record, ok := t.records[path]
	if !ok {
		return nil, nil
	}
	return &record, nil
}

func (t *fingerprintTracker) ListAnalyzedFiles(ctx context.Context, scope FileAnalysisScope) ([]FileAnalysisRecord, error) {
	return nil, nil
}

func (t *fingerprintTracker) SaveAnalyzedFiles(ctx context.Context, records []FileAnalysisRecord) error {
	if t.records == nil {
		t.records = map[string]FileAnalysisRecord{}
	}
	for _, record := range records {
		t.records[record.Path] = record
		t.saved = append(t.saved, record)
	}
	return nil
}

func (t *fingerprintTracker) DeleteAnalyzedFiles(ctx context.Context, scope FileAnalysisScope, paths []string) error {
	return nil
}
