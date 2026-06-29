package generator

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	workflowstore "github.com/silaswei-io/skills-seed/internal/infra/storage/workflow"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	workflowsvc "github.com/silaswei-io/skills-seed/internal/service/workflow"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(mockPattern *mocks.MockPatternRepository) *GeneratorService {
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	return NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
}

func TestGenerateSkills_NoPatterns(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)
}

func TestGenerateSkills_RepoError(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return nil, errors.New("db error")
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.Error(t, err)
}

func TestGenerateSkillsDoesNotCallAISummary(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
	}
	patterns[0].Confidence = 0.9

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return patterns, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)
}

func TestGenerateSkillsRendersEvidenceLocationsWithoutAISummary(t *testing.T) {
	pattern := domain.NewPattern("error-wrap", "Error Wrap", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap config load errors")
	pattern.SetRule("Wrap config errors with path context")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/config.go", Line: 42, Symbol: "LoadConfig", Kind: "function", Description: "wraps config errors", Confidence: 0.88},
	}

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))
	content := readGeneratedFile(t, tmpDir, "references", "patterns", "error.md")
	require.Contains(t, content, "Error Wrap")
	require.Contains(t, content, "Wrap config errors with path context")
}

func TestGenerateSkillsRendersBusinessPatternMapFromEvidenceLocations(t *testing.T) {
	pattern := domain.NewPattern("order-state-transition", "Order State Transition", domain.CategoryBusiness)
	pattern.Confidence = 0.87
	pattern.Frequency = 2
	pattern.SetDescription("订单状态流转必须经过订单服务的状态校验。")
	pattern.SetRule("新增订单状态或调整订单流程时，先检查 ApplyOrderTransition 的校验和错误分支。")
	pattern.SetExamples("func (s *OrderService) ApplyOrderTransition(ctx context.Context, orderID string) error {\n\torder, err := s.repo.Get(ctx, orderID)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn s.validateTransition(order)\n}", "")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{
			Path:        "internal/service/order/transition.go",
			Line:        42,
			Symbol:      "ApplyOrderTransition",
			Kind:        "method",
			Description: "订单状态流转入口",
			Confidence:  0.91,
		},
	}

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	index := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	require.Contains(t, index, "业务模式地图")
	require.Contains(t, index, "Order")
	require.Contains(t, index, "`internal/service/order/transition.go:42`")
	require.Contains(t, index, "新需求定位方式")

	detail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "order.md")
	require.Contains(t, detail, "需求匹配")
	require.Contains(t, detail, "代码证据入口")
	require.Contains(t, detail, "ApplyOrderTransition")
	require.Contains(t, detail, "订单状态流转入口")
}

func TestGenerateSkills_FillsMissingCategorySummaries(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	pattern.SetExamples("return fmt.Errorf(\"load: %w\", err)", "return err")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "references", "patterns", "error.md"))
	assert.NoError(t, err)
}

func TestResolveProjectOutputPathRejectsPathsOutsideProjectRoot(t *testing.T) {
	parent := t.TempDir()
	projectRoot := filepath.Join(parent, "repo")
	require.NoError(t, os.MkdirAll(projectRoot, 0755))

	inside, err := resolveProjectOutputPath(projectRoot, ".agents/skills/demo")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(projectRoot, ".agents", "skills", "demo"), inside)

	_, err = resolveProjectOutputPath(projectRoot, "../outside")
	require.Error(t, err)
}

func TestGenerateSkillsWithProgressReportsProjectSteps(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
			}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)

	var started []string
	var completed []string
	err := svc.GenerateSkillsWithProgress(context.Background(), t.TempDir(), func(label string) {
		started = append(started, label)
	}, func(label string) {
		completed = append(completed, label)
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"解析输出目录",
		"加载已学习模式",
		"读取项目画像",
		"检查输出目录",
		"整理生成数据",
		"写入技能文件",
	}, started)
	require.Equal(t, started, completed)
}

func TestGenerateSkillsRebuildsWhenInputFingerprintUnchanged(t *testing.T) {
	ctx := context.Background()
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")

	dbPath := filepath.Join(t.TempDir(), "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()
	require.NoError(t, patternRepo.Save(ctx, pattern))

	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
			}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(patternRepo, mockProfile, loader, cfg)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.FileExists(t, filepath.Join(outputPath, "SKILL.md"))
}

