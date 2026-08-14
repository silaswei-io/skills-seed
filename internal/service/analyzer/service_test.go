package analyzer

import (
	"context"
	"errors"
	"fmt"
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

	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	svc := NewAnalyzerService(mockAgent, &mocks.MockConfigReader{
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

func TestAnalyzeProjectProfileKeepsProjectMapSeparateFromCapabilities(t *testing.T) {
	tmpDir := t.TempDir()
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		fn: func(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
			return &agent.AnalyzeProjectResult{
				Language:     "go",
				Frameworks:   []string{"gin"},
				Architecture: "DDD",
				Summary:      "Test project summary",
			}, nil
		},
	}
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	svc := NewAnalyzerService(mockAgent, nil)
	result, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
	})
	require.NoError(t, err)
	assert.Equal(t, "go", result.Language)
	assert.Contains(t, result.Frameworks, "gin")
}

func TestAnalyzeProjectProfileUsesAgent(t *testing.T) {
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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	svc := NewAnalyzerService(mockAgent, nil)

	result, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
	})

	require.NoError(t, err)
	require.True(t, session.called)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "session profile", result.Summary)
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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
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
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
		MainFiles:   []string{"main.go"},
	})

	require.NoError(t, err)
	require.Contains(t, received.StructuralContext, "main calls service")
}

func TestAnalyzeProjectProfileCollectsEngineeringKnowledgeOutsideFocus(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("go test ./..."), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Taskfile.yml"), []byte("version: '3'"), 0o644))

	var received agent.ExtractAuthorityRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		result: &agent.AnalyzeProjectResult{Language: "go"},
		authorityFn: func(ctx context.Context, req *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
			received = *req
			require.Len(t, req.AuthoritySections, 2)
			return &agent.ExtractAuthorityResult{
				AuthoritySections: []agent.AuthoritySectionResult{
					{
						SectionID: req.AuthoritySections[0].ID,
						Rules: []domain.EngineeringRule{{
							Title: "Validation",
							Rule:  "Run the project validation suite.",
						}},
					},
					{SectionID: req.AuthoritySections[1].ID, NoRuleReason: "No project constraint was identified in this source."},
				},
			}, nil
		},
	}
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	mockAgent.ExtractAuthorityFn = session.ExtractAuthority
	svc := NewAnalyzerService(mockAgent, nil)

	result, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{
		ProjectName: "test",
		RootPath:    tmpDir,
		FocusPaths:  []string{"internal/service"},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"AGENTS.md", "Taskfile.yml"}, received.EngineeringKnowledge)
	require.Equal(t, "AGENTS.md", received.AuthoritySections[0].Source)
	require.Equal(t, "Taskfile.yml", received.AuthoritySections[1].Source)
	require.Len(t, result.EngineeringRules, 1)
	require.Equal(t, "AGENTS.md", result.EngineeringRules[0].Source)
	require.Equal(t, []domain.AuthorityCoverage{
		{Source: "AGENTS.md"},
		{Source: "Taskfile.yml"},
	}, result.AuthorityCoverage)
	require.NotEmpty(t, result.AuthorityRevision)
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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
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
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
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
		ProjectName: "test",
		RootPath:    tmpDir,
		Language:    "go",
		MainFiles:   []string{"main.go"},
	})

	require.NoError(t, err)
	require.Empty(t, received.StructuralContext)
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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	svc := NewAnalyzerService(mockAgent, nil)
	_, err := svc.analyzeProjectProfile(context.Background(), &AnalyzeProjectRequest{})
	assert.Error(t, err)
}

func TestValidateLearningAgendaCoverageAcceptsFocusAndSkipReceipt(t *testing.T) {
	err := validateLearningAgendaCoverage(
		[]string{"src/entry.ext", "src/support.ext"},
		[]domain.EvidenceFocus{{EntryPaths: []string{"src/entry.ext"}}},
		[]agent.LearningPathSkip{{Path: "src/support.ext", Reason: "No durable decision value."}},
	)

	require.NoError(t, err)
}

func TestPlanningSourceFactsExposeDensityAndBoundSymbolPreview(t *testing.T) {
	root := t.TempDir()
	maxSymbols := 4
	var source strings.Builder
	source.WriteString("package sample\n\n")
	for index := 0; index < maxSymbols+3; index++ {
		fmt.Fprintf(&source, "func Operation%d() {}\n", index)
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "service.go"), []byte(source.String()), 0o644))

	facts := planningSourceFacts(context.Background(), root, []string{"service.go"}, maxSymbols)

	require.Len(t, facts, 1)
	require.Equal(t, "service.go", facts[0].Path)
	require.Equal(t, maxSymbols+3, facts[0].SymbolCount)
	require.Len(t, facts[0].Symbols, maxSymbols)
	require.Greater(t, facts[0].NonBlankLines, maxSymbols)
}

