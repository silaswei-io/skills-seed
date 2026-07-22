package generator

import (
	"context"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

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
	svc := newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
	tmpDir := t.TempDir()
	err := svc.GenerateSkills(context.Background(), tmpDir)
	assert.NoError(t, err)
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
	svc := newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
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
	svc := newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)
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
	svc := newGeneratorService(mockPattern, &mocks.MockProjectProfileRepository{}, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business.md"), "business 分类包含 1 个项目特定模式")
	assert.Contains(t, readGeneratedFile(t, tmpDir, "references", "patterns", "business", "ai-rule.md"), "AI Rule")
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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "project-overview.md"))
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "## 项目概览摘要")
	require.Contains(t, text, "当前项目画像已覆盖 2 个模块/业务域")
	require.Contains(t, text, "home")
	require.Contains(t, text, "key-manage")
	require.NotContains(t, text, "## 项目概览摘要\n\nhome-info单元")
	require.NotContains(t, text, "## 技术架构\n\nhome-info单元")
}
