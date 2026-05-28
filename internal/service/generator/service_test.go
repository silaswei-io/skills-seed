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
	profilestore "github.com/silaswei-io/skills-seed/internal/infra/storage/profile"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(mockAgent *mocks.MockAgent, mockPattern *mocks.MockPatternRepository) *GeneratorService {
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	return NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)
}

func TestGenerateSkills_NoPatterns(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{}, nil
		},
	}

	svc := newTestService(mockAgent, mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)
}

func TestGenerateSkills_RepoError(t *testing.T) {
	mockAgent := &mocks.MockAgent{NameVal: "test", AvailableVal: true}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return nil, errors.New("db error")
		},
	}

	svc := newTestService(mockAgent, mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.Error(t, err)
}

func TestGenerateSkills_AIError(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "Error Handling", domain.CategoryError),
	}
	patterns[0].Confidence = 0.9

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return nil, errors.New("AI error")
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return patterns, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AI")
}

func TestGenerateSkills_FillsMissingCategorySummaries(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	pattern.SetExamples("return fmt.Errorf(\"load: %w\", err)", "return err")

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockAgent, mockPattern)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "references", "patterns", "error.md"))
	assert.NoError(t, err)
}

func TestGenerateSkills_SummaryRequestOmitsCodeExamplesAndExistingSkillContent(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9
	pattern.SetDescription("Wrap errors with context")
	pattern.SetRule("Use fmt.Errorf with %w")
	pattern.SetExamples("const secretGoodExample = true", "const secretBadExample = true")

	var received *agent.GenerateSkillsRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			copied := *req
			received = &copied
			return &agent.GenerateSkillsResult{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("<!-- generated-by: skills-seed v0.0.4 -->\nconst secretExistingSkillContent = true"), 0644))

	err := svc.GenerateSkills(context.Background(), tmpDir)

	require.NoError(t, err)
	require.NotNil(t, received)
	assert.NotContains(t, received.PatternsJSON, "secretGoodExample")
	assert.NotContains(t, received.PatternsJSON, "secretBadExample")
	assert.Equal(t, filepath.Join(tmpDir, "SKILL.md"), received.ExistingSkillsPath)
}

func TestGenerateSkills_PassesRuntimeUserContextToAISummary(t *testing.T) {
	pattern := domain.NewPattern("p1", "HSM Delivery Boundary", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("HSM 私有化交付边界")
	pattern.SetRule("保留用户说明中的交付边界")

	var received *agent.GenerateSkillsRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			copied := *req
			received = &copied
			return &agent.GenerateSkillsResult{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsmwebapi", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)
	ctx := runtimecontext.WithUserContext(context.Background(), "hsmwebapi 是管理 API；交付物是离线安装包，不是 SaaS。")

	require.NoError(t, svc.GenerateSkills(ctx, t.TempDir()))

	require.NotNil(t, received)
	assert.Equal(t, "hsmwebapi 是管理 API；交付物是离线安装包，不是 SaaS。", received.UserContext)
}

func TestGenerateSkills_AlwaysUsesAISummary(t *testing.T) {
	pattern := domain.NewPattern("p1", "HSM Delivery Boundary", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("HSM 私有化交付边界")
	pattern.SetRule("保留用户说明中的交付边界")

	called := false
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			called = true
			require.Equal(t, "hsmwebapi 是管理 API；交付物是离线安装包，不是 SaaS。", req.UserContext)
			return &agent.GenerateSkillsResult{
				CategorySummaries: map[string]agent.CategorySummary{
					"business": {
						Category: "business",
						Summary:  "AI 根据用户说明生成的业务边界摘要",
						Patterns: []string{"HSM Delivery Boundary"},
					},
				},
			}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsmwebapi", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)
	ctx := runtimecontext.WithUserContext(context.Background(), "hsmwebapi 是管理 API；交付物是离线安装包，不是 SaaS。")

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(ctx, tmpDir))

	require.True(t, called)
	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business.md"), "AI 根据用户说明生成的业务边界摘要")
}

func TestGenerateSkills_AIModeCallsAgentSummary(t *testing.T) {
	pattern := domain.NewPattern("p1", "AI Rule", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("ai mode rule")
	pattern.SetRule("ask agent for category summary")

	called := false
	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			called = true
			return &agent.GenerateSkillsResult{
				CategorySummaries: map[string]agent.CategorySummary{
					"business": {
						Category: "business",
						Summary:  "AI generated business summary",
						Patterns: []string{"AI Rule"},
					},
				},
			}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	loader := skills.NewLoader("zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	assert.True(t, called)
	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business.md"), "AI generated business summary")
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
	svc := newTestService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, mockPattern)
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
	svc := newTestService(&mocks.MockAgent{NameVal: "test", AvailableVal: true}, mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("<!-- generated-by: skills-seed v0.0.4 -->\nold generated skill\n"), 0644))

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	businessPattern := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.Contains(t, businessPattern, "Generated Rule")
	assert.NotContains(t, readGeneratedFile(t, tmpDir, "SKILL.md"), "old generated skill")
}

func TestGenerateSkills_RequiresProjectProfile(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{}, nil
		},
	}
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
	svc := NewGeneratorService(mockPattern, mockProfile, loader, mockAgent, cfg)

	err := svc.GenerateSkills(context.Background(), t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "项目画像")
}