func TestPlanningSourceFactsSharesConfiguredSymbolBudgetAcrossFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first.go", "second.go"} {
		source := "package sample\n\nfunc First() {}\nfunc Second() {}\nfunc Third() {}\n"
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(source), 0o644))
	}

	facts := planningSourceFacts(context.Background(), root, []string{"second.go", "first.go"}, 4)

	require.Len(t, facts, 2)
	require.Len(t, facts[0].Symbols, 2)
	require.Len(t, facts[1].Symbols, 2)
	require.Equal(t, 3, facts[0].SymbolCount)
	require.Equal(t, 3, facts[1].SymbolCount)
}

func TestValidateLearningAgendaCoverageRejectsMissingDecision(t *testing.T) {
	err := validateLearningAgendaCoverage(
		[]string{"src/entry.ext", "src/missing.ext"},
		[]domain.EvidenceFocus{{EntryPaths: []string{"src/entry.ext"}}},
		nil,
	)

	require.ErrorContains(t, err, "src/missing.ext")
}

func TestDropFocusedSkipReceiptsPrefersFocusDecision(t *testing.T) {
	skipped := dropFocusedSkipReceipts(
		[]domain.EvidenceFocus{{EntryPaths: []string{"src/entry.ext"}}},
		[]agent.LearningPathSkip{
			{Path: "src/entry.ext", Reason: "Redundant receipt."},
			{Path: "src/support.ext", Reason: "No durable decision value."},
		},
	)

	require.Equal(t, []agent.LearningPathSkip{{Path: "src/support.ext", Reason: "No durable decision value."}}, skipped)
	require.NoError(t, validateLearningAgendaCoverage(
		[]string{"src/entry.ext", "src/support.ext"},
		[]domain.EvidenceFocus{{EntryPaths: []string{"src/entry.ext"}}},
		skipped,
	))
}

func TestCompleteLearningAgendaCoverageAddsOnlyOmittedInputs(t *testing.T) {
	focuses := completeLearningAgendaCoverage(
		[]string{"src/entry.ext", "src/omitted-a.ext", "src/omitted-b.ext", "src/skipped.ext"},
		[]domain.EvidenceFocus{{ID: "existing", EntryPaths: []string{"src/entry.ext"}}},
		[]agent.LearningPathSkip{{Path: "src/skipped.ext", Reason: "No durable decision value."}},
	)

	require.Len(t, focuses, 2)
	require.Equal(t, "unassigned-evidence", focuses[1].ID)
	require.Equal(t, []string{"src/omitted-a.ext", "src/omitted-b.ext"}, focuses[1].EntryPaths)
	require.Empty(t, focuses[1].RouteTerms)
	require.Empty(t, focuses[1].Attributes)
	require.Empty(t, focuses[1].RiskSignals)
	require.NoError(t, validateLearningAgendaCoverage(
		[]string{"src/entry.ext", "src/omitted-a.ext", "src/omitted-b.ext", "src/skipped.ext"}, focuses,
		[]agent.LearningPathSkip{{Path: "src/skipped.ext", Reason: "No durable decision value."}},
	))
}

func TestRestrictLearningAgendaFocusesDropsUnlistedPathsAndEmptyFocuses(t *testing.T) {
	focuses := restrictLearningAgendaFocuses(
		[]string{"src/entry.ext", "src/related.ext"},
		[]domain.EvidenceFocus{
			{ID: "kept", EntryPaths: []string{"src/entry.ext", "src"}, RelatedPaths: []string{"src/related.ext", "src/entry.ext"}},
			{ID: "dropped", EntryPaths: []string{"outside.ext"}},
		},
	)

	require.Len(t, focuses, 1)
	require.Equal(t, "kept", focuses[0].ID)
	require.Equal(t, []string{"src/entry.ext"}, focuses[0].EntryPaths)
	require.Equal(t, []string{"src/related.ext", "src/entry.ext"}, focuses[0].RelatedPaths)
}

