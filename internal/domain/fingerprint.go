package domain

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const FileAnalysisSourceInputDigest = "input_digest"

type InputFingerprintDecision struct {
	record FileAnalysisRecord
	skip   bool
}

func PrepareInputFingerprint(ctx context.Context, tracker FileAnalysisTracker, scope FileAnalysisScope, path string, input any) (*InputFingerprintDecision, error) {
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
	record := FileAnalysisRecord{
		ProjectID:      scope.ProjectID,
		ScopePath:      scope.ScopePath,
		Path:           path,
		Hash:           hash,
		HashAlgorithm:  FileAnalysisHashMD5,
		Size:           int64(len(payload)),
		ModTime:        now,
		Source:         FileAnalysisSourceInputDigest,
		LastAnalyzedAt: now,
	}
	previous, err := tracker.GetAnalyzedFile(ctx, scope, path)
	if err != nil {
		return nil, err
	}
	return &InputFingerprintDecision{
		record: record,
		skip:   previous != nil && previous.Hash == hash && previous.HashAlgorithm == FileAnalysisHashMD5,
	}, nil
}

func (d *InputFingerprintDecision) ShouldSkip() bool {
	return d != nil && d.skip
}

func (d *InputFingerprintDecision) Commit(ctx context.Context, tracker FileAnalysisTracker) error {
	if d == nil || tracker == nil {
		return nil
	}
	return tracker.SaveAnalyzedFiles(ctx, []FileAnalysisRecord{d.record})
}
