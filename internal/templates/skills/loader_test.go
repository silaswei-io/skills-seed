package skills

import (
	"os"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoader_Render 测试主模板渲染
func TestLoader_Render(t *testing.T) {
	loader := NewLoader("zh-CN")

	data := map[string]interface{}{
		"ProgramVersion":      "v0.0.1",
		"SkillsTemplatesHash": "test-hash",
		"ProjectName":         "test-project",
		"SkillName":           "test-project-dev",
		"Language":            "go",
		"PatternCount":        10,
		"AvgConfidence":       85.5,
		"Categories":          3,
		"LastUpdated":         "2026-03-25 15:00:00",
		"CategorySummaries": map[string]interface{}{
			"api": map[string]interface{}{
				"Category":    "api",
				"Summary":     "API 相关模式",
				"Patterns":    []string{"模式1", "模式2"},
				"UsageScenes": []string{"场景1", "场景2"},
				"Priority":    2,
			},
		},
		"KeyPatterns":    []string{"关键模式1", "关键模式2"},
		"BusinessRules":  []string{"业务规则1", "业务规则2"},
		"BestPractices":  []string{"最佳实践1", "最佳实践2"},
		"CommonPatterns": []string{"通用模式1", "通用模式2"},
		"OverviewReferences": []ReferenceItem{
			{Title: "业务方法", Path: "./references/business-methods.md", Description: "完整业务方法清单"},
		},
		"ReferenceGroups": []ReferenceGroup{
			{
				Title: "业务与领域",
				Items: []ReferenceItem{
					{
						Title:       "API 模式",
						Path:        "./references/patterns/api.md",
						Description: "API 相关模式",
					},
				},
			},
		},
	}

	content, err := loader.Render("skill", data)

	assert.NoError(t, err)
	assert.NotEmpty(t, content)
	// 验证关键内容是否存在
	assert.Contains(t, content, "test-project")
	assert.Contains(t, content, "go")
	assert.Contains(t, content, "10")
	assert.Contains(t, content, "generated-by: skills-seed v0.0.1")
	assert.Contains(t, content, "skills-template-sha256: test-hash")
}

// TestLoader_Render_English 测试英文模板渲染
func TestLoader_Render_English(t *testing.T) {
	loader := NewLoader("en-US")

	data := map[string]interface{}{
		"ProgramVersion":      "v0.0.1",
		"SkillsTemplatesHash": "test-hash",
		"ProjectName":         "test-project",
		"SkillName":           "test-project-dev",
		"Language":            "go",
		"PatternCount":        10,
		"AvgConfidence":       85.5,
		"Categories":          3,
		"LastUpdated":         "2026-03-25 15:00:00",
		"CategorySummaries":   map[string]interface{}{},
		"KeyPatterns":         []string{},
		"BusinessRules":       []string{},
		"BestPractices":       []string{},
		"CommonPatterns":      []string{},
		"OverviewReferences":  []ReferenceItem{},
		"ReferenceGroups":     []ReferenceGroup{},
	}

	content, err := loader.Render("skill", data)

	assert.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "test-project")
	assert.Contains(t, content, "skills-template-sha256: test-hash")
}

func TestLoader_RenderZhSkillFrontmatterDescriptionIsLocalized(t *testing.T) {
	for _, agentName := range []string{"common", "claude", "codex"} {
		t.Run(agentName, func(t *testing.T) {
			loader := NewLoaderForAgent(agentName, "zh-CN")

			content, err := loader.Render("skill", fullSkillData())
			require.NoError(t, err)

			require.Contains(t, content, "description: 修改、审查或扩展 demo go 代码时使用")
			require.NotContains(t, content, "description: Use when")
		})
	}
}

// TestLoader_RenderReference 测试分类模板渲染
func TestLoader_RenderReference(t *testing.T) {
	loader := NewLoader("zh-CN")

	data := map[string]interface{}{
		"Category":        "api",
		"Summary":         "API 相关的编码模式",
		"Patterns":        []string{"API 路由命名规范", "错误处理模式"},
		"PatternObjects":  []domain.Pattern{*domain.NewPattern("p1", "API 路由命名规范", domain.CategoryAPI)},
		"UsageScenes":     []string{"创建新 API 端点时", "重构 API 路由时"},
		"Priority":        4,
		"PatternCount":    2,
		"Confidence":      87.5,
		"LastUpdated":     "2026-03-25 15:00:00",
		"BusinessMethods": []interface{}{}, // 添加业务方法
	}

	content, err := loader.RenderPattern("api", data)

	assert.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "api")
	assert.Contains(t, content, "API 相关的编码模式")
	assert.Contains(t, content, "API 路由命名规范")
}