func TestRestrictLearningAgendaSkipReceiptsDropsUnlistedPaths(t *testing.T) {
	skipped := restrictLearningAgendaSkipReceipts(
		[]string{"src/entry.ext", "src/support.ext"},
		[]agent.LearningPathSkip{
			{Path: "src/support.ext", Reason: "No durable decision value."},
			{Path: "src/context-only.ext", Reason: "Not an input file."},
		},
	)

	require.Equal(t, []agent.LearningPathSkip{{Path: "src/support.ext", Reason: "No durable decision value."}}, skipped)
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
	assertSamplePathsContain(t, files, "main.go", "internal/service/user.go")
	for _, file := range files {
		assert.NotEqual(t, "main_test.go", file.Path)
	}
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
				ConfigPatterns: []string{"YAML config"},
			}, nil
		},
	}
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	svc := NewAnalyzerService(mockAgent, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profile, err := svc.RefreshProjectProfile(ctx, tmpDir, "test-project", "", AnalyzeProjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, "go", profile.Language)
	assert.Contains(t, profile.Frameworks, "gin")
	assert.Contains(t, profile.Frameworks, "gorm")
	assert.NotEmpty(t, profile.KeyModules)
	assert.Empty(t, profile.CommonUtils)
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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.RefreshProjectProfile(context.Background(), tmpDir, "test-project", "", AnalyzeProjectOptions{})

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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
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
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	svc := NewAnalyzerService(mockAgent, nil)

	_, err := svc.buildProjectProfileResult(context.Background(), tmpDir, "test-project", "go", AnalyzeProjectOptions{
		FocusPaths: []string{filepath.Join(tmpDir, "internal", "service")},
	})

	require.NoError(t, err)
	assert.Contains(t, received.Structure, "Focused scan paths")
	assert.Contains(t, received.Structure, "internal/service")
	assert.NotContains(t, received.Structure, "internal/agent")
	assert.NotContains(t, received.Structure, "Project structure:")
}

func TestBuildProjectProfileResult_PassesRuntimeUserContext(t *testing.T) {
	tmpDir := t.TempDir()

	var received agent.ExtractAuthorityRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	session := &profileRefreshTestSession{
		result: &agent.AnalyzeProjectResult{Language: "go"},
		authorityFn: func(ctx context.Context, req *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
			received = *req
			require.Len(t, req.AuthoritySections, 1)
			return &agent.ExtractAuthorityResult{
				AuthoritySections: []agent.AuthoritySectionResult{{
					SectionID:    req.AuthoritySections[0].ID,
					NoRuleReason: "The runtime context describes the environment but contains no durable project constraint.",
				}},
			}, nil
		},
	}
	mockAgent.RefreshProjectProfileFn = session.RefreshProjectProfile
	mockAgent.ExtractAuthorityFn = session.ExtractAuthority
	svc := NewAnalyzerService(mockAgent, nil)
	ctx := runtimecontext.WithUserContext(context.Background(), "私有化 HSM 工作区，交付物是离线安装包。")

	_, err := svc.buildProjectProfileResult(ctx, tmpDir, "test-project", "go", AnalyzeProjectOptions{})

	require.NoError(t, err)
	assert.Equal(t, "私有化 HSM 工作区，交付物是离线安装包。", received.UserContext)
	assert.Equal(t, "user_context", received.AuthoritySections[0].Source)
}

type fakeStructuralCollector struct {
	context string
	err     error
}

type profileRefreshTestSession struct {
	called      bool
	result      *agent.AnalyzeProjectResult
	fn          func(context.Context, *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error)
	authorityFn func(context.Context, *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error)
}

func (s *profileRefreshTestSession) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	s.called = true
	if s.fn != nil {
		return s.fn(ctx, req)
	}
	return s.result, nil
}

func (s *profileRefreshTestSession) ExtractAuthority(ctx context.Context, req *agent.ExtractAuthorityRequest) (*agent.ExtractAuthorityResult, error) {
	if s.authorityFn != nil {
		return s.authorityFn(ctx, req)
	}
	sections := make([]agent.AuthoritySectionResult, 0, len(req.AuthoritySections))
	for _, section := range req.AuthoritySections {
		sections = append(sections, agent.AuthoritySectionResult{SectionID: section.ID, NoRuleReason: "No explicit project constraint was found."})
	}
	return &agent.ExtractAuthorityResult{AuthoritySections: sections}, nil
}

func (f fakeStructuralCollector) Collect(ctx context.Context, projectRoot string, req structuralContextRequest) (string, error) {
	return f.context, f.err
}
