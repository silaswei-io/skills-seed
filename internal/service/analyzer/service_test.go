package analyzer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	snapshotstore "github.com/silaswei-io/skills-seed/internal/infra/storage/snapshot"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProjectStructure(t *testing.T) {
	tmpDir := t.TempDir()
	// 创建一些目录和文件
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cmd"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	structure, err := svc.GetProjectStructure(tmpDir)
	require.NoError(t, err)
	assert.Contains(t, structure, "cmd")
	assert.Contains(t, structure, "internal")
	assert.Contains(t, structure, "main.go")
}

func TestGetProjectStructureUsesConfiguredExclude(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "generated"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "generated", "wire.go"), []byte("package generated"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	svc := NewAnalyzerService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: tmpDir},
		Exclude:    []string{"internal/generated/**"},
	})
	structure, err := svc.GetProjectStructure(tmpDir)

	require.NoError(t, err)
	assert.Contains(t, structure, "main.go")
	assert.NotContains(t, structure, "wire.go")
}

func TestFindMainFiles(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "cmd", "server"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cmd", "server", "main.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	mainFiles := svc.FindMainFiles(tmpDir)
	assert.NotEmpty(t, mainFiles)
	assert.Contains(t, mainFiles, "main.go")
}

func TestAnalyzePatterns(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			return &agent.AnalyzeResult{
				Issues: []domain.Issue{
					{File: "main.go", Severity: "warning", Message: "test"},
				},
				Confidence: 0.9,
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	result, err := svc.AnalyzePatterns(context.Background(), &AnalyzePatternsRequest{
		Files: []domain.FileInfo{{Path: "main.go", Content: "package main"}},
	})
	require.NoError(t, err)
	assert.Len(t, result.Issues, 1)
	assert.Equal(t, 0.9, result.Confidence)
}

func TestAnalyzePatterns_AIError(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCodeFn: func(ctx context.Context, req *agent.AnalyzeRequest) (*agent.AnalyzeResult, error) {
			return nil, errors.New("AI error")
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	_, err := svc.AnalyzePatterns(context.Background(), &AnalyzePatternsRequest{})
	assert.Error(t, err)
}

func TestAnalyzeProject(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return &agent.AnalyzeProjectResult{
				Language:           "go",
				Frameworks:         []string{"gin"},
				Architecture:       "DDD",
				ValidationCommands: []domain.ValidationCommand{{Command: "task verify", When: "after changes", Source: "Taskfile.yml"}},
				Summary:            "Test project summary",
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	result, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    "/tmp/test",
	})
	require.NoError(t, err)
	assert.Equal(t, "go", result.Language)
	assert.Contains(t, result.Frameworks, "gin")
	require.Len(t, result.ValidationCommands, 1)
	assert.Equal(t, "task verify", result.ValidationCommands[0].Command)
}

func TestNewProjectProfilePreservesValidationCommands(t *testing.T) {
	profile := NewProjectProfile(&AnalyzeProjectResult{
		Language:           "unknown",
		ValidationCommands: []domain.ValidationCommand{{Command: "task verify", When: "after changes", Source: "Taskfile.yml"}},
		Summary:            "profile",
	}, "demo", "")

	require.NotNil(t, profile)
	require.Len(t, profile.ValidationCommands, 1)
	assert.Equal(t, "task verify", profile.ValidationCommands[0].Command)
	assert.Equal(t, "Taskfile.yml", profile.ValidationCommands[0].Source)
}

func TestAnalyzeProjectAddsStructuralContext(t *testing.T) {
	tmpDir := t.TempDir()
	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		LearningCfg: config.LearningConfig{
			Current: config.CurrentLearningConfig{
				Structural: config.StructuralConfig{
					Enabled: true,
				},
			},
		},
	})
	svc.structuralCollector = fakeStructuralCollector{
		context: "## Structural Context\n- main calls service",
	}

	_, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
		MainFiles:   []string{"main.go"},
	})

	require.NoError(t, err)
	require.Contains(t, received.StructuralContext, "main calls service")
}

func TestAnalyzeProjectSkipsStructuralContextWithoutSeeds(t *testing.T) {
	tmpDir := t.TempDir()
	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		LearningCfg: config.LearningConfig{
			Current: config.CurrentLearningConfig{
				Structural: config.StructuralConfig{
					Enabled: true,
				},
			},
		},
	})
	svc.structuralCollector = fakeStructuralCollector{
		context: "## Structural Context\n- should not be used",
	}

	_, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
	})

	require.NoError(t, err)
	require.Empty(t, received.StructuralContext)
}