func TestGenerateSkillsDoesNotSkipWhenWorkflowScriptChanges(t *testing.T) {
	ctx := context.Background()
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")

	seedPath := t.TempDir()
	dbPath := filepath.Join(seedPath, "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()
	require.NoError(t, patternRepo.Save(ctx, pattern))

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
	}
	workflowRepo := workflowstore.NewRepository(seedPath)
	workflowSvc := workflowsvc.NewService(workflowRepo, mockAgent, "go")
	_, err = workflowSvc.UpsertWorkflow(ctx, workflowsvc.UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量",
	})
	require.NoError(t, err)
	scriptPath := filepath.Join(seedPath, "workflows", "deploy", "scripts", "smoke-test.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho first\n"), 0755))
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
			}, nil
		},
	}
	svc := NewGeneratorService(patternRepo, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	})
	svc.SetWorkflowRepository(workflowRepo)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho second\n"), 0755))
	require.NoError(t, svc.GenerateSkills(ctx, outputPath))

	generatedScript := readGeneratedFile(t, outputPath, "scripts", "workflows", "deploy", "smoke-test.sh")
	require.Contains(t, generatedScript, "echo second")
}

func TestGenerateSkillsRemovesDeletedWorkflowOutputs(t *testing.T) {
	ctx := context.Background()
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")

	seedPath := t.TempDir()
	dbPath := filepath.Join(seedPath, "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()
	require.NoError(t, patternRepo.Save(ctx, pattern))
	workflowRepo := workflowstore.NewRepository(seedPath)
	workflowSvc := workflowsvc.NewService(workflowRepo, &mocks.MockAgent{NameVal: "test", AvailableVal: true}, "go")
	_, err = workflowSvc.UpsertWorkflow(ctx, workflowsvc.UpsertRequest{Name: "deploy", Context: "发布前检查环境变量"})
	require.NoError(t, err)
	_, err = workflowSvc.UpsertWorkflow(ctx, workflowsvc.UpsertRequest{Name: "test", Context: "发布后执行 smoke test"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "workflows", "test", "scripts", "smoke-test.sh"), []byte("#!/bin/sh\necho ok\n"), 0755))

	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{ProjectName: "test", Language: "go", Summary: "demo"}, nil
		},
	}
	svc := NewGeneratorService(patternRepo, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	})
	svc.SetWorkflowRepository(workflowRepo)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.FileExists(t, filepath.Join(outputPath, "workflows", "deploy.md"))
	require.FileExists(t, filepath.Join(outputPath, "workflows", "test.md"))
	require.FileExists(t, filepath.Join(outputPath, "scripts", "workflows", "test", "smoke-test.sh"))

	require.NoError(t, os.RemoveAll(filepath.Join(seedPath, "workflows", "test")))
	require.NoError(t, svc.GenerateSkills(ctx, outputPath))

	require.FileExists(t, filepath.Join(outputPath, "workflows", "deploy.md"))
	require.NoFileExists(t, filepath.Join(outputPath, "workflows", "test.md"))
	require.NoDirExists(t, filepath.Join(outputPath, "scripts", "workflows", "test"))
	skill := readGeneratedFile(t, outputPath, "SKILL.md")
	require.Contains(t, skill, "./workflows/deploy.md")
	require.NotContains(t, skill, "./workflows/test.md")
}

func TestGenerateSkillsDoesNotSkipWhenReferenceOutputIsIncomplete(t *testing.T) {
	ctx := context.Background()
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")

	dbPath := filepath.Join(t.TempDir(), "project.db")
	patternRepo, err := boltdb.NewPatternRepository(dbPath)
	require.NoError(t, err)
	defer patternRepo.Close()
	require.NoError(t, patternRepo.Save(ctx, pattern))
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
			}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(patternRepo, mockProfile, loader, cfg)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))

	missingPath := filepath.Join(outputPath, "references", "patterns", "error.md")
	require.NoError(t, os.Remove(missingPath))

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.FileExists(t, missingPath)
}