// TestLoader_RenderReference_NotFound 测试不存在的模板
func TestLoader_RenderReference_NotFound(t *testing.T) {
	loader := NewLoader("zh-CN")

	data := map[string]interface{}{
		"Category": "unknown",
	}

	content, err := loader.RenderReference("unknown-category", "overview", data)

	assert.Error(t, err)
	assert.Empty(t, content)
}

func TestLoader_RenderReferenceFile(t *testing.T) {
	loader := NewLoader("zh-CN")

	content, err := loader.RenderReferenceFile("business-methods", projectOverviewData())

	require.NoError(t, err)
	assert.Contains(t, content, "Demo")
	assert.Contains(t, content, "func Demo()")
}

func TestLoader_RenderPatternFallsBackToDefaultTemplate(t *testing.T) {
	loader := NewLoader("zh-CN")

	content, err := loader.RenderPattern("custom-domain", map[string]interface{}{
		"Category":        "custom-domain",
		"Summary":         "自定义分类模式",
		"Patterns":        []string{"Custom Pattern"},
		"PatternObjects":  []domain.Pattern{*domain.NewPattern("p1", "Custom Pattern", domain.Category("custom-domain"))},
		"UsageScenes":     []string{"自定义场景"},
		"Priority":        3,
		"PatternCount":    1,
		"Confidence":      80.0,
		"LastUpdated":     "2026-05-21 00:00:00",
		"BusinessMethods": []*domain.BusinessMethod{},
	})

	require.NoError(t, err)
	assert.Contains(t, content, "custom-domain")
	assert.Contains(t, content, "Custom Pattern")
}

// TestLoader_TemplateExists 测试模板存在性检查
func TestLoader_TemplateExists(t *testing.T) {
	loader := NewLoader("zh-CN")

	// 测试存在的模板
	exists := loader.TemplateExists("skill")
	assert.True(t, exists)

	// 测试不存在的模板
	exists = loader.TemplateExists("nonexistent")
	assert.False(t, exists)
}

// TestLoader_GetLocale 测试语言设置
func TestLoader_GetLocale(t *testing.T) {
	loaderZh := NewLoader("zh-CN")
	assert.Equal(t, "zh-CN", loaderZh.GetLocale())

	loaderEn := NewLoader("en-US")
	assert.Equal(t, "en-US", loaderEn.GetLocale())
}

func TestLoader_RenderAllSkillTemplates(t *testing.T) {
	categories := []string{
		"api",
		"business",
		"concurrency",
		"config",
		"database",
		"error-handling",
		"middleware",
		"naming",
		"structure",
		"testing",
		"utils",
	}

	for _, agentName := range []string{"claude", "codex"} {
		t.Run(agentName, func(t *testing.T) {
			for _, locale := range []string{"en-US", "zh-CN"} {
				t.Run(locale, func(t *testing.T) {
					loader := NewLoaderForAgent(agentName, locale)

					mainContent, err := loader.Render("skill", fullSkillData())
					require.NoError(t, err)
					require.NotEmpty(t, mainContent)

					overview, err := loader.RenderProjectOverview(projectOverviewData())
					require.NoError(t, err)
					require.NotEmpty(t, overview)

					for _, reference := range []string{"business-methods", "modules", "common-utils"} {
						referenceContent, err := loader.RenderReferenceFile(reference, projectOverviewData())
						require.NoError(t, err)
						require.NotEmpty(t, referenceContent)
					}

					for _, category := range categories {
						t.Run(category, func(t *testing.T) {
							patternContent, err := loader.RenderPattern(category, categoryData(category))
							require.NoError(t, err)
							require.NotEmpty(t, patternContent)
						})
					}
				})
			}
		})
	}
}

func TestLoader_RenderAgentMetadata(t *testing.T) {
	loader := NewLoaderForAgent("codex", "en-US")

	files, err := loader.RenderAgentMetadataFiles(fullSkillData())
	require.NoError(t, err)
	require.Len(t, files, 1)
	content := files[0].Content
	require.Equal(t, "agents/openai.yaml", files[0].Path)
	require.Contains(t, content, "display_name")
	require.Contains(t, content, "$demo-dev")

	claudeLoader := NewLoaderForAgent("claude", "en-US")
	files, err = claudeLoader.RenderAgentMetadataFiles(fullSkillData())
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestSkillTemplates_DoNotKeepDuplicateOrRetiredReferenceTemplates(t *testing.T) {
	for _, path := range []string{
		"../../../embedfs/templates/skills/common/references/examples",
		"../../../embedfs/templates/skills/claude/references/project-overview.md.tmpl",
		"../../../embedfs/templates/skills/claude/references/project-overview.zh-CN.md.tmpl",
		"../../../embedfs/templates/skills/claude/references/patterns",
		"../../../embedfs/templates/skills/claude/references/examples",
		"../../../embedfs/templates/skills/codex/references/project-overview.md.tmpl",
		"../../../embedfs/templates/skills/codex/references/project-overview.zh-CN.md.tmpl",
		"../../../embedfs/templates/skills/codex/references/patterns",
		"../../../embedfs/templates/skills/codex/references/examples",
	} {
		_, err := os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist, path)
	}
}

