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
				Language:     "go",
				Frameworks:   []string{"gin"},
				Architecture: "DDD",
				Summary:      "Test project summary",
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
}

func TestAnalyzeProjectAddsCodeGraphContext(t *testing.T) {
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
		AnalysisCfg: config.AnalysisConfig{
			CodeGraph: config.CodeGraphConfig{
				Enabled:  true,
				Required: false,
				Command:  "codegraph",
			},
		},
	})
	svc.codeGraphCollector = fakeCodeGraphCollector{
		context: "## CodeGraph Context\n- main calls service",
	}

	_, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
	})

	require.NoError(t, err)
	require.Contains(t, received.StructuralContext, "main calls service")
}

func TestAnalyzeProjectSkipsUnavailableOptionalCodeGraph(t *testing.T) {
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
		AnalysisCfg: config.AnalysisConfig{
			CodeGraph: config.CodeGraphConfig{
				Enabled:  true,
				Required: false,
				Command:  "missing-codegraph",
			},
		},
	})
	svc.codeGraphCollector = fakeCodeGraphCollector{
		err: errCodeGraphUnavailable,
	}

	_, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
	})

	require.NoError(t, err)
	require.Empty(t, received.StructuralContext)
}

func TestAnalyzeProjectFailsWhenRequiredCodeGraphUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		AnalysisCfg: config.AnalysisConfig{
			CodeGraph: config.CodeGraphConfig{
				Enabled:  true,
				Required: true,
				Command:  "missing-codegraph",
			},
		},
	})
	svc.codeGraphCollector = fakeCodeGraphCollector{
		err: errCodeGraphUnavailable,
	}

	_, err := svc.AnalyzeProject(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "CodeGraph")
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

func TestAnalyzeCurrentCodebase(t *testing.T) {
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		AnalyzeCurrentCodebaseFn: func(ctx context.Context, req *agent.AnalyzeCurrentCodebaseRequest) (*agent.AnalyzeCurrentCodebaseResult, error) {
			return &agent.AnalyzeCurrentCodebaseResult{
				Patterns: []domain.Pattern{
					*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
				},
				BusinessRules:  []string{"Always wrap errors"},
				BestPractices:  []string{"Use structured logging"},
				CommonPatterns: []string{"Repository pattern"},
				Summary:        "Test codebase summary",
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
	assert.Contains(t, result.BusinessRules, "Always wrap errors")
}

func TestAnalyzeCurrentCodebaseAddsCodeGraphContext(t *testing.T) {
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
		AnalysisCfg: config.AnalysisConfig{
			CodeGraph: config.CodeGraphConfig{
				Enabled: true,
				Command: "codegraph",
			},
		},
	})
	svc.codeGraphCollector = fakeCodeGraphCollector{
		context: "## CodeGraph Context\n- service has 3 callers",
	}

	_, err := svc.AnalyzeCurrentCodebase(context.Background(), &AnalyzeCurrentCodebaseRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
	})

	require.NoError(t, err)
	require.Contains(t, received.StructuralContext, "service has 3 callers")
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
				Summary: "Extracted patterns",
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

func TestAnalyzeCodebaseFullPassesKnownPatterns(t *testing.T) {
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
	require.Equal(t, `[{"id":"known"}]`, received.KnownPatternsJSON)
	require.Equal(t, 1, received.KnownPatternsCount)
}

func TestCollectSampleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "service"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "service", "user.go"), []byte("package service"), 0644))
	// 测试文件应被排除。
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFiles(tmpDir, "go")
	assert.NotEmpty(t, files)
	// 结果不应包含测试文件。
	for _, f := range files {
		assert.NotContains(t, f.Path, "_test.go")
	}
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

func TestCollectSampleFiles_ExcludeVendor(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "vendor", "pkg"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "vendor", "pkg", "lib.go"), []byte("package pkg"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	svc := &AnalyzerService{}
	files := svc.collectSampleFiles(tmpDir, "go")
	for _, f := range files {
		assert.NotContains(t, f.Path, "vendor")
	}
}

func TestCollectSampleFiles_UsesConfiguredExclude(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "generated"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "generated", "wire.go"), []byte("package generated"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644))

	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Locale: "zh-CN", Language: "go"},
		AgentCfg:   config.AgentConfig{Provider: "test"},
		Exclude:    []string{"internal/generated/**"},
	})

	files := svc.collectSampleFiles(tmpDir, "go")
	require.NotEmpty(t, files)
	for _, f := range files {
		assert.NotContains(t, f.Path, "internal/generated")
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

type fakeCodeGraphCollector struct {
	context string
	err     error
}

func (f fakeCodeGraphCollector) Collect(ctx context.Context, projectRoot string, req codeGraphContextRequest) (string, error) {
	return f.context, f.err
}