func TestGenerateSkillsWithOptionsSkipsReferencesAndReferenceLinks(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Rule", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Use existing business rule")
	pattern.SetRule("Reuse the documented flow")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
				KeyModules: []domain.ModuleInfo{{
					Name:        "vocab",
					Path:        "internal/application/vocab",
					Description: "vocabulary business service",
				}},
				BusinessMethods: []domain.BusinessMethod{{
					Name:         "ActivatePlan",
					CodeLocation: domain.CodeLocation{CurrentLocation: "internal/application/vocab/service.go:1"},
					Description:  "activates a plan",
					Function:     "func ActivatePlan()",
					Usage:        "plan activation",
					Type:         "domain",
				}},
			}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)
	tmpDir := t.TempDir()

	err := svc.GenerateSkillsWithOptions(context.Background(), tmpDir, GenerateOptions{SkipReferences: true})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(tmpDir, "SKILL.md"))
	require.NoDirExists(t, filepath.Join(tmpDir, "references"))
	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, "本次生成未写入 references")
	assert.NotContains(t, skill, "./references/")
	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateSkillsWritesUserWorkflowsAndScripts(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{ProjectName: "test", Language: "go", Summary: "demo"}, nil
		},
	}
	seedPath := t.TempDir()
	workflowRepo := workflowstore.NewRepository(seedPath)
	workflowSvc := workflowsvc.NewService(workflowRepo, &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		OptimizeWorkflowFn: func(ctx context.Context, req *agent.OptimizeWorkflowRequest) (*agent.OptimizeWorkflowResult, error) {
			return &agent.OptimizeWorkflowResult{
				Title:   req.Name,
				Content: "# " + req.Name + "\n\n## 适用场景\n发布流程覆盖上线前环境与产物核验，以及上线后的冒烟验证。\n",
			}, nil
		},
	}, "go")
	_, err := workflowSvc.UpsertWorkflow(context.Background(), workflowsvc.UpsertRequest{
		Name:    "deploy",
		Context: "发布前检查环境变量和构建产物，发布后执行 smoke test",
	})
	require.NoError(t, err)
	scriptPath := filepath.Join(seedPath, "workflows", "deploy", "scripts", "smoke-test.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho ok\n"), 0755))

	svc := NewGeneratorService(mockPattern, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	})
	svc.SetWorkflowRepository(workflowRepo)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), outputPath))

	skill := readGeneratedFile(t, outputPath, "SKILL.md")
	require.Contains(t, skill, "## 用户工作流")
	require.Contains(t, skill, "./workflows/deploy.md")
	require.Contains(t, skill, "发布流程覆盖上线前环境与产物核验，以及上线后的冒烟验证。")
	require.NotContains(t, skill, "发布前检查环境变量和构建产物，发布后执行 smoke test")
	workflowContent := readGeneratedFile(t, outputPath, "workflows", "deploy.md")
	require.Contains(t, workflowContent, "## 适用场景")
	require.Contains(t, workflowContent, "发布流程覆盖上线前环境与产物核验")
	require.FileExists(t, filepath.Join(outputPath, "scripts", "workflows", "deploy", "smoke-test.sh"))
	assertNoBrokenMarkdownLinks(t, outputPath)
}

func TestGenerateSkillsRebuildsGeneratedOutputWithoutReadingExistingSkill(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	pattern.SetExamples("const secretGoodExample = true", "const secretBadExample = true")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("<!-- generated-by: skills-seed v0.0.4 -->\nconst secretExistingSkillContent = true"), 0644))

	err := svc.GenerateSkills(context.Background(), tmpDir)

	require.NoError(t, err)
	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.NotContains(t, skill, "secretExistingSkillContent")
	assert.NotContains(t, skill, "secretGoodExample")
	assert.NotContains(t, skill, "secretBadExample")
}

func TestGenerateSkillsDeterministicSummaryUsesMetricsAndHitStatsInRankedOrder(t *testing.T) {
	generic := domain.NewPattern("generic", "Generic Rule", domain.CategoryError)
	generic.Confidence = 0.95
	generic.Frequency = 10
	generic.Metrics = domain.PatternMetrics{
		SpecificityScore: 0.2,
		EvidenceCount:    1,
		GenericPenalty:   0.6,
		EffectiveScore:   0.25,
	}
	generic.SetDescription("Generic error handling guidance")
	generic.SetRule("Handle errors consistently")

	specific := domain.NewPattern("specific", "KMC Auth Boundary", domain.CategoryBusiness)
	specific.Confidence = 0.8
	specific.Frequency = 1
	specific.Metrics = domain.PatternMetrics{
		SpecificityScore: 0.9,
		EvidenceCount:    5,
		GenericPenalty:   0,
		EffectiveScore:   0.88,
	}
	specific.SetDescription("Use internal/service/auth/session.go when validating KMC admin sessions")
	specific.SetRule("Validate KMC admin sessions through SessionService before controller mutations")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*generic, *specific}, nil
		},
		GetPatternHitStatsFn: func(ctx context.Context) ([]domain.PatternHitStats, error) {
			return []domain.PatternHitStats{
				{Pattern: *generic, HitCount: 0},
				{Pattern: *specific, HitCount: 4},
			}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	business := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	errorPatterns := readGeneratedFile(t, tmpDir, "references", "patterns", "error.md")
	assert.Contains(t, business, "KMC Auth Boundary")
	assert.Contains(t, errorPatterns, "Generic Rule")
	spec := readGeneratedFile(t, tmpDir, "references", "project-spec.md")
	assert.Less(t, strings.Index(spec, "KMC Auth Boundary"), strings.Index(spec, "Generic Rule"))
}

func TestRankPatternsForGenerationUsesEffectiveScoreHitsAndConfidence(t *testing.T) {
	lowValue := domain.NewPattern("low", "Low Value", domain.CategoryError)
	lowValue.Confidence = 0.95
	lowValue.Metrics = domain.PatternMetrics{EffectiveScore: 0.2}

	highValue := domain.NewPattern("high", "High Value", domain.CategoryBusiness)
	highValue.Confidence = 0.8
	highValue.Metrics = domain.PatternMetrics{EffectiveScore: 0.85}

	input := []domain.Pattern{*lowValue, *highValue}
	insights := map[string]domain.PatternInsight{
		"low":  {HitCount: 0},
		"high": {HitCount: 3},
	}

	ranked := rankPatternsForGeneration(input, insights)

	require.Len(t, ranked, 2)
	assert.Equal(t, "high", ranked[0].ID)
	assert.Equal(t, "low", ranked[1].ID)
	assert.Equal(t, "low", input[0].ID)
	assert.Equal(t, 0.89, insights["high"].GenerationRank)
	assert.Equal(t, 0.22, insights["low"].GenerationRank)
}

