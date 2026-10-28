package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSkillsSplitsUnfocusedBusinessRulesIntoIndividualReferences(t *testing.T) {
	for _, locale := range []string{"zh-CN", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			patterns := make([]domain.Pattern, 0, 32)
			for i := 0; i < 32; i++ {
				pattern := domain.NewPattern(fmt.Sprintf("rule-%02d", i), fmt.Sprintf("Rule %02d", i), domain.CategoryBusiness)
				pattern.SetDescription(fmt.Sprintf("Unique business boundary %02d: %s", i, strings.Repeat("source-backed detail ", 20)))
				pattern.SetRule(fmt.Sprintf("Reuse the verified entry for rule %02d", i))
				pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: fmt.Sprintf("internal/domain/rule_%02d.go", i), Line: 10}}
				patterns = append(patterns, *pattern)
			}
			svc := newGeneratorService(&mocks.MockPatternRepository{
				GetAllFn: func(context.Context) ([]domain.Pattern, error) { return patterns, nil },
			}, &mocks.MockProjectProfileRepository{}, skills.NewLoader(locale), &mocks.MockConfigReader{
				ProjectCfg: config.ProjectConfig{Name: "test", Language: "go"},
			})
			output := t.TempDir()
			require.NoError(t, svc.GenerateSkills(context.Background(), output))

			index := readGeneratedFile(t, output, "references", "patterns", "business.md")
			require.NoFileExists(t, filepath.Join(output, "references", "patterns", "business", "other.md"))
			for _, pattern := range patterns {
				detail := readGeneratedFile(t, output, "references", "patterns", "business", "items", referenceFileID(pattern.ID)+".md")
				require.Contains(t, detail, strings.TrimSpace(pattern.Description))
				require.NotContains(t, index, strings.TrimSpace(pattern.Description))
			}
			require.Contains(t, index, "./business/items/")
			assertNoBrokenMarkdownLinks(t, output)
		})
	}
}

func TestGenerateSkillsUsesResponsibilityThemeIndexWithoutMergingRules(t *testing.T) {
	patterns := []domain.Pattern{
		*domain.NewPattern("template-create", "证书模板创建校验", domain.CategoryBusiness),
		*domain.NewPattern("template-update", "证书模板更新保护", domain.CategoryBusiness),
		*domain.NewPattern("admin-login", "管理员登录契约", domain.CategoryBusiness),
	}
	for i := range patterns {
		patterns[i].SetDescription(patterns[i].Name + " 的独立条件")
	}
	svc := newTestService(&mocks.MockPatternRepository{GetAllFn: func(context.Context) ([]domain.Pattern, error) {
		return patterns, nil
	}})
	output := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), output))
	index := readGeneratedFile(t, output, "references", "patterns", "business.md")
	require.Contains(t, index, "证书模板")
	require.Contains(t, index, "管理员登录契约")
	require.NotContains(t, index, "证书模板创建校验")
	group := readGeneratedFile(t, output, "references", "patterns", "business", "groups", referenceFileID("证书模板")[:16]+".md")
	require.Contains(t, group, "证书模板创建校验")
	require.Contains(t, group, "证书模板更新保护")
	require.NotContains(t, group, "独立条件")
	for _, pattern := range patterns {
		page := readGeneratedFile(t, output, "references", "patterns", "business", "items", referenceFileID(pattern.ID)+".md")
		require.Contains(t, page, pattern.Name+" 的独立条件")
	}
	assertNoBrokenMarkdownLinks(t, output)
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
	require.Contains(t, index, "直接入口")
	detail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "items", referenceFileID(pattern.ID)+".md")
	require.Contains(t, detail, "适用边界")
	require.Contains(t, detail, "订单状态流转必须经过订单服务的状态校验")
	require.Contains(t, detail, "ApplyOrderTransition")
	require.NotContains(t, index, "订单状态流转必须经过订单服务的状态校验")
	require.NotContains(t, index, "订单状态流转入口")
	require.NoFileExists(t, filepath.Join(tmpDir, "references", "patterns", "business", "order.md"))
}

