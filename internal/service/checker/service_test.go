package checker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	snapshotstore "github.com/silaswei-io/skills-seed/internal/infra/storage/snapshot"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCheck_GitErrorUsesActiveLocale(t *testing.T) {
	mockGit := &mocks.MockGitRepository{
		StagedFilesFn: func(context.Context) ([]domain.FileInfo, error) {
			return nil, errors.New("git error")
		},
	}
	svc := newTestService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, mockGit, &mocks.MockPatternRepository{})

	require.NoError(t, i18n.Init(i18n.LocaleEnglish))
	_, err := svc.Check(context.Background())
	require.ErrorContains(t, err, "Failed to get staged files")

	require.NoError(t, i18n.Init(i18n.LocaleChinese))
	_, err = svc.Check(context.Background())
	require.ErrorContains(t, err, "获取暂存文件失败")
}

func TestCheckAll_Success(t *testing.T) {
	projectRoot := t.TempDir()
	writeCheckerFile(t, projectRoot, "main.go", "package main\n")
	writeCheckerFile(t, projectRoot, "handler.go", "package main\n")

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
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: projectRoot},
	}
	svc := NewCheckerService(mockAgent, mockGit, mockPattern, cfg)
	issues, err := svc.CheckAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, issues, 0)
}

func TestCheckAll_FiltersExcludedFiles(t *testing.T) {
	projectRoot := t.TempDir()
	writeCheckerFile(t, projectRoot, "internal/service/user.go", "package service\n")
	writeCheckerFile(t, projectRoot, "internal/service/mocks/repo.go", "package mocks\n")
	writeCheckerFile(t, projectRoot, "api/user.pb.go", "package api\n")

	var analyzed []domain.FileInfo
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			analyzed = req.Files
			return &agent.AnalyzeResult{Issues: []domain.Issue{}, Confidence: 0.5}, nil
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
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: projectRoot},
		Exclude:    []string{"**/mocks/**", "**/*.pb.go"},
	}

	svc := NewCheckerService(mockAgent, mockGit, mockPattern, cfg)
	_, err := svc.CheckAll(context.Background())

	assert.NoError(t, err)
	require.Len(t, analyzed, 1)
	assert.Equal(t, "internal/service/user.go", analyzed[0].Path)
}

func TestCheckAllUsesSnapshotDiffsAndReplacesSnapshots(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	writeCheckerFile(t, projectRoot, "added.go", "package added\n")
	writeCheckerFile(t, projectRoot, "modified.go", "package main\nfunc newName() {}\n")
	writeCheckerFile(t, projectRoot, "unchanged.go", "package same\n")
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{
		"modified.go":  "package main\nfunc oldName() {}\n",
		"unchanged.go": "package same\n",
	}))

	var received *agent.AnalyzeRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			received = req
			return &agent.AnalyzeResult{Issues: []domain.Issue{}, Confidence: 0.5}, nil
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
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: projectRoot},
		Exclude:    []string{".*"},
	}
	svc := NewCheckerService(mockAgent, mockGit, mockPattern, cfg)
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	issues, err := svc.CheckAll(ctx)

	require.NoError(t, err)
	require.Empty(t, issues)
	require.NotNil(t, received)
	require.Equal(t, []domain.FileInfo{domain.NewFileInfo("added.go", "")}, received.Files)
	require.Len(t, received.DiffFiles, 1)
	require.Equal(t, "modified.go", received.DiffFiles[0].Path)
	diffContent, err := os.ReadFile(received.DiffFiles[0].DiffPath)
	require.NoError(t, err)
	require.Contains(t, string(diffContent), "-func oldName() {}")
	require.Contains(t, string(diffContent), "+func newName() {}")

	loaded, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"added.go":     "package added\n",
		"modified.go":  "package main\nfunc newName() {}\n",
		"unchanged.go": "package same\n",
	}, loaded)
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

func TestCheckFiles_RecordsPatternHits(t *testing.T) {
	recorded := []domain.PatternHit{}
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			return &agent.AnalyzeResult{
				Issues: []domain.Issue{
					{
						File:       "internal/service/checker/service.go",
						Line:       81,
						Severity:   domain.SeverityWarning,
						Message:    "use domain error",
						PatternID:  "domain-error-wrap",
						Confidence: 0.86,
					},
					{
						File:      "internal/service/checker/service.go",
						Line:      82,
						Severity:  domain.SeverityInfo,
						Message:   "no pattern id",
						PatternID: "",
					},
				},
				Confidence: 0.8,
			}, nil
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
		RecordPatternHitsFn: func(ctx context.Context, hits []domain.PatternHit) error {
			recorded = append(recorded, hits...)
			return nil
		},
	}

	svc := newTestService(mockAgent, mockGit, mockPattern)
	issues, err := svc.CheckFiles(context.Background(), []domain.FileInfo{
		{Path: "internal/service/checker/service.go", Content: "package checker"},
	})

	assert.NoError(t, err)
	assert.Len(t, issues, 2)
	require.Len(t, recorded, 1)
	assert.Equal(t, "domain-error-wrap", recorded[0].PatternID)
	assert.Equal(t, "internal/service/checker/service.go", recorded[0].File)
	assert.Equal(t, 81, recorded[0].Line)
	assert.Equal(t, domain.SeverityWarning, recorded[0].Severity)
	assert.Equal(t, 0.86, recorded[0].Confidence)
	assert.NotEmpty(t, recorded[0].CheckRunID)
	assert.False(t, recorded[0].CreatedAt.IsZero())
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

func writeCheckerFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