func TestAnalyzeProjectSkipsUnavailableOptionalStructuralContext(t *testing.T) {
	tmpDir := t.TempDir()
	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		LearningCfg: config.LearningConfig{
			Current: config.CurrentLearningConfig{
				Structural: config.StructuralConfig{
					Enabled: true,
				},
			},
		},
	})
	svc.structuralCollector = fakeStructuralCollector{
		err: errors.New("unavailable"),
	}

	_, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
		MainFiles:   []string{"main.go"},
	})

	require.NoError(t, err)
	require.Empty(t, received.StructuralContext)
}

func TestAnalyzeProject_AIError(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return nil, errors.New("AI error")
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	_, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{})
	assert.Error(t, err)
}

func TestTreeSitterCollectorMaxFileSizeUsesKilobytes(t *testing.T) {
	projectRoot := t.TempDir()
	smallSource := "package main\n\nfunc Small() {}\n"
	largeSource := "package main\n\n" + strings.Repeat("// padding\n", 140) + "\nfunc Large() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "small.go"), []byte(smallSource), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "large.go"), []byte(largeSource), 0644))

	collector := newStructuralCollector(config.StructuralConfig{
		Enabled:     true,
		Provider:    config.StructuralProviderTreeSitter,
		MaxSymbols:  10,
		MaxFileSize: 1,
	})

	result, err := collector.Collect(context.Background(), projectRoot, structuralContextRequest{
		SeedPaths: []string{"small.go", "large.go"},
	})

	require.NoError(t, err)
	require.Contains(t, result, "Small")
	require.NotContains(t, result, "Large")
}

func TestAnalyzeCurrentCodebase(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			return &agent.AnalyzeCurrentCodebaseResult{
				Patterns: []domain.Pattern{
					*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
				},
				ProfileDelta: domain.ProjectProfileDelta{
					ConfigPatterns: []string{"Always wrap errors"},
				},
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	result, err := svc.AnalyzeCurrentCodebase(context.Background(), &AnalyzeCurrentCodebaseRequest{
		ProjectName: "test",
		RootPath:    "/tmp/test",
		Language:    "go",
	})
	require.NoError(t, err)
	assert.Len(t, result.Patterns, 1)
	assert.Contains(t, result.ProfileDelta.ConfigPatterns, "Always wrap errors")
}

func TestAnalyzeCurrentCodebaseAddsStructuralContext(t *testing.T) {
	tmpDir := t.TempDir()
	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		LearningCfg: config.LearningConfig{
			Current: config.CurrentLearningConfig{
				Structural: config.StructuralConfig{
					Enabled: true,
				},
			},
		},
	})
	svc.structuralCollector = fakeStructuralCollector{
		context: "## Structural Context\n- service has 3 callers",
	}

	_, err := svc.AnalyzeCurrentCodebase(context.Background(), &AnalyzeCurrentCodebaseRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
		SampleFiles: []agent.SampleFile{{Path: "internal/service.go"}},
	})

	require.NoError(t, err)
	require.Contains(t, received.StructuralContext, "service has 3 callers")
}

func TestAnalyzeCurrentCodebasePassesBoundedSeedsToStructuralCollector(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		LearningCfg: config.LearningConfig{
			Current: config.CurrentLearningConfig{
				Structural: config.StructuralConfig{
					Enabled: true,
				},
			},
		},
	})
	collector := &recordingStructuralCollector{context: "## Structural Context\n"}
	svc.structuralCollector = collector

	_, err := svc.AnalyzeCurrentCodebase(context.Background(), &AnalyzeCurrentCodebaseRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
		FocusPaths:  []string{"internal/service"},
		MainFiles:   []string{"cmd/app/main.go"},
		SampleFiles: []agent.SampleFile{{Path: "internal/new.go"}},
		DiffFiles:   []agent.DiffFileRef{{Path: "internal/changed.go", DiffPath: "/tmp/changed.go.diff"}},
	})

	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"cmd/app/main.go",
		"internal/new.go",
		"internal/changed.go",
	}, collector.req.SeedPaths)
}

func TestAnalyzeCurrentCodebase_AIError(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			return nil, errors.New("AI error")
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	// AnalyzeCurrentCodebase 在 AI 失败时返回领域错误。
	_, err := svc.AnalyzeCurrentCodebase(context.Background(), &AnalyzeCurrentCodebaseRequest{})
	assert.Error(t, err)
}