func TestGenerateSkillsRoutesReviewedBusinessRuleDirectly(t *testing.T) {
	pattern := domain.NewPattern("certificate-status-transition", "Certificate Status Transition", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Applies the verified certificate status transition.")
	pattern.SetRule("Reuse the verified transition boundary before adding a certificate state change.")
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "internal/certificate/lifecycle.go",
		Line:   42,
		Symbol: "ApplyTransition",
		Kind:   "method",
	}}

	svc := newTestService(&mocks.MockPatternRepository{
		GetAllFn: func(context.Context) ([]domain.Pattern, error) { return []domain.Pattern{*pattern}, nil },
	})
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	require.NotContains(t, skill, "开发焦点")
	require.Contains(t, skill, "./references/patterns/business.md")
	require.Contains(t, skill, "不得从现有代码推断可直接编辑")
	require.NotContains(t, skill, "internal/certificate/lifecycle.go:42")

	index := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	require.Contains(t, index, "Certificate Status Transition")
	require.Contains(t, index, "./business/items/"+referenceFileID(pattern.ID)+".md")
	require.NoFileExists(t, filepath.Join(tmpDir, "references", "patterns", "business", "certificate-lifecycle.md"))
}

func TestGenerateSkillsIndexesReviewedReusableCapability(t *testing.T) {
	pattern := domain.NewPattern("sm9-encryption", "SM9 encryption capability", domain.CategoryUtils)
	pattern.SetDescription("Use the existing SM9 encryption entry for compatible ciphertext.")
	pattern.BusinessMethod = &domain.BusinessMethod{
		Name: "EncryptSM9", CodeLocation: domain.CodeLocation{CurrentLocation: "internal/crypto/sm9.go:20"},
		Description: "Encrypts data with the project's SM9 implementation.",
		Usage:       "SM9 加密、SM9 ciphertext、EncryptSM9",
		Type:        "common", Function: "func EncryptSM9(publicKey []byte, plaintext []byte) ([]byte, error)",
		Prerequisites: "SM9 public key", Returns: "ciphertext or an error",
	}
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "internal/crypto/sm9.go", Line: 20, Symbol: "EncryptSM9", Kind: "function"}}
	svc := newTestService(&mocks.MockPatternRepository{GetAllFn: func(context.Context) ([]domain.Pattern, error) {
		return []domain.Pattern{*pattern}, nil
	}})
	output := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), output))

	index := readGeneratedFile(t, output, "references", "business-methods.md")
	require.Contains(t, index, "SM9 加密")
	require.Contains(t, index, "EncryptSM9")
	require.Contains(t, index, "internal/crypto/sm9.go:20")
	require.Contains(t, readGeneratedFile(t, output, "SKILL.md"), "./references/business-methods.md")
	assertNoBrokenMarkdownLinks(t, output)
}

func TestGenerateSkillsMarksHighRiskBusinessPatternsAsCheckpoints(t *testing.T) {
	pattern := domain.NewPattern("resource-destroy", "Resource Destroy Command", domain.CategoryBusiness)
	pattern.Confidence = 0.9
	pattern.SetDescription("Destroy command deletes resource state and changes an external environment.")
	pattern.SetRule("Inspect destroy safeguards before changing this operation.")
	pattern.KnowledgeFlags = []string{domain.KnowledgeFlagOperationalRisk}
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{
		Path:   "tools/commands/destroy.ts",
		Line:   12,
		Symbol: "destroyResource",
		Kind:   "function",
	}}
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*pattern}, nil
		},
	}

	svc := newTestService(mockPattern)
	tmpDir := t.TempDir()
	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	index := readGeneratedFile(t, tmpDir, "references", "patterns", "business.md")
	require.Contains(t, index, "Resource Destroy Command")
	detail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "items", referenceFileID(pattern.ID)+".md")
	require.Contains(t, detail, "风险检查点")
	require.Contains(t, detail, "不要把危险动作当作普通复用建议")
}

func TestGenerateSkillsDoesNotProjectLegacyProfileUtilities(t *testing.T) {
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

	require.NoFileExists(t, filepath.Join(tmpDir, "references", "common-utils.md"))
	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	assert.NotContains(t, skill, "./references/common-utils.md")
	assertNoBrokenMarkdownLinks(t, tmpDir)
}

