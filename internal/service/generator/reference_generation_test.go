package generator

import (
	"context"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

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
	svc := newGeneratorService(patternRepo, mockProfile, loader, cfg)
	outputPath := t.TempDir()

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))

	missingPath := filepath.Join(outputPath, "references", "patterns", "error.md")
	require.NoError(t, os.Remove(missingPath))

	require.NoError(t, svc.GenerateSkills(ctx, outputPath))
	require.FileExists(t, missingPath)
}

func TestGenerateSkillsWithHooksSkipsReferencesAndReferenceLinks(t *testing.T) {
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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)
	tmpDir := t.TempDir()

	err := svc.GenerateSkillsWithHooks(context.Background(), tmpDir, GenerateProgressHooks{}, GenerateOptions{SkipReferences: true})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(tmpDir, "SKILL.md"))
	require.NoDirExists(t, filepath.Join(tmpDir, "references"))
	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, "本次生成未写入 references")
	assert.NotContains(t, skill, "./references/")
	assertNoBrokenMarkdownLinks(t, tmpDir)
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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)

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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "references"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("<!-- generated-by: skills-seed v0.13.9 -->\n# Old generated skill\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "references", "project-overview.md"), []byte("old overview"), 0644))

	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "project-overview.md"))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "Profile-backed project overview")
	assert.NotContains(t, string(content), "old overview")
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

func TestGenerateSkills_RendersRelatedReferencesForCategoryBoundaries(t *testing.T) {
	businessPattern := domain.NewPattern("operate-log", "Operate Log Domain Rule", domain.CategoryBusiness)
	businessPattern.Confidence = 0.9
	businessPattern.SetDescription("操作日志保存审计域规则")
	businessPattern.SetRule("记录操作日志时保持审计语义完整")

	apiPattern := domain.NewPattern("operation-extra", "Operation Log Response Extra", domain.CategoryAPI)
	apiPattern.Confidence = 0.92
	apiPattern.SetDescription("接口响应 Extra 字段承载操作日志 Before/After 数据")
	apiPattern.SetRule("响应 Extra 使用当前 API 映射约定填充")

	databasePattern := domain.NewPattern("audit-save", "Audit Persistence", domain.CategoryDatabase)
	databasePattern.Confidence = 0.88
	databasePattern.SetDescription("审计记录需要持久化")
	databasePattern.SetRule("持久化审计记录时复用仓储层约定")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*businessPattern, *apiPattern, *databasePattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, "进入任何分类模式后，先检查其中的“相关参考”")

	businessIndex := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.Contains(t, businessIndex, "## 相关参考")
	assert.Contains(t, businessIndex, "[API 模式](./api.md)")
	assert.Contains(t, businessIndex, "Extra/Before/After")
	assert.Contains(t, businessIndex, "[数据库模式](./database.md)")
	assert.NotContains(t, businessIndex, "./error.md")

	businessDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "operate-log-domain-rule.md")
	assert.Contains(t, businessDetail, "## 相关参考")
	assert.Contains(t, businessDetail, "[API 模式](../api.md)")
	assert.Contains(t, businessDetail, "接口返回")

	api := readGeneratedFile(t, tmpDir, "references", "patterns", "api.md")
	assert.Contains(t, api, "## 相关参考")
	assert.Contains(t, api, "[业务模式](./business.md)")
	assert.Contains(t, api, "操作日志 Extra")
	assert.NotContains(t, api, "./middleware.md")

	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateSkills_RendersEnglishRelatedReferences(t *testing.T) {
	apiPattern := domain.NewPattern("api-extra", "API Extra", domain.CategoryAPI)
	apiPattern.Confidence = 0.9
	apiPattern.SetDescription("API returns Extra payloads")
	apiPattern.SetRule("Populate Extra from mapped response models")

	businessPattern := domain.NewPattern("audit-domain", "Audit Domain", domain.CategoryBusiness)
	businessPattern.Confidence = 0.9
	businessPattern.SetDescription("Audit rules are domain behavior")
	businessPattern.SetRule("Preserve audit semantics")

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*apiPattern, *businessPattern}, nil
		},
	}
	loader := skills.NewLoader("en-US")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, `check its "Related References" section first`)

	api := readGeneratedFile(t, tmpDir, "references", "patterns", "api.md")
	assert.Contains(t, api, "## Related References")
	assert.Contains(t, api, "[Business Patterns](./business.md)")
	assert.Contains(t, api, "operation-log Extra")

	businessDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "audit-domain.md")
	assert.Contains(t, businessDetail, "## Related References")
	assert.Contains(t, businessDetail, "[API Patterns](../api.md)")

	assertNoBrokenMarkdownLinks(t, tmpDir)
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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)

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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, "description: 修改、调试、测试、审查或扩展 hsmwebapi go 代码时使用")
	assert.NotContains(t, skill, "平均置信度")
	assert.NotContains(t, skill, "91.60000000000001")

	businessPattern := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.NotContains(t, businessPattern, "## 业务方法汇总")
	assert.NotContains(t, businessPattern, "DuplicatedMethod")

	businessDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "business-flow.md")
	assert.Contains(t, businessDetail, "Business Flow")
	assert.NotContains(t, businessDetail, "DuplicatedMethod")

	businessMethods := readGeneratedFile(t, tmpDir, "references", "business-methods.md")
	assert.Contains(t, businessMethods, "DuplicatedMethod")

	modules := readGeneratedFile(t, tmpDir, "references", "modules.md")
	assert.Contains(t, modules, "已验证入口")
	assert.NotContains(t, modules, "TODO: 需读取插件内部代码确认")

	assertNoExcessiveBlankLines(t, tmpDir)
}

