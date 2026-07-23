package learner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLearnerService(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockGit := &mocks.MockGitRepository{}
	mockPattern := &mocks.MockPatternRepository{}
	mockTracker := &mocks.MockCommitTracker{}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	assert.NotNil(t, svc)
}

func TestKnownPatternsSnapshotIncludesEvidenceLocations(t *testing.T) {
	pattern := newLearnerTestPattern("error-wrap", "Error Wrap", domain.CategoryError)
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/config.go", Line: 42, Symbol: "LoadConfig", Kind: "function", Description: "wraps config errors", Confidence: 0.88},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	svc := NewLearnerService(nil, nil, mockPattern, nil, nil)

	snapshot, count := svc.KnownPatternsSnapshot(context.Background())

	require.Equal(t, 1, count)
	var data []map[string]any
	require.NoError(t, json.Unmarshal([]byte(snapshot), &data))
	require.Len(t, data, 1)
	locations, ok := data[0]["evidence_locations"].([]any)
	require.True(t, ok)
	require.Len(t, locations, 1)
	location, ok := locations[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "internal/service/config.go", location["path"])
	require.Equal(t, float64(42), location["line"])
}

func TestLearn_NoCommits(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockGit := &mocks.MockGitRepository{
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return []domain.CommitInfo{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{}
	mockTracker := &mocks.MockCommitTracker{}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.Learn(context.Background(), 10, "", 5)
	assert.NoError(t, err)
}

func TestLearn_GitError(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockGit := &mocks.MockGitRepository{
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return nil, errors.New("git error")
		},
	}
	mockPattern := &mocks.MockPatternRepository{}
	mockTracker := &mocks.MockCommitTracker{}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.Learn(context.Background(), 10, "", 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Git")
}

func TestLearn_AllCommitsAnalyzed(t *testing.T) {
	commits := []domain.CommitInfo{
		{Hash: "abc123", Message: "test commit", Author: "test"},
	}

	mockGit := &mocks.MockGitRepository{
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return commits, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{
		IsAnalyzedFn: func(ctx context.Context, hash string) (bool, error) {
			return true, nil
		},
	}
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.Learn(context.Background(), 10, "", 5)
	assert.NoError(t, err)
}

func TestLearn_Success(t *testing.T) {
	commits := []domain.CommitInfo{
		{Hash: "abc123", Message: "test", Author: "dev"},
		{Hash: "def456", Message: "test2", Author: "dev"},
	}

	savedPatterns := []string{}
	mockGit := &mocks.MockGitRepository{
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return commits, nil
		},
		ChangedFilesFn: func(ctx context.Context, hash string) ([]string, error) {
			return []string{"internal/" + hash + ".go"}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			savedPatterns = append(savedPatterns, p.Name)
			return nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{
		IsAnalyzedFn: func(ctx context.Context, hash string) (bool, error) {
			return false, nil
		},
	}
	markCalled := []string{}
	mockTracker.MarkAnalyzedFn = func(ctx context.Context, hash string) error {
		markCalled = append(markCalled, hash)
		return nil
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		BatchLearnFromCommitsFn: func(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
			require.Len(t, req.Commits, 2)
			require.Len(t, req.CommitFiles, 2)
			assert.Equal(t, "abc123", req.Commits[0].Hash)
			assert.Equal(t, []string{"internal/abc123.go"}, req.CommitFiles[0].Files)
			assert.Equal(t, "def456", req.Commits[1].Hash)
			assert.Equal(t, []string{"internal/def456.go"}, req.CommitFiles[1].Files)
			return &agent.BatchLearnResult{
				Patterns: []domain.Pattern{
					*newLearnerTestPattern("p1", "Error Handling", domain.CategoryError),
					*newLearnerTestPattern("p2", "Naming Convention", domain.CategoryNaming),
				},
				LearnedAt: time.Now(),
			}, nil
		},
	}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.Learn(context.Background(), 10, "", 5)
	assert.NoError(t, err)
	assert.Len(t, savedPatterns, 2)
	assert.Len(t, markCalled, 2)
}

func TestLearn_DoesNotMarkCommitsWhenPatternSaveFails(t *testing.T) {
	commits := []domain.CommitInfo{{Hash: "abc123", Message: "test", Author: "dev"}}
	mockGit := &mocks.MockGitRepository{
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return commits, nil
		},
		ChangedFilesFn: func(ctx context.Context, hash string) ([]string, error) {
			return []string{"main.go"}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		SaveFn: func(ctx context.Context, pattern *domain.Pattern) error {
			return errors.New("store failed")
		},
	}
	marked := false
	mockTracker := &mocks.MockCommitTracker{
		IsAnalyzedFn: func(ctx context.Context, hash string) (bool, error) { return false, nil },
		MarkAnalyzedFn: func(ctx context.Context, hash string) error {
			marked = true
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		BatchLearnFromCommitsFn: func(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
			return &agent.BatchLearnResult{Patterns: []domain.Pattern{*newLearnerTestPattern("p1", "Pattern", domain.CategoryError)}}, nil
		},
	}

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, curator.NewService(mockAgent, mockPattern))
	require.Error(t, svc.Learn(context.Background(), 10, "", 5))
	require.False(t, marked)
}

func TestLearn_SavesNewPatternWhenNoSimilarPattern(t *testing.T) {
	commits := []domain.CommitInfo{{Hash: "abc123", Message: "test", Author: "dev"}}
	savedPatterns := []string{}

	mockGit := &mocks.MockGitRepository{
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return commits, nil
		},
		ChangedFilesFn: func(ctx context.Context, hash string) ([]string, error) {
			return []string{"main.go"}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			savedPatterns = append(savedPatterns, p.Name)
			return nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{
		IsAnalyzedFn: func(ctx context.Context, hash string) (bool, error) {
			return false, nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		BatchLearnFromCommitsFn: func(ctx context.Context, req *agent.BatchLearnRequest) (*agent.BatchLearnResult, error) {
			return &agent.BatchLearnResult{
				Patterns:  []domain.Pattern{*newLearnerTestPattern("p1", "New Pattern", domain.CategoryError)},
				LearnedAt: time.Now(),
			}, nil
		},
	}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.Learn(context.Background(), 10, "", 5)
	require.NoError(t, err)
	assert.Equal(t, []string{"New Pattern"}, savedPatterns)
}

func TestLearnFromCommit_DoesNotFetchOrSendDiff(t *testing.T) {
	mockGit := &mocks.MockGitRepository{
		ChangedFilesFn: func(ctx context.Context, hash string) ([]string, error) {
			return []string{"main.go"}, nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		LearnFromCommitFn: func(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
			assert.Equal(t, "abc123", req.Commit.Hash)
			assert.Equal(t, []string{"main.go"}, req.ChangedFiles)
			return &agent.LearnResult{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.LearnFromCommit(context.Background(), domain.CommitInfo{Hash: "abc123"})
	assert.NoError(t, err)
}

func TestLearnFromStaged_Success(t *testing.T) {
	saved := []string{}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = append(saved, p.ID)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		LearnFromCommitFn: func(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
			assert.Equal(t, []string{"main.go"}, req.ChangedFiles)
			return &agent.LearnResult{
				Patterns: []domain.Pattern{
					*newLearnerTestPattern("p1", "Pattern 1", domain.CategoryError),
				},
				LearnedAt: time.Now(),
			}, nil
		},
	}
	mockGit := &mocks.MockGitRepository{
		StagedFilesFn: func(ctx context.Context) ([]domain.FileInfo, error) {
			return []domain.FileInfo{{Path: "main.go", Content: "package main"}}, nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.LearnFromStaged(context.Background(), domain.CommitInfo{Hash: "abc"})
	require.NoError(t, err)
	assert.Len(t, saved, 1)
}

func TestLearnFromStaged_AIError(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		LearnFromCommitFn: func(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
			return nil, errors.New("AI error")
		},
	}
	mockGit := &mocks.MockGitRepository{
		StagedFilesFn: func(ctx context.Context) ([]domain.FileInfo, error) {
			return []domain.FileInfo{{Path: "main.go"}}, nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.LearnFromStaged(context.Background(), domain.CommitInfo{Hash: "abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI")
}

func TestLearnFromCommit_WithMerge(t *testing.T) {
	existingPattern := newLearnerTestPattern("e1", "Error Handling", domain.CategoryError)
	existingPattern.Confidence = 0.8
	existingPattern.Frequency = 3
	existingPattern.SetRule("wrap errors with context")

	mockGit := &mocks.MockGitRepository{
		ChangedFilesFn: func(ctx context.Context, hash string) ([]string, error) {
			return []string{"main.go"}, nil
		},
	}
	mergedPatterns := []*domain.Pattern{}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*existingPattern}, nil
		},
		DeleteFn: func(ctx context.Context, id string) error {
			return nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			mergedPatterns = append(mergedPatterns, p)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		LearnFromCommitFn: func(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
			p := newLearnerTestPattern("p1", "Error Handling", domain.CategoryError)
			p.Confidence = 0.9
			p.SetRule("wrap errors with context")
			return &agent.LearnResult{
				Patterns:  []domain.Pattern{*p},
				LearnedAt: time.Now(),
			}, nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{}
	mockCurator := curator.NewService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockCurator)
	err := svc.LearnFromCommit(context.Background(), domain.CommitInfo{Hash: "abc123"})
	require.NoError(t, err)
	assert.Len(t, mergedPatterns, 1)
	// 本地合并保留质量得分更高的模式。
	assert.Equal(t, "p1", mergedPatterns[0].ID)
}

func TestSavePatterns_MergesSimilarPatterns(t *testing.T) {
	existingPattern := newLearnerTestPattern("existing", "Error Handling", domain.CategoryError)
	existingPattern.Confidence = 0.8
	existingPattern.Frequency = 2
	existingPattern.SetRule("wrap errors with context")

	var saved *domain.Pattern
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*existingPattern}, nil
		},
		DeleteFn: func(ctx context.Context, id string) error {
			return nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = p
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewLearnerService(mockAgent, &mocks.MockGitRepository{}, mockPattern, &mocks.MockCommitTracker{}, curator.NewService(mockAgent, mockPattern))

	newPattern := newLearnerTestPattern("new", "Error Handling", domain.CategoryError)
	newPattern.Confidence = 0.9
	newPattern.Frequency = 1
	newPattern.SetRule("wrap errors with context")
	count, err := svc.CurateAndSavePatterns(context.Background(), []domain.Pattern{*newPattern}, curator.OperationLearnCurrent)

	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NotNil(t, saved)
	require.Equal(t, "new", saved.ID)
	require.Equal(t, 1, saved.Frequency)
}

func TestCurateAndSavePatternsReturnsErrorWhenSaveFails(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			return errors.New("db closed")
		},
	}
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewLearnerService(mockAgent, &mocks.MockGitRepository{}, mockPattern, &mocks.MockCommitTracker{}, curator.NewService(mockAgent, mockPattern))

	count, err := svc.CurateAndSavePatterns(context.Background(), []domain.Pattern{*newLearnerTestPattern("new", "Error Handling", domain.CategoryError)}, curator.OperationLearnCurrent)

	require.Error(t, err)
	require.Zero(t, count)
	require.Contains(t, err.Error(), "db closed")
}

func TestSavePatterns_DoesNotPrintPerPatternSuccessLogs(t *testing.T) {
	t.Run("new patterns", func(t *testing.T) {
		mockPattern := &mocks.MockPatternRepository{
			FindSimilarFn: func(ctx context.Context, p *domain.Pattern) (*domain.Pattern, error) {
				return nil, nil
			},
			SaveFn: func(ctx context.Context, p *domain.Pattern) error {
				return nil
			},
		}
		mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
		svc := NewLearnerService(mockAgent, &mocks.MockGitRepository{}, mockPattern, &mocks.MockCommitTracker{}, curator.NewService(nil, mockPattern))

		patterns := []domain.Pattern{
			*newLearnerTestPattern("p1", "Error Handling", domain.CategoryError),
			*newLearnerTestPattern("p2", "Naming Convention", domain.CategoryNaming),
		}

		var count int
		output := captureStdout(t, func() {
			var err error
			count, err = svc.CurateAndSavePatterns(context.Background(), patterns, curator.OperationLearnHistory)
			require.NoError(t, err)
		})

		require.Equal(t, 2, count)
		require.Empty(t, output)
	})

	t.Run("merged patterns", func(t *testing.T) {
		existingPattern := newLearnerTestPattern("existing", "Error Handling", domain.CategoryError)
		existingPattern.Frequency = 2
		mockPattern := &mocks.MockPatternRepository{
			FindSimilarFn: func(ctx context.Context, p *domain.Pattern) (*domain.Pattern, error) {
				pCopy := *existingPattern
				return &pCopy, nil
			},
			SaveFn: func(ctx context.Context, p *domain.Pattern) error {
				return nil
			},
		}
		mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
		svc := NewLearnerService(mockAgent, &mocks.MockGitRepository{}, mockPattern, &mocks.MockCommitTracker{}, curator.NewService(nil, mockPattern))

		var count int
		output := captureStdout(t, func() {
			var err error
			count, err = svc.CurateAndSavePatterns(context.Background(), []domain.Pattern{*newLearnerTestPattern("new", "Error Handling", domain.CategoryError)}, curator.OperationLearnHistory)
			require.NoError(t, err)
		})

		require.Equal(t, 1, count)
		require.Empty(t, output)
	})
}

func newLearnerTestPattern(id, name string, category domain.Category) *domain.Pattern {
	pattern := domain.NewPattern(id, name, category)
	pattern.Rule = "Preserve the project-specific " + name + " rule."
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/example.go", Line: 1, Kind: "file"}}
	return pattern
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return string(output)
}