func TestGenerateSkillsDoesNotUseRuntimeUserContextDuringGenerate(t *testing.T) {
	pattern := domain.NewPattern("p1", "HSM Delivery Boundary", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("HSM 私有化交付边界")
	pattern.SetRule("保留用户说明中的交付边界")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsmwebapi", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
	ctx := runtimecontext.WithUserContext(context.Background(), "hsmwebapi 是管理 API；交付物是离线安装包，不是 SaaS。")

	require.NoError(t, svc.GenerateSkills(ctx, t.TempDir()))

}

func TestGenerateSkillsUsesDeterministicSummary(t *testing.T) {
	pattern := domain.NewPattern("p1", "HSM Delivery Boundary", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("HSM 私有化交付边界")
	pattern.SetRule("保留用户说明中的交付边界")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsmwebapi", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
	ctx := runtimecontext.WithUserContext(context.Background(), "hsmwebapi 是管理 API；交付物是离线安装包，不是 SaaS。")

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(ctx, tmpDir))

	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business.md"), "business 分类包含 1 个项目特定模式")
	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business", "hsm-delivery-boundary.md"), "HSM Delivery Boundary")
}

func TestGenerateSkillsDoesNotCallAgentSummary(t *testing.T) {
	pattern := domain.NewPattern("p1", "AI Rule", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("ai mode rule")
	pattern.SetRule("ask agent for category summary")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business.md"), "business 分类包含 1 个项目特定模式")
	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business", "ai.md"), "AI Rule")
}

func TestGenerateSkills_DoesNotOverwriteManualSkill(t *testing.T) {
	pattern := domain.NewPattern("p1", "Manual Rule", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("manual skill rule")
	pattern.SetRule("do not overwrite manual skill")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("# Manual Skill\n"), 0644))

	err := svc.GenerateSkills(context.Background(), tmpDir)

	require.Error(t, err)
	var manualErr *ManualSkillExistsError
	require.ErrorAs(t, err, &manualErr)
	assert.Equal(t, filepath.Join(tmpDir, "SKILL.md"), manualErr.Path)
	assert.Equal(t, "# Manual Skill\n", readGeneratedFile(t, tmpDir, "SKILL.md"))
	require.NoFileExists(t, filepath.Join(tmpDir, "references", "project-spec.md"))
}

func TestGenerateSkills_OverwritesGeneratedSkill(t *testing.T) {
	pattern := domain.NewPattern("p1", "Generated Rule", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("generated skill rule")
	pattern.SetRule("overwrite generated skill")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("<!-- generated-by: skills-seed v0.0.4 -->\nold generated skill\n"), 0644))

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	businessPattern := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.Contains(t, businessPattern, "./business/generated.md")
	assert.NotContains(t, businessPattern, "#### ✅ 代码证据")
	assert.NotContains(t, businessPattern, "```go")
	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business", "generated.md"), "Generated Rule")
	assert.NotContains(t, readGeneratedFile(t, tmpDir, "SKILL.md"), "old generated skill")
}

func TestGenerateSkills_RequiresProjectProfile(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return nil, profilestore.ErrProfileNotFound
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)

	err := svc.GenerateSkills(context.Background(), t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "项目画像")
}

func TestGenerateSkills_RendersProjectOverviewFromProfile(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
			}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "references"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "references", "project-overview.md"), []byte("old overview"), 0644))

	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "project-overview.md"))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "Profile-backed project overview")
	assert.NotContains(t, string(content), "old overview")
}

func TestGenerateSkills_ProjectOverviewDoesNotPromoteUnitSummaryToProjectFact(t *testing.T) {
	pattern := domain.NewPattern("p1", "Home Pattern", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName:  "hsmwebapi",
				Language:     "go",
				Summary:      "home-info单元使用go-zero框架实现首页仪表盘功能",
				Architecture: "home-info单元采用典型的go-zero分层架构",
				GeneratedAt:  "2026-05-19 12:00:00",
				KeyModules: []domain.ModuleInfo{
					{Name: "home", Path: "internal/logic/home"},
					{Name: "key-manage", Path: "plugins/key_manage"},
				},
			}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsmwebapi", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "project-overview.md"))
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "## 项目概览摘要")
	require.Contains(t, text, "当前已学习到 2 个模块/业务域")
	require.Contains(t, text, "home")
	require.Contains(t, text, "key-manage")
	require.NotContains(t, text, "## 项目概览摘要\n\nhome-info单元")
	require.NotContains(t, text, "## 技术架构\n\nhome-info单元")
}

