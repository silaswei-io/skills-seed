package checker

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
)

func newTestService(mockAgent *mocks.MockAgent, mockGit *mocks.MockGitRepository, mockPattern *mocks.MockPatternRepository) *CheckerService {
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	return NewCheckerService(mockAgent, mockGit, mockPattern, cfg)
}

func TestCheck_Success(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			assert.Empty(t, req.Files[0].Content)
			return &agent.AnalyzeResult{
				Issues: []domain.Issue{
					{File: "main.go", Severity: "warning", Message: "test issue"},
				},
				Confidence: 0.9,
			}, nil
		},
	}
	mockGit := &mocks.MockGitRepository{
		StagedFilesFn: func(ctx context.Context) ([]domain.FileInfo, error) {
			return []domain.FileInfo{
				{Path: "main.go", Content: "package main", Language: "go"},
			}, nil
		},
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return []domain.CommitInfo{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}

	svc := newTestService(mockAgent, mockGit, mockPattern)
	issues, err := svc.Check(context.Background())
	assert.NoError(t, err)
	assert.Len(t, issues, 1)
}

func TestCheck_GitError(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockGit := &mocks.MockGitRepository{
		StagedFilesFn: func(ctx context.Context) ([]domain.FileInfo, error) {
			return nil, errors.New("git error")
		},
	}
	mockPattern := &mocks.MockPatternRepository{}

	svc := newTestService(mockAgent, mockGit, mockPattern)
	_, err := svc.Check(context.Background())
	assert.Error(t, err)
}

func TestCheckAll_Success(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			for _, file := range req.Files {
				assert.Empty(t, file.Content)
			}
			return &agent.AnalyzeResult{Issues: []domain.Issue{}, Confidence: 0.5}, nil
		},
	}
	mockGit := &mocks.MockGitRepository{
		AllFilesFn: func(ctx context.Context) ([]domain.FileInfo, error) {
			return []domain.FileInfo{
				{Path: "main.go", Content: "package main", Language: "go"},
				{Path: "handler.go", Content: "package main", Language: "go"},
			}, nil
		},
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return []domain.CommitInfo{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}

	svc := newTestService(mockAgent, mockGit, mockPattern)
	issues, err := svc.CheckAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, issues, 0)
}

func TestCheckAll_FiltersExcludedFiles(t *testing.T) {
	var analyzed []domain.FileInfo
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			analyzed = req.Files
			return &agent.AnalyzeResult{Issues: []domain.Issue{}, Confidence: 0.5}, nil
		},
	}
	mockGit := &mocks.MockGitRepository{
		AllFilesFn: func(ctx context.Context) ([]domain.FileInfo, error) {
			return []domain.FileInfo{
				domain.NewFileInfo("internal/service/user.go", "package service"),
				domain.NewFileInfo("internal/service/mocks/repo.go", "package mocks"),
				domain.NewFileInfo("api/user.pb.go", "package api"),
			}, nil
		},
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return []domain.CommitInfo{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
		Exclude:    []string{"**/mocks/**", "**/*.pb.go"},
	}

	svc := NewCheckerService(mockAgent, mockGit, mockPattern, cfg)
	_, err := svc.CheckAll(context.Background())

	assert.NoError(t, err)
	assert.Len(t, analyzed, 1)
	assert.Equal(t, "internal/service/user.go", analyzed[0].Path)
}

func TestCheckFiles_AIError(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			return nil, errors.New("AI error")
		},
	}
	mockGit := &mocks.MockGitRepository{
		CommitsFn: func(ctx context.Context, limit int, since string) ([]domain.CommitInfo, error) {
			return []domain.CommitInfo{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}

	svc := newTestService(mockAgent, mockGit, mockPattern)
	_, err := svc.CheckFiles(context.Background(), []domain.FileInfo{
		{Path: "main.go", Content: "package main"},
	})
	assert.Error(t, err)
}

func TestCheckFiles_EmptySkipsAgent(t *testing.T) {
	called := false
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			called = true
			return &agent.AnalyzeResult{}, nil
		},
	}

	svc := newTestService(mockAgent, &mocks.MockGitRepository{}, &mocks.MockPatternRepository{})
	issues, err := svc.CheckFiles(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, issues)
	assert.False(t, called)
}

func TestGetPatterns(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "Pattern 1", domain.CategoryError),
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return patterns, nil
		},
	}
	svc := newTestService(nil, nil, mockPattern)
	result, err := svc.GetPatterns(context.Background())
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetHighConfidencePatterns(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "High", domain.CategoryError),
	}
	patterns[0].Confidence = 0.95
	mockPattern := &mocks.MockPatternRepository{
		GetHighConfidenceFn: func(ctx context.Context, threshold float64) ([]domain.Pattern, error) {
			return patterns, nil
		},
	}
	svc := newTestService(nil, nil, mockPattern)
	result, err := svc.GetHighConfidencePatterns(context.Background(), 0.8)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestCheckFiles_PatternRepoError(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return nil, errors.New("db error")
		},
	}
	svc := newTestService(nil, nil, mockPattern)
	_, err := svc.CheckFiles(context.Background(), []domain.FileInfo{
		{Path: "main.go"},
	})
	assert.Error(t, err)
}