func TestGenerateSkills_RendersEvidenceScopedGuidance(t *testing.T) {
	apiPattern := domain.NewPattern("api", "JZero Code Generation Convention", domain.CategoryAPI)
	apiPattern.Confidence = 0.95
	apiPattern.Frequency = 2
	apiPattern.SetDescription("jzero generates handlers and types from .api files")
	apiPattern.SetRule("Do not hand-edit generated handlers or types")
	apiPattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/gen/api.go", Line: 10, Symbol: "GenerateAPI", Kind: "function", Confidence: 0.9}}
	businessPattern := domain.NewPattern("business", "Plan Lifecycle State Machine", domain.CategoryBusiness)
	businessPattern.Confidence = 0.95
	businessPattern.Frequency = 2
	businessPattern.SetDescription("Only one plan can be active")
	businessPattern.SetRule("Deactivate existing active plans before creating or activating a plan")
	businessPattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/application/vocab/service.go", Line: 1, Symbol: "ActivatePlan", Kind: "method", Confidence: 0.9}}
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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.NotContains(t, skill, "## 常用工作流")
	assert.NotContains(t, skill, "新增或调整 API")
	assert.NotContains(t, skill, "修改业务流程")
	assert.NotContains(t, skill, "接入或调整外部依赖")
	assert.Contains(t, skill, "[验证策略](./references/validation.md)")
	assert.NotContains(t, skill, "## 验证命令")
	assert.NotContains(t, skill, "`jzero gen`")
	assert.NotContains(t, skill, "`go test ./...`")
	assert.NotContains(t, skill, "`go test ./internal/application/vocab`")
	assert.Contains(t, skill, "只有用户维护或内置的显式规则可视为硬约束")
	assert.Contains(t, skill, "## 模式参考")
	validation := readGeneratedFile(t, tmpDir, "references", "validation.md")
	assert.NotContains(t, validation, "## 范围矩阵")
	assert.Contains(t, validation, "`task verify` - 业务逻辑变更后运行（来源：`Taskfile.yml`）")

	spec := readGeneratedFile(t, tmpDir, "references", "project-spec.md")
	assert.NotContains(t, spec, "## 修改来源")
	assert.NotContains(t, spec, "## 参考观察")
	assert.Contains(t, spec, "[验证策略](./validation.md)")
	assert.NotContains(t, spec, "## 验证命令")
	assert.Contains(t, spec, "jzero gen generates handlers, types, routes, and swagger from desc/api .api files")
	assert.NotContains(t, spec, "Only one plan can be active")
	assert.NotContains(t, spec, "Do not hand-edit generated handlers or types")
	assert.NotContains(t, spec, "Deactivate existing active plans before creating or activating a plan")

	overview := readGeneratedFile(t, tmpDir, "references", "project-overview.md")
	assert.NotContains(t, overview, "## 验证命令")

	api := readGeneratedFile(t, tmpDir, "references", "patterns", "api.md")
	assert.Contains(t, api, "## 用户规则与可复用解决方案")
	assert.Contains(t, api, "### 可复用解决方案")
	assert.Contains(t, api, "jzero generates handlers and types from .api files")
	assert.NotContains(t, api, "Do not hand-edit generated handlers or types")
	assert.Contains(t, api, "使用：命中相同需求时先检查并优先复用这些入口")

	businessIndex := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.NotContains(t, businessIndex, "## 规则断言")
	assert.Contains(t, businessIndex, "## 子域路由")
	businessDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "vocab.md")
	assert.Contains(t, businessDetail, "适用与实现观察（需按源码复核）")
	assert.Contains(t, businessDetail, "Only one plan can be active")
	assert.NotContains(t, businessDetail, "复用建议（需按源码复核）")
	assert.NotContains(t, businessDetail, "Deactivate existing active plans before creating or activating a plan")
	assert.Contains(t, businessDetail, "ActivatePlan")
}

func TestGenerateSkills_DisambiguatesDuplicateModuleHeadings(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
			pattern.Confidence = 0.9
			pattern.Frequency = 2
			return []domain.Pattern{*pattern}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "demo",
				Language:    "go",
				KeyModules: []domain.ModuleInfo{
					{Name: "home", Path: "internal/handler/home", Description: "handler layer"},
					{Name: "home", Path: "internal/logic/home", Description: "logic layer"},
				},
			}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{ProjectCfg: config.ProjectConfig{Name: "demo", Language: "go"}}
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	modules := readGeneratedFile(t, tmpDir, "references", "modules.md")
	assert.NotContains(t, modules, "## home\n")
	assert.Contains(t, modules, "## home (handler/home)")
	assert.Contains(t, modules, "## home (logic/home)")
	assert.Contains(t, modules, "**源码路径**: `internal/handler/home`")
	assert.Contains(t, modules, "职责观察（需按源码复核）")
}