func TestGenerateSkills_MainSkillReferencesOnlyGeneratedCategories(t *testing.T) {
	businessPattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	businessPattern.Confidence = 0.9
	businessPattern.SetDescription("Use the project business flow")
	businessPattern.SetRule("Reuse the existing business flow")

	databasePattern := domain.NewPattern("p2", "Repository Query", domain.CategoryDatabase)
	databasePattern.Confidence = 0.8
	databasePattern.SetDescription("Use repository query helpers")
	databasePattern.SetRule("Keep database access in repository code")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*businessPattern, *databasePattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	content, err := os.ReadFile(filepath.Join(tmpDir, "SKILL.md"))
	require.NoError(t, err)
	skill := string(content)

	assert.Contains(t, skill, "./references/patterns/business.md")
	assert.Contains(t, skill, "./references/patterns/database.md")
	assert.NotContains(t, skill, "./references/examples/")

	for _, missingCategory := range []string{"naming", "error", "utils", "testing", "api", "middleware", "config", "structure", "concurrency"} {
		assert.NotContains(t, skill, "./references/patterns/"+missingCategory+".md")
	}
}

func TestGenerateSkills_SplitsProfileReferences(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName:  "test",
				Language:     "go",
				Summary:      "Profile-backed project overview",
				Architecture: "Layered service",
				GeneratedAt:  "2026-05-19 12:00:00",
				KeyModules: []domain.ModuleInfo{
					{
						Name:             "certificate-service",
						Path:             "internal/service/certificate",
						Description:      "certificate business orchestration",
						Responsibilities: []string{"issue certificates"},
						Dependencies:     []string{"domain", "repository"},
						Dependents:       []string{"api"},
						KeyMethods:       []string{"IssueCertificate"},
					},
				},
				BusinessMethods: []domain.BusinessMethod{
					{
						Name:          "IssueCertificate",
						CodeLocation:  domain.CodeLocation{CurrentLocation: "internal/service/certificate/service.go:42"},
						Description:   "issues a certificate after validating request state",
						Function:      "func (s *Service) IssueCertificate(ctx context.Context, req *IssueRequest) error",
						Usage:         "certificate issuance flow",
						Type:          "domain",
						Prerequisites: "initialized service and valid request",
						Returns:       "error when validation or persistence fails",
					},
				},
				CommonUtils: []domain.UtilityFunction{
					{
						Name:        "NormalizeSerial",
						File:        "internal/pkg/serial.go",
						Signature:   "func NormalizeSerial(serial string) string",
						Description: "normalizes certificate serial values",
						Usage:       "before serial comparison",
					},
				},
			}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	overview, err := os.ReadFile(filepath.Join(tmpDir, "references", "project-overview.md"))
	require.NoError(t, err)
	overviewText := string(overview)
	assert.Contains(t, overviewText, "./business-methods.md")
	assert.Contains(t, overviewText, "./modules.md")
	assert.Contains(t, overviewText, "./common-utils.md")
	assert.NotContains(t, overviewText, "func (s *Service) IssueCertificate(ctx context.Context, req *IssueRequest) error")

	businessMethods, err := os.ReadFile(filepath.Join(tmpDir, "references", "business-methods.md"))
	require.NoError(t, err)
	assert.Contains(t, string(businessMethods), "IssueCertificate")
	assert.Contains(t, string(businessMethods), "- **位置状态**: `valid`")
	assert.Contains(t, string(businessMethods), "func (s *Service) IssueCertificate(ctx context.Context, req *IssueRequest) error")
	assert.Contains(t, string(businessMethods), "./patterns/business.md")

	modules, err := os.ReadFile(filepath.Join(tmpDir, "references", "modules.md"))
	require.NoError(t, err)
	assert.Contains(t, string(modules), "certificate-service")
	assert.Contains(t, string(modules), "internal/service/certificate")

	commonUtils, err := os.ReadFile(filepath.Join(tmpDir, "references", "common-utils.md"))
	require.NoError(t, err)
	assert.Contains(t, string(commonUtils), "NormalizeSerial")
	assert.Contains(t, string(commonUtils), "func NormalizeSerial(serial string) string")
	assert.NotContains(t, string(commonUtils), "./patterns/utils.md")

	skill, err := os.ReadFile(filepath.Join(tmpDir, "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(skill), "./references/business-methods.md")
	assert.Contains(t, string(skill), "./references/modules.md")
	assert.Contains(t, string(skill), "./references/common-utils.md")

	_, err = os.Stat(filepath.Join(tmpDir, "references", "examples"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateSkills_HidesCommonUtilsCoveredByBusinessPatterns(t *testing.T) {
	pattern := domain.NewPattern("sm4-gcm-encrypt-decrypt", "SM4-GCM加解密", domain.CategoryBusiness)
	pattern.Confidence = 0.92
	pattern.SetDescription("使用国密SM4算法的GCM模式进行对称加密和解密")
	pattern.SetRule("复用已学习的SM4-GCM加解密规则")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "pkg/crypto/sm4gcm/sm4gcm.go",
		Symbol: "Encrypt",
		Kind:   "function",
	}}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "profile",
				CommonUtils: []domain.UtilityFunction{
					{
						Name:        "SM4Encrypt",
						File:        "pkg/crypto/sm4gcm/sm4gcm.go",
						Signature:   "func SM4Encrypt(key []byte, plaintext []byte) ([]byte, error)",
						Description: "encrypts data",
					},
					{
						Name:        "FormatFileSize",
						File:        "pkg/utils/utils.go",
						Signature:   "func FormatFileSize(sizeBytes int) string",
						Description: "formats file size",
					},
				},
			}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	commonUtils, err := os.ReadFile(filepath.Join(tmpDir, "references", "common-utils.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(commonUtils), "SM4Encrypt")
	assert.Contains(t, string(commonUtils), "FormatFileSize")
	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateSkills_RendersCompactActionableSkillReferences(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.916
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "hsmwebapi",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-21 12:00:00",
				KeyModules: []domain.ModuleInfo{
					{
						Name:       "plugins/key_manage",
						Path:       "plugins/key_manage",
						KeyMethods: []string{"TODO: 需读取插件内部代码确认", "RegisterKeyManage"},
					},
				},
				BusinessMethods: []domain.BusinessMethod{
					{
						Name:         "DuplicatedMethod",
						CodeLocation: domain.CodeLocation{CurrentLocation: "internal/service/demo.go:10"},
						Description:  "method documented once in the split reference",
						Usage:        "business flow reuse",
						Type:         "domain",
						Function:     "func DuplicatedMethod() error",
					},
				},
			}, nil
		},
	}

	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsmwebapi", Language: "go"},
		AgentCfg:   config.AgentConfig{Engine: "claude"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, "description: 修改、审查或扩展 hsmwebapi go 代码且涉及")
	assert.Contains(t, skill, "**平均置信度**: 91.60%")
	assert.NotContains(t, skill, "91.60000000000001")

	businessPattern := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.NotContains(t, businessPattern, "## 业务方法汇总")
	assert.NotContains(t, businessPattern, "DuplicatedMethod")

	businessDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "other.md")
	assert.Contains(t, businessDetail, "Business Flow")
	assert.NotContains(t, businessDetail, "DuplicatedMethod")

	businessMethods := readGeneratedFile(t, tmpDir, "references", "business-methods.md")
	assert.Contains(t, businessMethods, "DuplicatedMethod")

	modules := readGeneratedFile(t, tmpDir, "references", "modules.md")
	assert.Contains(t, modules, "RegisterKeyManage")
	assert.NotContains(t, modules, "TODO: 需读取插件内部代码确认")

	assertNoExcessiveBlankLines(t, tmpDir)
}