func TestGenerateSkills_RendersProjectOverviewFromProfile(t *testing.T) {
	pattern := domain.NewPattern("p1", "Error Wrapping", domain.CategoryError)
	pattern.Confidence = 0.9

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{}, nil
		},
	}
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
	svc := NewGeneratorService(mockPattern, mockProfile, loader, mockAgent, cfg)

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

func TestGenerateSkills_MainSkillReferencesOnlyGeneratedCategories(t *testing.T) {
	businessPattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	businessPattern.Confidence = 0.9
	businessPattern.SetDescription("Use the project business flow")
	businessPattern.SetRule("Reuse the existing business flow")

	databasePattern := domain.NewPattern("p2", "Repository Query", domain.CategoryDatabase)
	databasePattern.Confidence = 0.8
	databasePattern.SetDescription("Use repository query helpers")
	databasePattern.SetRule("Keep database access in repository code")

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*businessPattern, *databasePattern}, nil
		},
	}

	svc := newTestService(mockAgent, mockPattern)
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

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{}, nil
		},
	}
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
						Location:      "internal/service/certificate/service.go:42",
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
	svc := NewGeneratorService(mockPattern, mockProfile, loader, mockAgent, cfg)

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
	assert.Contains(t, string(businessMethods), "func (s *Service) IssueCertificate(ctx context.Context, req *IssueRequest) error")

	modules, err := os.ReadFile(filepath.Join(tmpDir, "references", "modules.md"))
	require.NoError(t, err)
	assert.Contains(t, string(modules), "certificate-service")
	assert.Contains(t, string(modules), "internal/service/certificate")

	commonUtils, err := os.ReadFile(filepath.Join(tmpDir, "references", "common-utils.md"))
	require.NoError(t, err)
	assert.Contains(t, string(commonUtils), "NormalizeSerial")
	assert.Contains(t, string(commonUtils), "func NormalizeSerial(serial string) string")

	skill, err := os.ReadFile(filepath.Join(tmpDir, "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(skill), "./references/business-methods.md")
	assert.Contains(t, string(skill), "./references/modules.md")
	assert.Contains(t, string(skill), "./references/common-utils.md")

	_, err = os.Stat(filepath.Join(tmpDir, "references", "examples"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateWorkspaceSkills_RendersOnlyWorkspaceRoot(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetForProjectFn: func(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
			require.Fail(t, "workspace root generation should not read child profiles")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("codex", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Provider: "codex"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, &mocks.MockAgent{NameVal: "codex", AvailableVal: true}, cfg)

	require.NoError(t, svc.GenerateSkills(context.Background(), ""))

	require.FileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-workspace", "SKILL.md"))
	require.FileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-workspace", "references", "workspace-overview.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "skills-seed-skills", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "skills-seed-skills", "references", "project-spec.md"))
}

func TestGenerateWorkspaceSkills_RendersRuntimeContextInWorkspaceReferences(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "hsmwebapi"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "core-engine"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsm-workspace", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{
				{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
				{ID: "core-engine", Path: "core-engine", Type: "library", Language: "go"},
			},
		},
		AgentCfg: config.AgentConfig{Provider: "claude"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg)
	ctx := runtimecontext.WithUserContext(context.Background(), strings.TrimSpace(`
HSM 工作区用于管理密码设备、密钥服务、KMIP 接入和日志/网络组件。
hsmwebapi 是管理 API 入口，core-engine 是核心能力库。
产品是私有化部署，交付物是离线安装包；不要建议 SaaS 化。
常见任务：改管理端接口先进入 hsmwebapi，改核心密码能力先进入 core-engine。
验证管道：先运行 go test ./...，再按受影响模块补充集成验证。
`))

	require.NoError(t, svc.GenerateSkills(ctx, ""))

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "hsm-workspace-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "HSM 工作区用于管理密码设备")
	require.Contains(t, overview, "hsmwebapi 是管理 API 入口")
	require.Contains(t, overview, "私有化部署")
	require.Contains(t, overview, "go test ./...")
}

func TestGenerateWorkspaceSkills_ContextUsesWorkspaceAIPromptsForRootSkill(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "hsmwebapi"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "kmip-go"), 0755))

	var profileReq *agent.AnalyzeWorkspaceProfileRequest
	var specReq *agent.AnalyzeWorkspaceSpecRequest
	mockAgent := &mocks.MockAgent{
		NameVal: "claude", AvailableVal: true,
		AnalyzeWorkspaceProfileFn: func(ctx context.Context, req *agent.AnalyzeWorkspaceProfileRequest) (*domain.WorkspaceProfile, error) {
			profileReq = req
			workspaceInput := readFilePath(t, req.WorkspaceInputPath)
			userContext := readFilePath(t, req.UserContextPath)
			require.Contains(t, workspaceInput, `"id": "hsmwebapi"`)
			require.Contains(t, workspaceInput, `"skill_path": "hsmwebapi/.claude/skills/skills-seed-skills/SKILL.md"`)
			require.Contains(t, userContext, "hsmwebapi 为主后端")
			return &domain.WorkspaceProfile{
				Name:     req.WorkspaceName,
				RootPath: req.WorkspaceRoot,
				Summary:  "HSM 工作区负责私有化密码设备管理，hsmwebapi 调用 kmip-go 暴露 KMIP 能力。",
				Projects: []domain.WorkspaceProject{
					{
						ID:             "hsmwebapi",
						Path:           "hsmwebapi",
						Type:           "backend",
						Language:       "go",
						Responsibility: "主后端，负责管理 API 和调用 KMIP 能力。",
					},
					{
						ID:             "kmip-go",
						Path:           "kmip-go",
						Type:           "backend",
						Language:       "go",
						Responsibility: "KMIP 协议能力实现。",
					},
				},
				Dependencies: []domain.WorkspaceDependency{
					{From: "hsmwebapi", To: "kmip-go", Reason: "hsmwebapi 调用 kmip-go 实现 KMIP 能力"},
				},
				ImpactRoutes: []domain.WorkspaceRoute{
					{PathPattern: "kmip-go/**", ProjectIDs: []string{"hsmwebapi", "kmip-go"}, Reason: "KMIP 能力变更需要同步检查调用方"},
				},
			}, nil
		},
		AnalyzeWorkspaceSpecFn: func(ctx context.Context, req *agent.AnalyzeWorkspaceSpecRequest) (*domain.WorkspaceSpec, error) {
			specReq = req
			workspaceInput := readFilePath(t, req.WorkspaceInputPath)
			profileInput := readFilePath(t, req.WorkspaceProfilePath)
			userContext := readFilePath(t, req.UserContextPath)
			require.Contains(t, workspaceInput, `"id": "kmip-go"`)
			require.Contains(t, profileInput, "hsmwebapi 调用 kmip-go")
			require.Contains(t, userContext, "KMIP 的能力")
			return &domain.WorkspaceSpec{
				Name:     req.WorkspaceName,
				RootPath: req.WorkspaceRoot,
				Routing: []domain.WorkspaceRoute{
					{PathPattern: "hsmwebapi/**", ProjectIDs: []string{"hsmwebapi"}, Reason: "管理 API 变更先读取 hsmwebapi skill"},
					{PathPattern: "kmip-go/**", ProjectIDs: []string{"hsmwebapi", "kmip-go"}, Reason: "KMIP 能力变更需同步检查 hsmwebapi 调用"},
				},
				Rules: []domain.WorkspaceRule{
					{Title: "KMIP 能力同步", Description: "修改 kmip-go 后必须检查 hsmwebapi 的调用适配和集成验证。", AppliesTo: []string{"hsmwebapi", "kmip-go"}},
				},
				ChangeOrder: []string{
					"先确认 KMIP 契约和核心能力边界",
					"再更新 hsmwebapi 调用和验证",
				},
				LoadMultipleSkillsWhen: []domain.WorkspaceLoadMultipleSkill{
					{Condition: "修改 kmip-go/**", ProjectIDs: []string{"hsmwebapi", "kmip-go"}, Reason: "调用方和能力实现必须一起检查"},
				},
			}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsm-workspace", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{
				{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
				{ID: "kmip-go", Path: "kmip-go", Type: "backend", Language: "go"},
			},
		},
		AgentCfg: config.AgentConfig{Provider: "claude"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)
	ctx := runtimecontext.WithUserContext(context.Background(), "hsmwebapi 为主后端，它调用 kmip-go 实现 KMIP 的能力。")

	require.NoError(t, svc.GenerateSkills(ctx, ""))

	require.NotNil(t, profileReq)
	require.NotEmpty(t, profileReq.WorkspaceInputPath)
	require.NotEmpty(t, profileReq.UserContextPath)
	require.Contains(t, filepath.ToSlash(profileReq.WorkspaceInputPath), ".skills-seed/memory/runtime/")
	require.Contains(t, filepath.ToSlash(profileReq.UserContextPath), ".skills-seed/memory/runtime/")
	require.NotNil(t, specReq)
	require.NotEmpty(t, specReq.WorkspaceInputPath)
	require.NotEmpty(t, specReq.WorkspaceProfilePath)
	require.Contains(t, filepath.ToSlash(specReq.WorkspaceProfilePath), ".skills-seed/memory/runtime/")
	require.Equal(t, profileReq.UserContextPath, specReq.UserContextPath)
	require.NoFileExists(t, profileReq.WorkspaceInputPath)
	require.NoFileExists(t, profileReq.UserContextPath)
	require.NoFileExists(t, specReq.WorkspaceProfilePath)

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "hsm-workspace-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "HSM 工作区负责私有化密码设备管理")
	require.Contains(t, overview, "主后端，负责管理 API")
	require.Contains(t, overview, "hsmwebapi 调用 kmip-go 实现 KMIP 能力")
	require.Contains(t, overview, "KMIP 能力变更需要同步检查调用方")

	rules := readGeneratedFile(t, projectRoot, ".claude", "skills", "hsm-workspace-workspace", "references", "cross-project-rules.md")
	require.Contains(t, rules, "KMIP 能力同步")
	require.Contains(t, rules, "先确认 KMIP 契约和核心能力边界")
	require.Contains(t, rules, "修改 kmip-go/**")
}

