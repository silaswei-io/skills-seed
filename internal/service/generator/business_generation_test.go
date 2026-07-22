package generator

import (
	"context"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

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
	require.Contains(t, detail, "可复用解决方案与证据")
	require.Contains(t, detail, "适用与实现观察（需按源码复核）")
	require.NotContains(t, detail, "复用建议（需按源码复核）")
	require.Contains(t, detail, "订单状态流转必须经过订单服务的状态校验")
	require.Contains(t, detail, "ApplyOrderTransition")
	require.NotContains(t, detail, "订单状态流转入口")
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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)

	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	commonUtils, err := os.ReadFile(filepath.Join(tmpDir, "references", "common-utils.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(commonUtils), "SM4Encrypt")
	assert.Contains(t, string(commonUtils), "FormatFileSize")
	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateSkills_GroupsBusinessMethodsBySourceModule(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*domain.NewPattern("p1", "Business Rule", domain.CategoryBusiness)}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "backend",
				Language:    "go",
				Summary:     "Profile-backed project overview",
				BusinessMethods: []domain.BusinessMethod{
					{
						Name:         "Run",
						CodeLocation: domain.CodeLocation{CurrentLocation: "cmd/server.go:20"},
						Description:  "starts the server",
						Function:     "func (s *Server) Run(ctx context.Context) error",
						Type:         "domain",
					},
					{
						Name:         "Run",
						CodeLocation: domain.CodeLocation{CurrentLocation: "internal/plugin/vpn/run.go:12"},
						Description:  "runs VPN plugin command",
						Function:     "func (p *Plugin) Run(ctx context.Context) error",
						Type:         "domain",
					},
				},
			}, nil
		},
	}
	svc := newGeneratorService(mockPattern, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "backend"},
	})
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	methods := readGeneratedFile(t, tmpDir, "references", "business-methods.md")
	assert.Contains(t, methods, "## Cmd")
	assert.Contains(t, methods, "### Run")
	assert.Contains(t, methods, "## Vpn")
	assert.Contains(t, methods, "- **归属模块**: `vpn`")
	assert.NotContains(t, methods, "原始方法名")
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

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "patterns", "business", "business-flow.md"))
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
	svc := newGeneratorService(mockPattern, mockProfile, loader, cfg)
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
	assert.Contains(t, billingDetail, "#### 代码证据")
	assert.Contains(t, billingDetail, "Activate")

	notificationDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "notification.md")
	assert.Contains(t, notificationDetail, "Delivery Rule")

	assertNoBrokenMarkdownLinks(t, tmpDir)
}