func TestLoader_RenderMissingMapKeyFails(t *testing.T) {
	loader := NewLoader("en-US")

	_, err := loader.Render("skill", map[string]interface{}{
		"ProgramVersion":      "v0.0.1",
		"SkillsTemplatesHash": "test-hash",
		"ProjectName":         "demo",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "SkillName")
}

func fullSkillData() map[string]interface{} {
	return map[string]interface{}{
		"ProgramVersion":      "v0.0.1",
		"SkillsTemplatesHash": "test-hash",
		"ProjectName":         "demo",
		"SkillName":           "demo-dev",
		"Language":            "go",
		"PatternCount":        1,
		"AvgConfidence":       90.0,
		"Categories":          1,
		"LastUpdated":         "2026-05-19 00:00:00",
		"CategorySummaries":   map[string]agent.CategorySummary{},
		"KeyPatterns":         []agent.PatternSummary{},
		"BusinessRules":       []string{"business rule"},
		"BestPractices":       []string{"best practice"},
		"CommonPatterns":      []string{"common pattern"},
		"OverviewReferences": []ReferenceItem{
			{Title: "业务方法", Path: "./references/business-methods.md", Description: "完整业务方法清单"},
		},
		"ReferenceGroups": []ReferenceGroup{
			{
				Title: "业务与领域",
				Items: []ReferenceItem{
					{
						Title:       "业务模式",
						Path:        "./references/patterns/business.md",
						Description: "业务逻辑模式",
					},
				},
			},
		},
	}
}

func categoryData(category string) map[string]interface{} {
	pattern := domain.NewPattern("p1", "Demo Pattern", domain.CategoryAPI)
	pattern.SetDescription("demo description")
	pattern.SetRule("demo rule")
	pattern.SetExamples("good()", "bad()")
	pattern.Confidence = 0.9
	pattern.Frequency = 2
	method := &domain.BusinessMethod{
		Name:          "DemoMethod",
		Location:      "internal/demo.go:10",
		Description:   "demo business method",
		Usage:         "use in demo flow",
		Type:          "domain",
		Function:      "func DemoMethod(ctx context.Context) error",
		Prerequisites: "initialized service",
		Returns:       "error when demo fails",
	}
	pattern.BusinessMethod = method

	return map[string]interface{}{
		"Category":        category,
		"Summary":         "demo summary",
		"Patterns":        []string{"Demo Pattern"},
		"PatternObjects":  []domain.Pattern{*pattern},
		"UsageScenes":     []string{"demo usage"},
		"Priority":        4,
		"PatternCount":    1,
		"Confidence":      90.0,
		"LastUpdated":     "2026-05-19 00:00:00",
		"BusinessMethods": []*domain.BusinessMethod{method},
		"SamplePatterns":  []string{"Demo Pattern"},
	}
}

func projectOverviewData() map[string]interface{} {
	return map[string]interface{}{
		"ProjectName":       "demo",
		"Language":          "go",
		"GeneratedAt":       "2026-05-19 00:00:00",
		"Summary":           "demo summary",
		"Architecture":      "layered",
		"Layers":            []domain.ArchitectureLayer{{Name: "service", Description: "business", Responsibilities: []string{"orchestrate"}, Files: []string{"internal/service/demo.go"}}},
		"DependencyGraph":   "command -> service -> domain",
		"DataFlow":          "request -> service -> repository",
		"Frameworks":        []string{"cobra"},
		"Dependencies":      []string{"bbolt"},
		"FrameworkPatterns": []string{"cobra commands"},
		"Structure":         "internal/",
		"OverviewReferences": []ReferenceItem{
			{Title: "业务方法", Path: "./business-methods.md", Description: "完整业务方法清单"},
			{Title: "关键模块", Path: "./modules.md", Description: "完整模块清单"},
			{Title: "通用工具", Path: "./common-utils.md", Description: "工具方法清单"},
		},
		"KeyModules":      []domain.ModuleInfo{{Name: "service", Path: "internal/service", Description: "business layer", Responsibilities: []string{"orchestrate"}, Dependencies: []string{"domain"}, Dependents: []string{"command"}, KeyMethods: []string{"Run()"}}},
		"BusinessMethods": []domain.BusinessMethod{{Name: "Demo", Location: "internal/demo.go:10", Description: "demo", Function: "func Demo()", Usage: "demo", Type: "domain"}},
		"CommonUtils":     []domain.UtilityFunction{{Name: "DemoUtil", File: "internal/utils/demo.go", Signature: "func DemoUtil()", Description: "demo util", Usage: "demo"}},
		"ConfigPatterns":  []string{"yaml config"},
	}
}