func TestGenerateWorkspaceSkills_RootSkillStaysConciseAndRoutesViaOverview(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend", ".skills-seed"), 0755))
	childConfig := `
project:
  name: "backend"
  mode: "project"
  language: "go"
  locale: "zh-CN"
  root_path: ""
agent:
  provider: "codex"
  commands:
    codex: "codex"
output:
  skills_paths:
    codex: ".agents/skills/custom-child-skill"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "backend", ".skills-seed", "config.yaml"), []byte(childConfig), 0644))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Provider: "claude"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg)

	require.NoError(t, svc.GenerateSkills(context.Background(), ""))

	rootSkill := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md")
	require.NotContains(t, rootSkill, "## 工作区地图")
	require.NotContains(t, rootSkill, "## 路由规则")
	require.NotContains(t, rootSkill, "## 影响范围判断")
	require.NotContains(t, rootSkill, "## 跨项目执行顺序")
	require.NotContains(t, rootSkill, "## 并发边界")
	require.NotContains(t, rootSkill, "backend/.agents/skills/custom-child-skill/SKILL.md")
	require.Contains(t, rootSkill, "[工作区概览](./references/workspace-overview.md)")
	require.Contains(t, rootSkill, "[跨项目规则](./references/cross-project-rules.md)")

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "子项目配置：`backend/.skills-seed/config.yaml`")
	require.Contains(t, overview, "backend/.agents/skills/custom-child-skill/SKILL.md")
}

func TestGenerateWorkspaceSkills_RoutingTableHasNoBlankLines(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "frontend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{
				{ID: "backend", Path: "backend", Type: "backend", Language: "go"},
				{ID: "frontend", Path: "frontend", Type: "frontend", Language: "typescript"},
			},
		},
		AgentCfg: config.AgentConfig{Provider: "claude"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg)

	require.NoError(t, svc.GenerateSkills(context.Background(), ""))

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	assertMarkdownTableHasNoBlankLines(t, overview, "## 路由表")
}

func TestGenerateWorkspaceSkills_DoesNotReadRuntimeContextFromWorkspaceMemory(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".skills-seed", "memory"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, ".skills-seed", "memory", "workspace-profile.json"), []byte(`{
  "name": "demo",
  "root_path": "`+projectRoot+`",
  "summary": "旧的 workspace memory summary 不应进入新生成结果",
  "projects": []
}`), 0644))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Provider: "claude"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg)

	require.NoError(t, svc.GenerateSkills(context.Background(), ""))

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.NotContains(t, overview, "旧的 workspace memory summary")
}

func TestGenerateWorkspaceSkills_UsesConfiguredProviderOnly(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend"), 0755))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetForProjectFn: func(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
			require.Fail(t, "workspace root generation should not read child profiles")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Provider: "claude"},
		OutputCfg: config.OutputConfig{SkillsPaths: map[string]string{
			"claude": ".claude/skills/skills-seed-skills",
			"codex":  ".agents/skills/skills-seed-skills",
		}},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg)

	require.NoError(t, svc.GenerateSkills(context.Background(), ""))
	require.FileExists(t, filepath.Join(projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, ".agents", "skills", "demo-workspace", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".claude", "skills", "skills-seed-skills", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "skills-seed-skills", "SKILL.md"))

	rootSkill := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md")
	require.Contains(t, rootSkill, "[工作区概览](./references/workspace-overview.md)")
	require.Contains(t, rootSkill, "[跨项目规则](./references/cross-project-rules.md)")
	require.NotContains(t, rootSkill, "backend/.claude/skills/skills-seed-skills/SKILL.md")

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	require.Contains(t, overview, "未显式配置契约路径")
	require.Contains(t, overview, "默认写入边界")
}

func TestGenerateWorkspaceSkills_UsesChildProjectConfigPath(t *testing.T) {
	projectRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "backend", ".skills-seed"), 0755))
	childConfig := `
project:
  name: "backend"
  mode: "project"
  language: "go"
  locale: "zh-CN"
  root_path: ""
agent:
  provider: "codex"
  commands:
    codex: "codex"
output:
  skills_paths:
    codex: ".agents/skills/custom-child-skill"
`
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "backend", ".skills-seed", "config.yaml"), []byte(childConfig), 0644))

	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			require.Fail(t, "workspace root generation should not read centralized patterns")
			return nil, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetForProjectFn: func(ctx context.Context, projectID string) (*domain.ProjectProfile, error) {
			require.Fail(t, "workspace root generation should not read child profiles")
			return nil, nil
		},
	}
	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "demo", Mode: domain.ModeWorkspace, RootPath: projectRoot, Language: "go"},
		WorkspaceCfg: config.WorkspaceConfig{
			Projects: []config.WorkspaceProjectConfig{{ID: "backend", Path: "backend", Type: "backend", Language: "go"}},
		},
		AgentCfg: config.AgentConfig{Provider: "claude"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, &mocks.MockAgent{NameVal: "claude", AvailableVal: true}, cfg)

	require.NoError(t, svc.GenerateSkills(context.Background(), ""))
	require.FileExists(t, filepath.Join(projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".agents", "skills", "custom-child-skill", "SKILL.md"))
	require.NoFileExists(t, filepath.Join(projectRoot, "backend", ".claude", "skills", "skills-seed-skills", "SKILL.md"))

	rootSkill := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "SKILL.md")
	assert.NotContains(t, rootSkill, "子项目自带 skills-seed")
	assert.NotContains(t, rootSkill, "backend/.skills-seed/config.yaml")
	assert.NotContains(t, rootSkill, "backend/.agents/skills/custom-child-skill/SKILL.md")

	overview := readGeneratedFile(t, projectRoot, ".claude", "skills", "demo-workspace", "references", "workspace-overview.md")
	assert.Contains(t, overview, "子项目配置：`backend/.skills-seed/config.yaml`")
	assert.Contains(t, overview, "backend/.agents/skills/custom-child-skill/SKILL.md")
}

func TestGenerateSkills_RendersCompactActionableSkillReferences(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.916
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")

	mockAgent := &mocks.MockAgent{
		NameVal: "claude", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{
				CategorySummaries: map[string]agent.CategorySummary{
					"business": {
						Category:    "business",
						Summary:     "Business-specific conventions",
						Patterns:    []string{"Business Flow"},
						UsageScenes: []string{"when changing business flows"},
						Priority:    5,
						BusinessMethods: []*domain.BusinessMethod{
							{
								Name:        "DuplicatedMethod",
								Location:    "internal/service/demo.go:10",
								Description: "method already documented in business-methods.md",
								Usage:       "business flow reuse",
								Type:        "domain",
								Function:    "func DuplicatedMethod() error",
							},
						},
					},
				},
			}, nil
		},
	}
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
						Name:        "DuplicatedMethod",
						Location:    "internal/service/demo.go:10",
						Description: "method documented once in the split reference",
						Usage:       "business flow reuse",
						Type:        "domain",
						Function:    "func DuplicatedMethod() error",
					},
				},
			}, nil
		},
	}

	loader := skills.NewLoaderForAgent("claude", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "hsmwebapi", Language: "go"},
		AgentCfg:   config.AgentConfig{Provider: "claude"},
	}
	svc := NewGeneratorService(mockPattern, mockProfile, loader, mockAgent, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.Contains(t, skill, "description: 修改、审查或扩展 hsmwebapi go 代码时使用")
	assert.Contains(t, skill, "**平均置信度**: 91.60%")
	assert.NotContains(t, skill, "91.60000000000001")

	businessPattern := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	assert.NotContains(t, businessPattern, "## 业务方法汇总")
	assert.NotContains(t, businessPattern, "DuplicatedMethod")

	businessMethods := readGeneratedFile(t, tmpDir, "references", "business-methods.md")
	assert.Contains(t, businessMethods, "DuplicatedMethod")

	modules := readGeneratedFile(t, tmpDir, "references", "modules.md")
	assert.Contains(t, modules, "RegisterKeyManage")
	assert.NotContains(t, modules, "TODO: 需读取插件内部代码确认")

	assertNoExcessiveBlankLines(t, tmpDir)
}

func TestGenerateSkills_SkipsEmptyBusinessMethodDetails(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")
	pattern.SetBusinessMethod(&domain.BusinessMethod{})

	mockAgent := &mocks.MockAgent{
		NameVal: "test", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockAgent, mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "patterns", "business.md"))
	require.NoError(t, err)
	businessPattern := string(content)

	assert.NotContains(t, businessPattern, "业务方法详情")
	assert.NotContains(t, businessPattern, "| **方法** | `` |")
	assert.NotContains(t, businessPattern, "```go\n\n```")
}

func TestGenerateSkills_CodexWritesOpenAIMetadata(t *testing.T) {
	pattern := domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Use the existing business flow")
	pattern.SetRule("Reuse the documented flow")

	mockAgent := &mocks.MockAgent{
		NameVal: "codex", AvailableVal: true,
		GenerateSkillsSummaryFn: func(ctx context.Context, req *agent.GenerateSkillsRequest) (*agent.GenerateSkillsResult, error) {
			return &agent.GenerateSkillsResult{}, nil
		},
	}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}
	loader := skills.NewLoaderForAgent("codex", "zh-CN")
	cfg := &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
		AgentCfg:   config.AgentConfig{Provider: "codex"},
	}
	svc := NewGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, mockAgent, cfg)

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
	svc := newTestService(nil, nil)

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
	svc := newTestService(nil, nil)
	stats := svc.calculateStats([]domain.Pattern{})
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, 0.0, stats.AvgConfidence)
}

func TestCalculateCategoryConfidence(t *testing.T) {
	svc := newTestService(nil, nil)
	patterns := []domain.Pattern{
		*domain.NewPattern("p1", "E1", domain.CategoryError),
		*domain.NewPattern("p2", "E2", domain.CategoryError),
		*domain.NewPattern("p3", "N1", domain.CategoryNaming),
	}
	patterns[0].Confidence = 0.8
	patterns[1].Confidence = 0.9
	patterns[2].Confidence = 0.7

	assert.InDelta(t, 0.85, svc.calculateCategoryConfidence(patterns, "error"), 0.01)
	assert.InDelta(t, 0.7, svc.calculateCategoryConfidence(patterns, "naming"), 0.01)
	assert.InDelta(t, 0.0, svc.calculateCategoryConfidence(patterns, "testing"), 0.01)
}