func TestGenerateSkills_RendersEvidenceScopedGuidance(t *testing.T) {
	apiPattern := domain.NewPattern("api", "JZero Code Generation Convention", domain.CategoryAPI)
	apiPattern.Confidence = 0.95
	apiPattern.SetDescription("jzero generates handlers and types from .api files")
	apiPattern.SetRule("Do not hand-edit generated handlers or types")
	businessPattern := domain.NewPattern("business", "Plan Lifecycle State Machine", domain.CategoryBusiness)
	businessPattern.Confidence = 0.95
	businessPattern.SetDescription("Only one plan can be active")
	businessPattern.SetRule("Deactivate existing active plans before creating or activating a plan")
	concurrencyPattern := domain.NewPattern("concurrency", "Mutex Guarded In-Memory Store", domain.CategoryConcurrency)
	concurrencyPattern.Confidence = 0.95
	concurrencyPattern.SetDescription("The service uses mutex protected maps")
	concurrencyPattern.SetRule("Public methods lock and Locked helpers assume caller holds lock")
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*apiPattern, *businessPattern, *concurrencyPattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName:       "backend",
				Language:          "go",
				Summary:           "Word Weave backend",
				Frameworks:        []string{"jzero", "go-zero", "gRPC"},
				FrameworkPatterns: []string{"jzero gen generates handlers, types, routes, and swagger from desc/api .api files"},
				ConfigPatterns:    []string{"YAML config with hot reload"},
				KeyModules: []domain.ModuleInfo{
					{Name: "vocab", Path: "internal/application/vocab", Description: "business service"},
					{Name: "mapper", Path: "internal/logic/mapper", Description: "API response mapping"},
					{Name: "rpcclient", Path: "internal/rpcclient", Description: "typed gRPC wrappers"},
				},
				BusinessMethods: []domain.BusinessMethod{{
					Name:         "ActivatePlan",
					CodeLocation: domain.CodeLocation{CurrentLocation: "internal/application/vocab/service.go:1"},
					Description:  "activates a plan",
					Function:     "func ActivatePlan()",
					Usage:        "plan activation",
					Type:         "domain",
				}},
				ValidationCommands: []domain.ValidationCommand{
					{Command: "task verify", When: "业务逻辑变更后运行", Source: "Taskfile.yml"},
				},
			}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "backend", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, "## 常用工作流")
	assert.Contains(t, skill, "新增或调整 API")
	assert.Contains(t, skill, "修改业务流程")
	assert.Contains(t, skill, "接入或调整外部依赖")
	assert.Contains(t, skill, "## 验证命令")
	assert.Contains(t, skill, "`task verify` - 业务逻辑变更后运行（来源：`Taskfile.yml`）")
	assert.NotContains(t, skill, "`jzero gen`")
	assert.NotContains(t, skill, "`go test ./...`")
	assert.NotContains(t, skill, "`go test ./internal/application/vocab`")
	assert.Contains(t, skill, "只有带代码证据的 pattern 规则可视为硬约束")
	assert.Contains(t, skill, "## 模式参考")

	spec := readGeneratedFile(t, tmpDir, "references", "project-spec.md")
	assert.NotContains(t, spec, "## 修改来源")
	assert.Contains(t, spec, "## 参考观察")
	assert.Contains(t, spec, "## 验证命令")
	assert.Contains(t, spec, "`task verify` - 业务逻辑变更后运行（来源：`Taskfile.yml`）")
	assert.Contains(t, spec, "以下内容来自已学习模式，但缺少足够直接的代码证据")
	assert.Contains(t, spec, "Do not hand-edit generated handlers or types")
	assert.Contains(t, spec, "Deactivate existing active plans before creating or activating a plan")

	overview := readGeneratedFile(t, tmpDir, "references", "project-overview.md")
	assert.Contains(t, overview, "## 验证命令")
	assert.Contains(t, overview, "`task verify` - 业务逻辑变更后运行（来源：`Taskfile.yml`）")
}