func TestGenerateSkills_SkipsWeakCommonUtilsReference(t *testing.T) {
	mockPattern := &mocks.MockPatternRepository{
		GetAllFn: func(ctx context.Context) ([]domain.Pattern, error) {
			return []domain.Pattern{*domain.NewPattern("p1", "Business Flow", domain.CategoryBusiness)}, nil
		},
	}
	mockProfile := &mocks.MockProjectProfileRepository{
		GetFn: func(ctx context.Context) (*domain.ProjectProfile, error) {
			return &domain.ProjectProfile{
				ProjectName: "test",
				Language:    "typescript",
				Summary:     "profile",
				CommonUtils: []domain.UtilityFunction{{
					Name:        "BuildCacheKey",
					File:        "src/cache/key.ts",
					Signature:   "function BuildCacheKey(id: string): string",
					Description: "builds a cache key",
				}},
			}, nil
		},
	}
	svc := newGeneratorService(mockPattern, mockProfile, skills.NewLoader("zh-CN"), &mocks.MockConfigReader{
		ProjectCfg: config.ProjectConfig{Name: "test", Language: "typescript"},
	})
	tmpDir := t.TempDir()

	require.NoError(t, svc.GenerateSkills(context.Background(), tmpDir))

	require.NoFileExists(t, filepath.Join(tmpDir, "references", "common-utils.md"))
	skill := readGeneratedFile(t, tmpDir, "SKILL.md")
	require.NotContains(t, skill, "./references/common-utils.md")
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
						Name:          "Run",
						CodeLocation:  domain.CodeLocation{CurrentLocation: "cmd/server.go:20"},
						Description:   "starts the server",
						Usage:         "server startup",
						Function:      "func (s *Server) Run(ctx context.Context) error",
						Prerequisites: "initialized server dependencies",
						Returns:       "an error when startup fails",
						Type:          "domain",
					},
					{
						Name:          "Run",
						CodeLocation:  domain.CodeLocation{CurrentLocation: "internal/plugin/vpn/run.go:12"},
						Description:   "runs VPN plugin command",
						Usage:         "VPN plugin execution",
						Function:      "func (p *Plugin) Run(ctx context.Context) error",
						Prerequisites: "initialized VPN plugin",
						Returns:       "an error when plugin execution fails",
						Type:          "domain",
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
	assert.Contains(t, methods, "[Run](./business-methods/")
	assert.Contains(t, methods, "## Vpn")
	detailFiles, err := filepath.Glob(filepath.Join(tmpDir, "references", "business-methods", "*.md"))
	require.NoError(t, err)
	require.Len(t, detailFiles, 2)
	var details string
	for _, path := range detailFiles {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		details += string(content)
	}
	assert.Contains(t, details, "- **归属模块**: `vpn`")
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

	content, err := os.ReadFile(filepath.Join(tmpDir, "references", "patterns", "business.md"))
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
	patterns[0].ScopePath = "internal/application/billing"
	patterns[1].ScopePath = "internal/application/billing"
	patterns[2].ScopePath = "internal/application/notification"

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
	for _, pattern := range patterns {
		assert.Contains(t, index, "./business/items/"+referenceFileID(pattern.ID)+".md")
	}
	assert.Contains(t, index, "Delivery Rule")
	assert.NotContains(t, index, "func (s *Service)")
	assert.NotContains(t, index, "#### ✅ 代码证据")

	activationDetail := readGeneratedFile(t, tmpDir, "references", "patterns", "business", "items", referenceFileID(patterns[0].ID)+".md")
	assert.Contains(t, activationDetail, "已验证入口")
	assert.Contains(t, activationDetail, "internal/application/billing/service.go:10")
	require.NoFileExists(t, filepath.Join(tmpDir, "references", "patterns", "business", "billing.md"))
	require.NoFileExists(t, filepath.Join(tmpDir, "references", "patterns", "business", "notification.md"))

	assertNoBrokenMarkdownLinks(t, tmpDir)
}