func TestAnalyzeCodebaseFull(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}"), 0644))

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			return &agent.AnalyzeCurrentCodebaseResult{
				Patterns: []domain.Pattern{
					*domain.NewPattern("p1", "Test Pattern", domain.CategoryNaming),
				},
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	result, patterns, err := svc.AnalyzeCodebaseFull(context.Background(), tmpDir, "test", "go")
	require.NoError(t, err)
	assert.Len(t, result.Patterns, 1)
	assert.Len(t, patterns, 1)
}

func TestAnalyzeCodebaseFullWithFocusPathsOnlySendsFocusedSamples(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "agent"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "prompts"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "agent", "agent.go"), []byte("package agent\nfunc Agent() {}\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "prompts", "loader.go"), []byte("package prompts\nfunc Loader() {}\n"), 0644))

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{
				Patterns: []domain.Pattern{
					*domain.NewPattern("agent-pattern", "Agent Pattern", domain.CategoryStructure),
				},
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	_, patterns, err := svc.AnalyzeCodebaseFullWithOptions(context.Background(), tmpDir, "test", "go", AnalyzeCodebaseOptions{
		FocusPaths: []string{filepath.Join(tmpDir, "internal", "agent")},
	})

	require.NoError(t, err)
	require.Len(t, patterns, 1)
	require.Equal(t, []string{"internal/agent"}, received.FocusPaths)
	require.NotEmpty(t, received.SampleFiles)
	for _, file := range received.SampleFiles {
		require.Contains(t, file.Path, "internal/agent")
		require.NotContains(t, file.Path, "internal/prompts")
	}
}

func TestAnalyzeCodebaseFullWithFocusPathsOnlyDiffsFocusedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "agent"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "prompts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "agent", "agent.go"), []byte("package agent\nfunc NewAgent() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "prompts", "loader.go"), []byte("package prompts\nfunc NewLoader() {}\n"), 0o644))
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{
		"internal/agent/agent.go":    "package agent\nfunc OldAgent() {}\n",
		"internal/agent/deleted.go":  "package agent\nfunc Deleted() {}\n",
		"internal/prompts/loader.go": "package prompts\nfunc OldLoader() {}\n",
		"internal/unchanged/keep.go": "package unchanged\n",
	}))

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: tmpDir},
		Exclude:    []string{".*"},
	})
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	_, _, err := svc.AnalyzeCodebaseFullWithOptions(ctx, tmpDir, "test", "go", AnalyzeCodebaseOptions{
		FocusPaths:       []string{filepath.Join(tmpDir, "internal", "agent")},
		UseSnapshotDiffs: true,
	})

	require.NoError(t, err)
	require.Len(t, received.DiffFiles, 2)
	require.Equal(t, "internal/agent/agent.go", received.DiffFiles[0].Path)
	require.Equal(t, "internal/agent/deleted.go", received.DiffFiles[1].Path)
	require.Empty(t, received.SampleFiles)
	loaded, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, "package agent\nfunc OldAgent() {}\n", loaded["internal/agent/agent.go"])
	require.Equal(t, "package agent\nfunc Deleted() {}\n", loaded["internal/agent/deleted.go"])
	require.Equal(t, "package prompts\nfunc OldLoader() {}\n", loaded["internal/prompts/loader.go"])
	require.Equal(t, "package unchanged\n", loaded["internal/unchanged/keep.go"])
}

func TestAnalyzeCodebaseFullDoesNotPassKnownPatternsToCurrentAnalysis(t *testing.T) {
	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal:      "mock",
		AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644))

	_, _, err := svc.AnalyzeCodebaseFullWithOptions(context.Background(), tmpDir, "demo", "go", AnalyzeCodebaseOptions{
		KnownPatternsJSON:  `[{"id":"known"}]`,
		KnownPatternsCount: 1,
	})

	require.NoError(t, err)
	require.Empty(t, received.KnownPatternsJSON)
	require.Zero(t, received.KnownPatternsCount)
}