func TestGenerateSkills_OmitsValidationCommandsWhenNotLearned(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*domain.NewPattern("p1", "Business Rule", domain.CategoryBusiness)}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "backend",
				Language:    "unknown",
				Summary:     "Profile-backed project overview",
			}, nil
		},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "backend"},
	})
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.NotContains(t, skill, "## 验证命令")
	overview := readGeneratedFile(t, tmpDir, "references", "project-overview.md")
	assert.NotContains(t, overview, "## 验证命令")
}

func TestGenerateSkills_SkipsEmptyBusinessMethodDetails(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")
	pattern.SetBusinessMethod(&domain.BusinessMethod{})
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "patterns", "business", "other.md"))
	require.NoError(t, err)
	businessPattern := string(content)

	assert.NotContains(t, businessPattern, "业务方法详情")
	assert.NotContains(t, businessPattern, "| **方法** | `` |")
	assert.NotContains(t, businessPattern, "```go\n\n```")
}

func TestGenerateSkills_RendersBusinessIndexAndDomainDetails(t *testing.T) {
	patterns := []domain.Pattern{
		businessPatternWithLocation("billing-activation", "Activation Rule", "Activation deactivates existing active entities", "Always deactivate existing entities before activation", "func (s *Service) Activate(planID int64) (Plan, error) {\n\treturn Plan{}, nil\n}", "internal/application/billing/service.go:10"),
		businessPatternWithLocation("billing-renewal", "Renewal Rule", "Renewal keeps billing state consistent", "Renew only after validating billing state", "func (s *Service) Renew(planID int64) error {\n\treturn nil\n}", "internal/application/billing/renewal.go:12"),
		businessPatternWithLocation("notification-delivery", "Delivery Rule", "Delivery records notification attempts", "Record delivery attempts before retry", "func (s *Service) Deliver(id int64) error {\n\treturn nil\n}", "internal/application/notification/delivery.go:20"),
	}

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return patterns, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				GeneratedAt: "2026-05-19 12:00:00",
				KeyModules: []domain.ModuleInfo{{
					Name:        "vocab",
					Path:        "internal/application/vocab",
					Description: "vocabulary business service",
				}},
				BusinessMethods: []domain.BusinessMethod{{
					Name:         "ActivatePlan",
					CodeLocation: domain.CodeLocation{CurrentLocation: "internal/application/vocab/service.go:1"},
					Description:  "activates a plan",
					Function:     "func ActivatePlan()",
					Usage:        "plan activation",
					Type:         "domain",
				}},
			}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, cfg)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	index := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.Contains(t, index, "业务模式地图")
	assert.Contains(t, index, "./business/billing.md")
	assert.Contains(t, index, "./business/notification.md")
	assert.NotContains(t, index, "func (s *Service)")
	assert.NotContains(t, index, "#### ✅ 代码证据")

	billingDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "billing.md")
	assert.Contains(t, billingDetail, "Activation Rule")
	assert.Contains(t, billingDetail, "Renewal Rule")
	assert.Contains(t, billingDetail, "#### ✅ 代码证据")
	assert.Contains(t, billingDetail, "Activate")

	notificationDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "notification.md")
	assert.Contains(t, notificationDetail, "Delivery Rule")

	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateSkills_CodexWritesOpenAIMetadata(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	loader := skills.NewLoaderForAgent("codex", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
		AgentCfg:   config.AgentConfig{Engine: "codex"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)

	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)

	openAIPath := filepath.Join(tmpDir, "agents", "openai.yaml")
	content, err := os.ReadFile(openAIPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "display_name")
	assert.Contains(t, string(content), "$test-dev")
}