func TestGenerateSkills_FiltersMissingEvidencePaths(t *testing.T) {
	existingPath := "internal/service/existing.go"
	missingPath := "internal/logic/missing.go"
	pattern := domain.NewPattern("p1", "Node Metrics", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Collects metrics")
	pattern.SetRule("Use existing metrics flow")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: existingPath, Line: 12, Symbol: "Existing", Kind: "function", Confidence: 0.9},
		{Path: missingPath, Line: 33, Symbol: "Missing", Kind: "function", Confidence: 0.9},
	}
	pattern.SetBusinessMethod(&domain.BusinessMethod{
		Name:         "Metrics",
		CodeLocation: domain.CodeLocation{CurrentLocation: missingPath + ":33"},
		Description:  "missing metrics method",
		Function:     "func Metrics()",
		Type:         "domain",
	})
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "internal/service"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, existingPath), []byte("package service\n\nfunc Existing() {}\n"), 0644))
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "backend",
				Language:    "go",
				BusinessMethods: []domain.BusinessMethod{{
					Name:         "Metrics",
					CodeLocation: domain.CodeLocation{CurrentLocation: missingPath + ":33"},
					Description:  "missing metrics method",
					Function:     "func Metrics()",
					Type:         "domain",
				}},
				KeyModules: []domain.ModuleInfo{{
					Name: "missing",
					Path: missingPath,
				}},
			}, nil
		},
	}
	svc := newGeneratorService(mockPattern, mockProfile, skills.NewLoader("en-US"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "backend", Language: "go", RootPath: projectRoot},
	})
	tmpDir := filepath.Join(projectRoot, ".agents", "skills", "backend-dev")

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	businessIndex := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	require.Contains(t, businessIndex, existingPath+":3")
	require.NotContains(t, businessIndex, missingPath)

	require.NoFileExists(t, filepath.Join(tmpDir, "references", "business-methods.md"))

	spec := readGeneratedFile(t, tmpDir, "references", "project-spec.md")
	require.NotContains(t, spec, missingPath)
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
	svc := newGeneratorService(mockPattern, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "backend"},
	})
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.NotContains(t, skill, "## 验证命令")
	assert.Contains(t, skill, "[验证策略](./references/validation.md)")
	validation := readGeneratedFile(t, tmpDir, "references", "validation.md")
	assert.Contains(t, validation, "未学习到有仓库证据的验证命令")
	overview := readGeneratedFile(t, tmpDir, "references", "project-overview.md")
	assert.NotContains(t, overview, "## 验证命令")
}

func TestGenerateSkills_RendersLocalMultiModuleTestMatrix(t *testing.T) {
	projectRoot := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":                    "module example/root\n",
		"root_test.go":              "package root\n",
		"plugins/a/go.mod":          "module example/a\n",
		"plugins/a/a_test.go":       "package a\n",
		"plugins/without/go.mod":    "module example/without\n",
		"plugins/a/child/b_test.go": "package child\n",
	} {
		fullPath := filepath.Join(projectRoot, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}
	mockPattern := &mocks.MockPatternRepository{GetAllFn: func(context.Context) ([]domain.Pattern, error) {
		return []domain.Pattern{*domain.NewPattern("p1", "Business Rule", domain.CategoryBusiness)}, nil
	}}
	mockProfile := &mocks.MockProjectProfileRepository{GetFn: func(context.Context) (*domain.ProjectProfile, error) {
		return &domain.ProjectProfile{ProjectName: "backend", Language: "go"}, nil
	}}
	svc := newGeneratorService(mockPattern, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "backend", Language: "go", RootPath: projectRoot},
	})
	outputPath := filepath.Join(projectRoot, ".agents", "skills", "backend-dev")

	require.NoError(t, svc.GenerateSkills(context.Background(), outputPath))

	skill := readGeneratedFile(t, outputPath, "SKILL.md")
	require.Contains(t, skill, "[测试矩阵](./references/testing.md)")
	require.NotContains(t, skill, "plugins/a/a_test.go")
	testingReference := readGeneratedFile(t, outputPath, "references", "testing.md")
	require.Contains(t, testingReference, "| `.` | `go test ./...` | 1 | `go.mod` |")
	require.Contains(t, testingReference, "| `plugins/a` | `go test ./...` | 2 | `plugins/a/go.mod` |")
	require.Contains(t, testingReference, "`plugins/a/child/b_test.go`")
	require.Contains(t, testingReference, "模块 `plugins/without`")
	require.Contains(t, testingReference, "| `plugins/without` | `go test ./...` | 0 | `plugins/without/go.mod` |")
	assertNoBrokenMarkdownLinks(t, outputPath)
}