func TestAnalyzeCodebaseFullUsesSnapshotDiffsWithoutCommittingSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "added.go"), []byte("package added\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "modified.go"), []byte("package main\nfunc newName() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "unchanged.go"), []byte("package same\n"), 0o644))
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{
		"modified.go":  "package main\nfunc oldName() {}\n",
		"unchanged.go": "package same\n",
	}))

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{
				Patterns: []domain.Pattern{*domain.NewPattern("p1", "Snapshot Pattern", domain.CategoryStructure)},
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: tmpDir},
		Exclude:    []string{".*"},
	})
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	_, patterns, err := svc.AnalyzeCodebaseFullWithOptions(ctx, tmpDir, "test", "go", AnalyzeCodebaseOptions{})

	require.NoError(t, err)
	require.Len(t, patterns, 1)
	require.Len(t, received.SampleFiles, 1)
	require.Equal(t, "added.go", received.SampleFiles[0].Path)
	require.Len(t, received.DiffFiles, 1)
	require.Equal(t, "modified.go", received.DiffFiles[0].Path)
	diffContent, err := os.ReadFile(received.DiffFiles[0].DiffPath)
	require.NoError(t, err)
	require.Contains(t, string(diffContent), "-func oldName() {}")
	require.Contains(t, string(diffContent), "+func newName() {}")

	loaded, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"modified.go":  "package main\nfunc oldName() {}\n",
		"unchanged.go": "package same\n",
	}, loaded)
}

func TestAnalyzeCodebaseFullWithExternalRunContextDoesNotCommitSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "auth"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "key"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "auth", "login.go"), []byte("package auth\nfunc NewLogin() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "key", "create.go"), []byte("package key\nfunc NewCreate() {}\n"), 0o644))
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{
		"internal/auth/login.go": "package auth\nfunc OldLogin() {}\n",
		"internal/key/create.go": "package key\nfunc OldCreate() {}\n",
	}))

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: tmpDir},
		Exclude:    []string{".*"},
	})
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)
	runContext, err := svc.BuildCodebaseRunContext(ctx, tmpDir, "go", AnalyzeCodebaseOptions{
		SelectedFiles: []domain.FileInfo{
			{Path: "internal/auth/login.go"},
			{Path: "internal/key/create.go"},
		},
		SelectedFilesSet: true,
		UseSnapshotDiffs: true,
	})
	require.NoError(t, err)

	_, _, err = svc.AnalyzeCodebaseFullWithOptions(ctx, tmpDir, "test", "go", AnalyzeCodebaseOptions{
		FocusPaths:       []string{filepath.Join(tmpDir, "internal", "auth")},
		UseSnapshotDiffs: true,
		RunContext:       runContext,
	})

	require.NoError(t, err)
	require.Len(t, received.DiffFiles, 1)
	require.Equal(t, "internal/auth/login.go", received.DiffFiles[0].Path)
	loaded, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, "package auth\nfunc OldLogin() {}\n", loaded["internal/auth/login.go"])
	require.Equal(t, "package key\nfunc OldCreate() {}\n", loaded["internal/key/create.go"])
}

func TestAnalyzeCodebaseFullDoesNotDiffGitIgnoredSnapshotFiles(t *testing.T) {
	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, ".skills-seed")
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "ignored"), 0o755))
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	require.NoError(t, exec.Command("git", "-C", tmpDir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("ignored/\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc NewMain() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ignored", "generated.go"), []byte("package ignored\nfunc Generated() {}\n"), 0o644))
	repo := snapshotstore.NewRepository(seedPath)
	require.NoError(t, repo.Replace(map[string]string{
		"main.go":              "package main\nfunc OldMain() {}\n",
		"ignored/generated.go": "package ignored\nfunc OldGenerated() {}\n",
	}))

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	configRepo, err := config.NewRepository(seedPath, "zh-CN")
	require.NoError(t, err)
	svc := NewAnalyzerService(mockAgent, configRepo)
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	_, _, err = svc.AnalyzeCodebaseFullWithOptions(ctx, tmpDir, "test", "go", AnalyzeCodebaseOptions{
		SelectedFiles: []domain.FileInfo{
			{Path: "main.go"},
			{Path: "ignored/generated.go"},
		},
		SelectedFilesSet: true,
		UseSnapshotDiffs: true,
	})

	require.NoError(t, err)
	require.Len(t, received.DiffFiles, 1)
	require.Equal(t, "main.go", received.DiffFiles[0].Path)
	for _, diff := range received.DiffFiles {
		require.NotEqual(t, "ignored/generated.go", diff.Path)
	}
	loaded, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, "package ignored\nfunc OldGenerated() {}\n", loaded["ignored/generated.go"])
}