var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

func readGeneratedFile(t *testing.T, root string, parts ...string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	require.NoError(t, err)
	return string(content)
}

func businessPattern(id, name, description, rule, example string) domain.Pattern {
	pattern := domain.NewPattern(id, name, domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.Frequency = 1
	pattern.SetDescription(description)
	pattern.SetRule(rule)
	pattern.SetExamples(example, "")
	return *pattern
}

func businessPatternWithLocation(id, name, description, rule, example, location string) domain.Pattern {
	pattern := businessPattern(id, name, description, rule, example)
	pattern.SetBusinessMethod(&domain.BusinessMethod{
		Name:         name,
		CodeLocation: domain.CodeLocation{CurrentLocation: location},
		Description:  description,
		Type:         "domain",
		Function:     strings.Split(example, "{")[0],
	})
	return pattern
}

func readFilePath(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func assertNoBrokenMarkdownLinks(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		content, err := os.ReadFile(path)
		require.NoError(t, err)

		for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(content), -1) {
			target := strings.TrimSpace(match[1])
			if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if fragmentIndex := strings.Index(target, "#"); fragmentIndex >= 0 {
				target = target[:fragmentIndex]
			}
			if target == "" {
				continue
			}

			targetPath := filepath.Clean(filepath.Join(filepath.Dir(path), target))
			_, err := os.Stat(targetPath)
			assert.NoErrorf(t, err, "broken link in %s: %s", path, match[1])
		}
		return nil
	})
	require.NoError(t, err)
}

func assertNoExcessiveBlankLines(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContainsf(t, string(content), "\n\n\n", "excessive blank lines in %s", path)
		return nil
	})
	require.NoError(t, err)
}

func assertMarkdownTableHasNoBlankLines(t *testing.T, content, heading string) {
	t.Helper()

	start := strings.Index(content, heading)
	require.NotEqual(t, -1, start)
	section := content[start+len(heading):]
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	lines := strings.Split(section, "\n")
	firstTableLine := -1
	lastTableLine := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			if firstTableLine < 0 {
				firstTableLine = i
			}
			lastTableLine = i
		}
	}
	require.GreaterOrEqual(t, firstTableLine, 0)
	require.GreaterOrEqual(t, lastTableLine, firstTableLine)
	for i := firstTableLine; i <= lastTableLine; i++ {
		require.NotEmpty(t, strings.TrimSpace(lines[i]), "blank line inside markdown table under %s", heading)
	}
}

func TestCalculateStats(t *testing.T) {
	svc := newTestService(nil)

	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
		*domain.NewPattern("p2", "Naming", domain.CategoryNaming),
		*domain.NewPattern("p3", "Database", domain.CategoryDatabase),
	}
	patterns[0].Confidence = 0.9
	patterns[0].Frequency = 5
	patterns[1].Confidence = 0.7
	patterns[1].Frequency = 2
	patterns[2].Confidence = 0.85
	patterns[2].Frequency = 4

	stats := svc.calculateStats(patterns)
	assert.Equal(t, 3, stats.Total)
	assert.InDelta(t, 0.816, stats.AvgConfidence, 0.01)
	assert.Len(t, stats.ByCategory, 3)
	assert.Len(t, stats.HighConfidence, 2) // 0.9 and 0.85
	assert.Len(t, stats.Frequent, 2)       // 5 and 4 (> 3)
}

func TestCalculateStats_Empty(t *testing.T) {
	svc := newTestService(nil)
	stats := svc.calculateStats([]domain.Pattern{})
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, 0.0, stats.AvgConfidence)
}

func TestCalculateCategoryConfidence(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "E1", domain.CategoryError),
		*domain.NewPattern("p2", "E2", domain.CategoryError),
		*domain.NewPattern("p3", "N1", domain.CategoryNaming),
	}
	patterns[0].Confidence = 0.8
	patterns[1].Confidence = 0.9
	patterns[2].Confidence = 0.7

	assert.InDelta(t, 0.85, calculateCategoryConfidence(patterns, "error"), 0.01)
	assert.InDelta(t, 0.7, calculateCategoryConfidence(patterns, "naming"), 0.01)
	assert.InDelta(t, 0.0, calculateCategoryConfidence(patterns, "testing"), 0.01)
}
