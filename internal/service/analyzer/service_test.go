package analyzer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
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

func TestAnalyzeProjectProfile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "helper.go"), []byte("package helper\n\nfunc BuildResponse(value any) error { return nil }\n"), 0o644))
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return &agent.AnalyzeProjectResult{
				Language:     "go",
				Frameworks:   []string{"gin"},
				Architecture: "DDD",
				CommonUtils: []domain.UtilityFunction{
					{Name: "BuildResponse", File: "helper.go:99", Signature: "func BuildResponse(value any) error", Description: "AI summary"},
					{Name: "SuccessResp", File: "helper.go:3", Signature: "func SuccessResp()"},
				},
				ValidationCommands: []domain.ValidationCommand{{Command: "task verify", When: "after changes", Source: "Taskfile.yml"}},
				Summary:            "Test project summary",
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	result, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName:     "test",
		RootPath:        tmpDir,
		LearningSession: session,
	})
	require.NoError(t, err)
	assert.Equal(t, "go", result.Language)
	assert.Contains(t, result.Frameworks, "gin")
	require.Len(t, result.CommonUtils, 1)
	assert.Equal(t, "BuildResponse", result.CommonUtils[0].Name)
	assert.Equal(t, "helper.go:3", result.CommonUtils[0].File)
	assert.Empty(t, result.CommonUtils[0].Description)
	require.Len(t, result.ValidationCommands, 1)
	assert.Equal(t, "task verify", result.ValidationCommands[0].Command)
}

func TestAnalyzeProjectProfileUsesLearningSession(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		result: &agent.AnalyzeProjectResult{
			Language: "go",
			Summary:  "session profile",
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	result, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName:     "test",
		RootPath:        tmpDir,
		LearningSession: session,
	})

	require.NoError(t, err)
	require.True(t, session.called)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "session profile", result.Summary)
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

func TestAnalyzeProjectProfileAddsStructuralContext(t *testing.T) {
	tmpDir := t.TempDir()
	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
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

	_, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName:     "test",
		RootPath:        tmpDir,
		Language:        "go",
		MainFiles:       []string{"main.go"},
		LearningSession: session,
	})

	require.NoError(t, err)
	require.Contains(t, received.StructuralContext, "main calls service")
}

func TestAnalyzeProjectProfileCollectsEngineeringKnowledgeOutsideFocus(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("go test ./..."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Taskfile.yml"), []byte("version: '3'"), 0o644))

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{
				Language: "go",
				EngineeringRules: []domain.EngineeringRule{{
					Title:    "Validation",
					Rule:     "Run go test ./...",
					Source:   "AGENTS.md",
					Evidence: []string{"AGENTS.md"},
				}},
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	result, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName:     "test",
		RootPath:        tmpDir,
		FocusPaths:      []string{"internal/service"},
		LearningSession: session,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"AGENTS.md", "Taskfile.yml"}, received.EngineeringKnowledge)
	require.Len(t, result.EngineeringRules, 1)
	require.Equal(t, "AGENTS.md", result.EngineeringRules[0].Source)
}

func TestAnalyzeProjectProfileSkipsStructuralContextWithoutSeeds(t *testing.T) {
	tmpDir := t.TempDir()
	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
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

	_, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName:     "test",
		RootPath:        tmpDir,
		Language:        "go",
		LearningSession: session,
	})

	require.NoError(t, err)
	require.Empty(t, received.StructuralContext)
}

func TestAnalyzeProjectProfileSkipsUnavailableOptionalStructuralContext(t *testing.T) {
	tmpDir := t.TempDir()
	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
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

	_, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName:     "test",
		RootPath:        tmpDir,
		Language:        "go",
		MainFiles:       []string{"main.go"},
		LearningSession: session,
	})

	require.NoError(t, err)
	require.Empty(t, received.StructuralContext)
}