func TestAnalyzeCodebaseFullSkipsDocumentsButKeepsDocsSourceInSnapshotDiffs(t *testing.T) {
	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, ".skills-seed")
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "docs", "examples"), 0o755))
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.MD"), []byte("# readme\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "Guide.MD"), []byte("# guide\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "examples", "main.go"), []byte("package examples\n"), 0o644))

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: tmpDir},
		Exclude:    []string{".*"},
	})
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	_, _, err := svc.AnalyzeCodebaseFullWithOptions(ctx, tmpDir, "test", "go", AnalyzeCodebaseOptions{})

	require.NoError(t, err)
	require.Equal(t, []agent.SampleFile{{Path: "docs/examples/main.go"}}, received.SampleFiles)
	require.Empty(t, received.DiffFiles)
}

func TestAnalyzeCodebaseFullCapsSnapshotAddedSampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	for i := 0; i < 20; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("added_%02d.go", i))
		require.NoError(t, os.WriteFile(path, []byte("package added\n"), 0o644))
	}

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go", RootPath: tmpDir},
		Exclude:    []string{".*"},
	})
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	_, _, err := svc.AnalyzeCodebaseFullWithOptions(ctx, tmpDir, "test", "go", AnalyzeCodebaseOptions{})

	require.NoError(t, err)
	require.Len(t, received.SampleFiles, maxSampleFiles)
}

func TestCollectSampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "user.go"), []byte("package service"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFiles(tmpDir, "go")
	assert.NotEmpty(t, files)
	assertSamplePathsContain(t, files, "main.go", "internal/service/user.go", "main_test.go")
}

func TestCollectSampleFiles_ReturnsPathsWithoutEmbeddingContent(t *testing.T) {
	tmpDir := t.TempDir()
	longUTF8Content := "package main\n\n// " + strings.Repeat("创建SSH会话", 400)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "webshell.go"), []byte(longUTF8Content), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFiles(tmpDir, "go")

	require.Len(t, files, 1)
	assert.Equal(t, "webshell.go", files[0].Path)
}

func TestCollectSampleFiles_DoesNotTreatVendorAsKeyword(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "vendor", "pkg"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vendor", "pkg", "lib.go"), []byte("package pkg"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFiles(tmpDir, "go")
	assertSamplePathsContain(t, files, "main.go", "vendor/pkg/lib.go")
}

func TestCollectSampleFilesKeepsSourceFilesUnderDocs(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "docs", "examples"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "examples", "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "Guide.MD"), []byte("# guide"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFiles(tmpDir, "go")

	require.Len(t, files, 1)
	require.Equal(t, "docs/examples/main.go", files[0].Path)
}

func TestCollectSampleFiles_UsesConfiguredExclude(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "generated"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "generated", "wire.go"), []byte("package generated"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Locale: "zh-CN", Language: "go"},
		AgentCfg:   config.AgentConfig{Engine: "test"},
		Exclude:    []string{"internal/generated/**"},
	})

	files := svc.collectSampleFiles(tmpDir, "go")
	require.NotEmpty(t, files)
	for _, f := range files {
		assert.NotContains(t, f.Path, "internal/generated")
	}
}

func assertSamplePathsContain(t *testing.T, files []agent.SampleFile, expected ...string) {
	t.Helper()
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	for _, path := range expected {
		require.Contains(t, paths, path)
	}
}

func TestNewAnalyzerService_DefaultLocale(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewAnalyzerService(mockAgent, nil)
	assert.NotNil(t, svc)
}

