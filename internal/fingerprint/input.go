package fingerprint

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

const SourceInputDigest = "input_digest"

type Decision struct {
	record domain.FileAnalysisRecord
	skip   bool
}

func Prepare(ctx context.Context, tracker domain.FileAnalysisTracker, scope domain.FileAnalysisScope, path string, input any) (*Decision, error) {
	if tracker == nil {
		return nil, nil
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal fingerprint input: %w", err)
	}
	sum := md5.Sum(payload)
	hash := hex.EncodeToString(sum[:])
	now := time.Now().Format(time.RFC3339)
	record := domain.FileAnalysisRecord{
		ProjectID:      scope.ProjectID,
		ScopePath:      scope.ScopePath,
		Path:           path,
		Hash:           hash,
		HashAlgorithm:  domain.FileAnalysisHashMD5,
		Size:           int64(len(payload)),
		ModTime:        now,
		Source:         SourceInputDigest,
		LastAnalyzedAt: now,
	}
	previous, err := tracker.GetAnalyzedFile(ctx, scope, path)
	if err != nil {
		return nil, err
	}
	return &Decision{
		record: record,
		skip:   previous != nil && previous.Hash == hash && previous.HashAlgorithm == domain.FileAnalysisHashMD5,
	}, nil
}

func (d *Decision) ShouldSkip() bool {
	return d != nil && d.skip
}

func (d *Decision) Commit(ctx context.Context, tracker domain.FileAnalysisTracker) error {
	if d == nil || tracker == nil {
		return nil
	}
	return tracker.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{d.record})
}