func TestSelectLearningCandidatesUsesStructuralSeedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	var agentReq agent.SelectLearningCandidatesRequest
	var structuralReqs []structuralContextRequest
	var stages []SelectLearningCandidatesStage
	session := &profileRefreshTestSession{
		selectFn: func(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
			agentReq = *req
			return &agent.SelectLearningCandidatesResult{
				SelectedPaths: []string{"internal/auth/service.go"},
				Reason:        "service entry is enough",
			}, nil
		},
	}
	svc := NewAnalyzerService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, &mocks.MockConfigReader{
		LearningCfg: config.LearningConfig{
			Current: config.CurrentLearningConfig{
				Structural: config.StructuralConfig{Enabled: true},
			},
		},
	})
	svc.structuralCollector = recordingStructuralCollector{
		context:  "## Structural Context\n- service routes auth",
		requests: &structuralReqs,
	}

	result, err := svc.SelectLearningCandidates(context.Background(), &SelectLearningCandidatesRequest{
		ProjectName:         "test",
		RootPath:            tmpDir,
		Language:            "go",
		CandidatePaths:      []string{"internal/auth/service.go", "internal/auth/types.go"},
		StructuralSeedPaths: []string{"internal/auth/service.go"},
		Progress: func(stage SelectLearningCandidatesStage) {
			stages = append(stages, stage)
		},
		LearningSession: session,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"internal/auth/service.go"}, result.SelectedPaths)
	require.Len(t, structuralReqs, 1)
	require.Equal(t, []string{"internal/auth/service.go"}, structuralReqs[0].SeedPaths)
	require.Equal(t, []string{"internal/auth/service.go"}, structuralReqs[0].FocusPaths)
	require.Contains(t, agentReq.StructuralContext, "service routes auth")
	require.Equal(t, []SelectLearningCandidatesStage{
		SelectLearningCandidatesStageStructuralContext,
		SelectLearningCandidatesStageAgent,
	}, stages)
}

func TestSelectLearningCandidatesSkipsStructuralContextWithoutSeeds(t *testing.T) {
	tmpDir := t.TempDir()
	var agentReq agent.SelectLearningCandidatesRequest
	var structuralReqs []structuralContextRequest
	session := &profileRefreshTestSession{
		selectFn: func(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
			agentReq = *req
			return &agent.SelectLearningCandidatesResult{
				SelectedPaths: req.CandidatePaths,
				Reason:        "no structural seed needed",
			}, nil
		},
	}
	svc := NewAnalyzerService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, &mocks.MockConfigReader{
		LearningCfg: config.LearningConfig{
			Current: config.CurrentLearningConfig{
				Structural: config.StructuralConfig{Enabled: true},
			},
		},
	})
	svc.structuralCollector = recordingStructuralCollector{
		context:  "## Structural Context\n- should not be collected",
		requests: &structuralReqs,
	}

	result, err := svc.SelectLearningCandidates(context.Background(), &SelectLearningCandidatesRequest{
		ProjectName:     "test",
		RootPath:        tmpDir,
		Language:        "go",
		CandidatePaths:  []string{"a.go", "b.go"},
		LearningSession: session,
	})

	require.NoError(t, err)
	require.Equal(t, []string{"a.go", "b.go"}, result.SelectedPaths)
	require.Empty(t, structuralReqs)
	require.Empty(t, agentReq.StructuralContext)
}

func TestAnalyzeProjectProfile_AIError(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return nil, errors.New("AI error")
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	_, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{LearningSession: session})
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

func TestCollectSampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "user.go"), []byte("package service"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFilesFromRoots(tmpDir, nil, "go")
	assert.NotEmpty(t, files)
	assertSamplePathsContain(t, files, "main.go", "internal/service/user.go", "main_test.go")
}

func TestCollectSampleFiles_ReturnsPathsWithoutEmbeddingContent(t *testing.T) {
	tmpDir := t.TempDir()
	longUTF8Content := "package main\n\n// " + strings.Repeat("创建SSH会话", 400)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "webshell.go"), []byte(longUTF8Content), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFilesFromRoots(tmpDir, nil, "go")

	require.Len(t, files, 1)
	assert.Equal(t, "webshell.go", files[0].Path)
}

func TestCollectSampleFiles_DoesNotTreatVendorAsKeyword(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "vendor", "pkg"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vendor", "pkg", "lib.go"), []byte("package pkg"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFilesFromRoots(tmpDir, nil, "go")
	assertSamplePathsContain(t, files, "main.go", "vendor/pkg/lib.go")
}