func TestAnalyzeProjectFull_WithMock(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return &agent.AnalyzeProjectResult{
				Language:       "go",
				Frameworks:     []string{"gin", "gorm"},
				Architecture:   "Clean Architecture",
				Dependencies:   []string{"github.com/gin-gonic/gin"},
				Summary:        "A test project",
				KeyModules:     []domain.ModuleInfo{{Name: "handler", Path: "internal/handler"}},
				CommonUtils:    []domain.UtilityFunction{{Name: "Response", File: "pkg/response.go"}},
				ConfigPatterns: []string{"YAML config"},
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := svc.AnalyzeProjectFull(ctx, "/tmp/test", "test-project")
	require.NoError(t, err)
	assert.Equal(t, "go", result.Language)
	assert.Contains(t, result.Frameworks, "gin")
	assert.Contains(t, result.Frameworks, "gorm")
	assert.NotEmpty(t, result.KeyModules)
	assert.NotEmpty(t, result.CommonUtils)
}

func TestAnalyzeProjectFull_PassesReadmePathWithoutContent(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# secret readme content"), 0644))

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.AnalyzeProjectFull(context.Background(), tmpDir, "test-project")

	require.NoError(t, err)
	assert.Equal(t, "README.md", received.ReadmePath)
}

func TestAnalyzeProjectFullWithOptions_PassesIncrementalProfileContext(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "agent"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "service.go"), []byte("package service\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "agent", "agent.go"), []byte("package agent\n"), 0644))

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	existingProfile := &domain.ProjectProfile{
		ProjectName:  "test-project",
		Language:     "go",
		Architecture: "Clean Architecture",
		KeyModules:   []domain.ModuleInfo{{Name: "service", Path: "internal/service"}},
	}

	_, err := svc.AnalyzeProjectFullWithOptions(context.Background(), tmpDir, "test-project", "go", AnalyzeProjectOptions{
		ExistingProfile: existingProfile,
		FocusPaths:      []string{filepath.Join(tmpDir, "internal", "service")},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"internal/service"}, received.FocusPaths)
	assert.Contains(t, received.ExistingProfileJSON, `"architecture": "Clean Architecture"`)
	assert.Contains(t, received.Structure, "Focused scan paths")
	assert.Contains(t, received.Structure, "internal/service")
}

func TestAnalyzeProjectFullWithOptions_FocusedStructureOmitsUnfocusedTree(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "agent"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "service.go"), []byte("package service\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "agent", "agent.go"), []byte("package agent\n"), 0644))

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.AnalyzeProjectFullWithOptions(context.Background(), tmpDir, "test-project", "go", AnalyzeProjectOptions{
		FocusPaths: []string{filepath.Join(tmpDir, "internal", "service")},
	})

	require.NoError(t, err)
	assert.Contains(t, received.Structure, "Focused scan paths")
	assert.Contains(t, received.Structure, "internal/service")
	assert.NotContains(t, received.Structure, "internal/agent")
	assert.NotContains(t, received.Structure, "Project structure:")
}

func TestAnalyzeCodebaseFullWithOptions_FocusedStructureOmitsUnfocusedTree(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "agent"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "service.go"), []byte("package service\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "agent", "agent.go"), []byte("package agent\n"), 0644))

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	_, _, err := svc.AnalyzeCodebaseFullWithOptions(context.Background(), tmpDir, "test-project", "go", AnalyzeCodebaseOptions{
		FocusPaths: []string{filepath.Join(tmpDir, "internal", "service")},
	})

	require.NoError(t, err)
	assert.Contains(t, received.Structure, "Focused scan paths")
	assert.Contains(t, received.Structure, "internal/service")
	assert.NotContains(t, received.Structure, "internal/agent")
	assert.NotContains(t, received.Structure, "Project structure:")
}

func TestAnalyzeProjectFullWithOptions_PassesRuntimeUserContext(t *testing.T) {
	tmpDir := t.TempDir()

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeProjectFn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	ctx := runtimecontext.WithUserContext(context.Background(), "私有化 HSM 工作区，交付物是离线安装包。")

	_, err := svc.AnalyzeProjectFullWithOptions(ctx, tmpDir, "test-project", "go", AnalyzeProjectOptions{})

	require.NoError(t, err)
	assert.Equal(t, "私有化 HSM 工作区，交付物是离线安装包。", received.UserContext)
}

func TestAnalyzeCodebaseFullWithOptions_PassesRuntimeUserContext(t *testing.T) {
	tmpDir := t.TempDir()

	var received agent.AnalyzeCurrentCodebaseRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			received = *req
			return &agent.AnalyzeCurrentCodebaseResult{}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	ctx := runtimecontext.WithUserContext(context.Background(), "hsmwebapi 是管理 API，core-engine 是核心能力库。")

	_, _, err := svc.AnalyzeCodebaseFullWithOptions(ctx, tmpDir, "test-project", "go", AnalyzeCodebaseOptions{})

	require.NoError(t, err)
	assert.Equal(t, "hsmwebapi 是管理 API，core-engine 是核心能力库。", received.UserContext)
}

type fakeStructuralCollector struct {
	context string
	err     error
}

func (f fakeStructuralCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	return f.context, f.err
}

type recordingStructuralCollector struct {
	context string
	err     error
	req     structuralContextRequest
}

func (r *recordingStructuralCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	r.req = req
	return r.context, r.err
}
