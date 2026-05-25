package learner

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/merger"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLearnerService(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockGit := &mocks.MockGitRepository{}
	mockPattern := &mocks.MockPatternRepository{}
	mockTracker := &mocks.MockCommitTracker{}
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
	assert.NotNil(t, svc)
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
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
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
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
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
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
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
		FindSimilarFn: func(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error) {
			return nil, errors.New("not found")
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
					*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
					*domain.NewPattern("p2", "Naming Convention", domain.CategoryNaming),
				},
				LearnedAt: time.Now(),
			}, nil
		},
	}
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
	err := svc.Learn(context.Background(), 10, "", 5)
	assert.NoError(t, err)
	assert.Len(t, savedPatterns, 2)
	assert.Len(t, markCalled, 2)
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
		FindSimilarFn: func(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error) {
			return nil, nil
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
				Patterns:  []domain.Pattern{*domain.NewPattern("p1", "New Pattern", domain.CategoryError)},
				LearnedAt: time.Now(),
			}, nil
		},
	}
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
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
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
	err := svc.LearnFromCommit(context.Background(), domain.CommitInfo{Hash: "abc123"})
	assert.NoError(t, err)
}

func TestLearnFromStaged_Success(t *testing.T) {
	saved := []string{}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
		FindSimilarFn: func(ctx context.Context, p *domain.Pattern) (*domain.Pattern, error) {
			return nil, errors.New("not found")
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
					*domain.NewPattern("p1", "Pattern 1", domain.CategoryError),
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
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
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
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
	err := svc.LearnFromStaged(context.Background(), domain.CommitInfo{Hash: "abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI")
}

func TestLearnFromCommit_WithMerge(t *testing.T) {
	existingPattern := domain.NewPattern("e1", "Error Handling", domain.CategoryError)
	existingPattern.Frequency = 3

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
		FindSimilarFn: func(ctx context.Context, p *domain.Pattern) (*domain.Pattern, error) {
			// Simulate finding a similar pattern
			pCopy := *existingPattern
			return &pCopy, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			mergedPatterns = append(mergedPatterns, p)
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		LearnFromCommitFn: func(ctx context.Context, req *agent.LearnRequest) (*agent.LearnResult, error) {
			return &agent.LearnResult{
				Patterns: []domain.Pattern{
					*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
				},
				LearnedAt: time.Now(),
			}, nil
		},
	}
	mockTracker := &mocks.MockCommitTracker{}
	mockMerger := merger.NewMergerService(mockAgent, mockPattern)

	svc := NewLearnerService(mockAgent, mockGit, mockPattern, mockTracker, mockMerger)
	err := svc.LearnFromCommit(context.Background(), domain.CommitInfo{Hash: "abc123"})
	require.NoError(t, err)
	assert.Len(t, mergedPatterns, 1)
	// Should be merged into existing pattern
	assert.Equal(t, "e1", mergedPatterns[0].ID)
}

func TestSavePatterns_MergesSimilarPatterns(t *testing.T) {
	existingPattern := domain.NewPattern("existing", "Error Handling", domain.CategoryError)
	existingPattern.Frequency = 2

	var saved *domain.Pattern
	mockPattern := &mocks.MockPatternRepository{
		FindSimilarFn: func(ctx context.Context, p *domain.Pattern) (*domain.Pattern, error) {
			pCopy := *existingPattern
			return &pCopy, nil
		},
		SaveFn: func(ctx context.Context, p *domain.Pattern) error {
			saved = p
			return nil
		},
	}
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewLearnerService(mockAgent, &mocks.MockGitRepository{}, mockPattern, &mocks.MockCommitTracker{}, merger.NewMergerService(mockAgent, mockPattern))

	newPattern := domain.NewPattern("new", "Error Handling", domain.CategoryError)
	newPattern.Frequency = 1
	count := svc.SavePatterns(context.Background(), []domain.Pattern{*newPattern}, "learn_current")

	require.Equal(t, 1, count)
	require.NotNil(t, saved)
	require.Equal(t, "existing", saved.ID)
	require.Equal(t, 3, saved.Frequency)
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
		svc := NewLearnerService(mockAgent, &mocks.MockGitRepository{}, mockPattern, &mocks.MockCommitTracker{}, merger.NewMergerService(mockAgent, mockPattern))

		patterns := []domain.Pattern{
			*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
			*domain.NewPattern("p2", "Naming Convention", domain.CategoryNaming),
		}

		var count int
		output := captureStdout(t, func() {
			count = svc.SavePatterns(context.Background(), patterns, "learn_current")
		})

		require.Equal(t, 2, count)
		require.Empty(t, output)
	})

	t.Run("merged patterns", func(t *testing.T) {
		existingPattern := domain.NewPattern("existing", "Error Handling", domain.CategoryError)
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
		svc := NewLearnerService(mockAgent, &mocks.MockGitRepository{}, mockPattern, &mocks.MockCommitTracker{}, merger.NewMergerService(mockAgent, mockPattern))

		var count int
		output := captureStdout(t, func() {
			count = svc.SavePatterns(context.Background(), []domain.Pattern{*domain.NewPattern("new", "Error Handling", domain.CategoryError)}, "learn_current")
		})

		require.Equal(t, 1, count)
		require.Empty(t, output)
	})
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