func TestCollectSampleFilesKeepsSourceFilesUnderDocs(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "docs", "examples"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "examples", "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "Guide.MD"), []byte("# guide"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFilesFromRoots(tmpDir, nil, "go")

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

	files := svc.collectSampleFilesFromRoots(tmpDir, nil, "go")
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

func TestRefreshProjectProfile_WithMock(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pkg", "response.go"), []byte("package pkg\n\nfunc Response(value any) error { return nil }\n"), 0o644))
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return &agent.AnalyzeProjectResult{
				Language:       "go",
				Frameworks:     []string{"gin", "gorm"},
				Architecture:   "Clean Architecture",
				Dependencies:   []string{"github.com/gin-gonic/gin"},
				Summary:        "A test project",
				KeyModules:     []domain.ModuleInfo{{Name: "handler", Path: "internal/handler"}},
				CommonUtils:    []domain.UtilityFunction{{Name: "Response", File: "pkg/response.go", Signature: "func Response(value any) error"}},
				ConfigPatterns: []string{"YAML config"},
			}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profile, err := svc.RefreshProjectProfile(ctx, tmpDir, "test-project", "", AnalyzeProjectOptions{
		LearningSession: session,
	})
	require.NoError(t, err)
	assert.Equal(t, "go", profile.Language)
	assert.Contains(t, profile.Frameworks, "gin")
	assert.Contains(t, profile.Frameworks, "gorm")
	assert.NotEmpty(t, profile.KeyModules)
	assert.NotEmpty(t, profile.CommonUtils)
}

func TestRefreshProjectProfile_PassesReadmePathWithoutContent(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# secret readme content"), 0644))

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.RefreshProjectProfile(context.Background(), tmpDir, "test-project", "", AnalyzeProjectOptions{
		LearningSession: session,
	})

	require.NoError(t, err)
	assert.Equal(t, "README.md", received.ReadmePath)
}

func TestBuildProjectProfileResult_PassesIncrementalProfileContext(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "agent"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "service.go"), []byte("package service\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "agent", "agent.go"), []byte("package agent\n"), 0644))

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
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

	_, err := svc.buildProjectProfileResult(context.Background(), tmpDir, "test-project", "go", AnalyzeProjectOptions{
		ExistingProfile: existingProfile,
		FocusPaths:      []string{filepath.Join(tmpDir, "internal", "service")},
		LearningSession: session,
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"internal/service"}, received.FocusPaths)
	assert.Contains(t, received.ExistingProfileJSON, `"architecture": "Clean Architecture"`)
	assert.Contains(t, received.Structure, "Focused scan paths")
	assert.Contains(t, received.Structure, "internal/service")
}

func TestBuildProjectProfileResult_FocusedStructureOmitsUnfocusedTree(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "agent"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "service.go"), []byte("package service\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "agent", "agent.go"), []byte("package agent\n"), 0644))

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.buildProjectProfileResult(context.Background(), tmpDir, "test-project", "go", AnalyzeProjectOptions{
		FocusPaths:      []string{filepath.Join(tmpDir, "internal", "service")},
		LearningSession: session,
	})

	require.NoError(t, err)
	assert.Contains(t, received.Structure, "Focused scan paths")
	assert.Contains(t, received.Structure, "internal/service")
	assert.NotContains(t, received.Structure, "internal/agent")
	assert.NotContains(t, received.Structure, "Project structure:")
}

func TestBuildProjectProfileResult_PassesRuntimeUserContext(t *testing.T) {
	tmpDir := t.TempDir()

	var received agent.AnalyzeProjectRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			received = *req
			return &agent.AnalyzeProjectResult{Language: "go"}, nil
		},
	}
	svc := NewAnalyzerService(mockAgent, nil)
	ctx := runtimecontext.WithUserContext(context.Background(), "私有化 HSM 工作区，交付物是离线安装包。")

	_, err := svc.buildProjectProfileResult(ctx, tmpDir, "test-project", "go", AnalyzeProjectOptions{
		LearningSession: session,
	})

	require.NoError(t, err)
	assert.Equal(t, "私有化 HSM 工作区，交付物是离线安装包。", received.UserContext)
}

type fakeStructuralCollector struct {
	context string
	err     error
}

type recordingStructuralCollector struct {
	context  string
	err      error
	requests *[]structuralContextRequest
}

type profileRefreshTestSession struct {
	called   bool
	result   *agent.AnalyzeProjectResult
	fn       func(context.Context, *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error)
	selectFn func(context.Context, *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error)
}

func (s *profileRefreshTestSession) SessionID() string {
	return "profile-refresh-test"
}

func (s *profileRefreshTestSession) SelectLearningCandidates(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
	if s.selectFn != nil {
		return s.selectFn(ctx, req)
	}
	return &agent.SelectLearningCandidatesResult{}, nil
}

func (s *profileRefreshTestSession) PlanLearningAgenda(context.Context, *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	return &agent.PlanLearningAgendaResult{}, nil
}

func (s *profileRefreshTestSession) AnalyzeCurrentCodebaseBatch(context.Context, *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	return &agent.AnalyzeCurrentCodebaseBatchResult{}, nil
}

func (s *profileRefreshTestSession) AnalyzeCurrentDeltaBatch(context.Context, *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	return &agent.AnalyzeCurrentDeltaBatchResult{}, nil
}

func (s *profileRefreshTestSession) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	s.called = true
	if s.fn != nil {
		return s.fn(ctx, req)
	}
	return s.result, nil
}

func (s *profileRefreshTestSession) Close(context.Context) error {
	return nil
}

func (f fakeStructuralCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	return f.context, f.err
}

func (f recordingStructuralCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	if f.requests != nil {
		*f.requests = append(*f.requests, req)
	}
	return f.context, f.err
}
