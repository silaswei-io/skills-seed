package skills

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/metadata"
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
		"SkillDescription":    "修改 test-project go 代码且涉及项目约定时使用",
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
		"KeyInsights": []string{
			"错误处理是跨层一致性核心",
		},
		"ImprovementSuggestions": []string{
			"为外部调用补充超时测试",
		},
		"OverviewReferences": []ReferenceItem{
			{Title: "业务方法", Path: "./references/business-methods.md", Description: "项目级业务入口和可复用方法索引"},
		},
		"Workflows": []map[string]interface{}{
			{
				"Title":       "新增或调整 API",
				"AppliesWhen": "接口或生成层变化时",
				"Steps":       []string{"修改接口定义。", "运行生成命令。"},
			},
		},
		"WorkflowReferences": []map[string]string{
			{"Name": "部署工作流", "Path": "./workflows/deploy.md", "Description": "发布前后检查"},
		},
		"ValidationCommands": []map[string]string{
			{"Command": "task verify", "When": "项目代码变化后", "Source": "Taskfile.yml"},
		},
		"StateSummaries": []string{"Task: 保持任务状态迁移。"},
		"References":     fullReferenceAvailability(),
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

	content, err := loader.Render("project-skill", data)

	assert.NoError(t, err)
	assert.NotEmpty(t, content)
	// 验证关键内容是否存在
	assert.Contains(t, content, "test-project")
	assert.Contains(t, content, "go")
	assert.Contains(t, content, "10")
	assert.Contains(t, content, "generated-by: skills-seed v0.0.1")
	assert.Contains(t, content, "skills-template-sha256: test-hash")
	assert.Contains(t, content, "项目入口 skill")
	assert.Contains(t, content, "业务模式地图")
	assert.Contains(t, content, "常用工作流")
	assert.Contains(t, content, "部署工作流")
	assert.Contains(t, content, "task verify")
	assert.Contains(t, content, "错误处理是跨层一致性核心")
	assert.NotContains(t, content, "为外部调用补充超时测试")
}

// TestLoader_Render_English 测试英文模板渲染
func TestLoader_Render_English(t *testing.T) {
	loader := NewLoader("en-US")

	data := map[string]interface{}{
		"ProgramVersion":      "v0.0.1",
		"SkillsTemplatesHash": "test-hash",
		"ProjectName":         "test-project",
		"SkillName":           "test-project-dev",
		"SkillDescription":    "Use when modifying test-project go code involving project conventions",
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
		"KeyInsights":         []string{"Error handling is a cross-layer consistency concern"},
		"ImprovementSuggestions": []string{
			"Add timeout tests for external calls",
		},
		"OverviewReferences": []ReferenceItem{},
		"Workflows": []map[string]interface{}{
			{
				"Title":       "Add Or Change API",
				"AppliesWhen": "endpoints change",
				"Steps":       []string{"Change the API source files first."},
			},
		},
		"WorkflowReferences": []map[string]string{},
		"ValidationCommands": []map[string]string{
			{"Command": "task verify", "When": "project code changes", "Source": "Taskfile.yml"},
		},
		"StateSummaries":  []string{"Task: preserve task state transitions."},
		"References":      fullReferenceAvailability(),
		"ReferenceGroups": []ReferenceGroup{},
	}

	content, err := loader.Render("project-skill", data)

	assert.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.Contains(t, content, "test-project")
	assert.Contains(t, content, "skills-template-sha256: test-hash")
	assert.Contains(t, content, "project entry skill")
	assert.Contains(t, content, "Common Workflows")
	assert.Contains(t, content, "task verify")
	assert.Contains(t, content, "Error handling is a cross-layer consistency concern")
	assert.NotContains(t, content, "Add timeout tests for external calls")
}

func TestLoader_RenderZhSkillFrontmatterDescriptionIsLocalized(t *testing.T) {
	for _, agentName := range []string{"common", "claude", "codex"} {
		t.Run(agentName, func(t *testing.T) {
			loader := NewLoaderForAgent(agentName, "zh-CN")

			content, err := loader.Render("project-skill", fullSkillData())
			require.NoError(t, err)

			require.Contains(t, content, "description: 修改、审查或扩展 demo go 代码且涉及项目特定约定时使用")
			require.NotContains(t, content, "description: Use when")
		})
	}
}

func TestLoader_DefaultLocaleRendersEnglishSkill(t *testing.T) {
	loader := NewLoaderForAgent("codex", "")
	data := fullSkillData()
	data["SkillDescription"] = "Use when modifying, reviewing, or extending demo go code involving project-specific conventions"

	content, err := loader.Render("project-skill", data)

	require.NoError(t, err)
	require.Contains(t, content, "description: Use when modifying, reviewing, or extending demo go code involving project-specific conventions")
	require.NotContains(t, content, "description: 修改、审查或扩展")
}

func TestLoader_RenderWorkspaceSkillFromEmbedTemplate(t *testing.T) {
	loader := NewLoaderForAgent("codex", "zh-CN")
	data := map[string]interface{}{
		"ProgramVersion":      "v0.0.4",
		"SkillsTemplatesHash": "hash",
		"SkillName":           "demo-workspace",
		"WorkspaceName":       "demo",
		"ProjectCount":        1,
		"Projects": []map[string]interface{}{
			{
				"ID":              "backend",
				"Path":            "backend",
				"Type":            "backend",
				"Language":        "go",
				"SkillName":       "backend-dev",
				"SkillPath":       "backend/.agents/skills/backend-dev/SKILL.md",
				"ProjectSpecPath": "backend/.agents/skills/backend-dev/references/project-spec.md",
				"SelfManaged":     false,
			},
		},
		"Shared":             []map[string]string{},
		"Contracts":          []map[string]string{{"Path": "proto"}},
		"Infra":              []map[string]string{},
		"HasShared":          false,
		"HasContracts":       true,
		"HasInfra":           false,
		"WorkflowReferences": []map[string]string{},
	}

	content, err := loader.Render("workspace-skill", data)
	require.NoError(t, err)

	require.Contains(t, content, "description: 修改、审查或扩展 demo 工作区代码时使用")
	require.Contains(t, content, "[工作区概览](./references/workspace-overview.md)")
	require.Contains(t, content, "[跨项目规则](./references/cross-project-rules.md)")
	require.NotContains(t, content, "backend/.agents/skills/backend-dev/SKILL.md")
	require.NotContains(t, content, "影响范围判断")
	require.NotContains(t, content, "Use when modifying")
}

func TestLoader_OmitsVisibleSkillsSeedGeneratedNoticeByDefault(t *testing.T) {
	loader := NewLoader("zh-CN")

	content, err := loader.RenderPattern("business", map[string]interface{}{
		"Category":          "business",
		"Priority":          1,
		"PatternCount":      9,
		"Confidence":        90.0,
		"LastUpdated":       "2026-06-08 12:00:00",
		"Summary":           "业务模式",
		"PatternObjects":    []domain.Pattern{*domain.NewPattern("p1", "业务规则", domain.CategoryBusiness)},
		"UsageScenes":       []string{},
		"CodeFenceLanguage": "go",
	})
	require.NoError(t, err)

	require.NotContains(t, content, "此文档由 skills-seed 从")
	require.NotContains(t, content, "自动生成")
}

// TestLoader_RenderReference 测试分类模板渲染
func TestLoader_RenderReference(t *testing.T) {
	loader := NewLoader("zh-CN")

	data := map[string]interface{}{
		"Category":          "api",
		"Summary":           "API 相关的编码模式",
		"Patterns":          []string{"API 路由命名规范", "错误处理模式"},
		"PatternObjects":    []domain.Pattern{*domain.NewPattern("p1", "API 路由命名规范", domain.CategoryAPI)},
		"UsageScenes":       []string{"创建新 API 端点时", "重构 API 路由时"},
		"Priority":          4,
		"PatternCount":      2,
		"Confidence":        87.5,
		"LastUpdated":       "2026-03-25 15:00:00",
		"BusinessMethods":   []interface{}{}, // 添加业务方法
		"CodeFenceLanguage": "go",
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

func TestLoader_RenderReferencesUseConfiguredCodeFenceLanguage(t *testing.T) {
	loader := NewLoader("zh-CN")
	pattern := domain.NewPattern("p1", "API Pattern", domain.CategoryAPI)
	pattern.SetExamples("function demo() {\n  return true\n}", "")

	data := categoryData("api")
	data["CodeFenceLanguage"] = "typescript"
	data["PatternObjects"] = []domain.Pattern{*pattern}

	content, err := loader.RenderPattern("api", data)

	require.NoError(t, err)
	require.Contains(t, content, "```typescript")
	require.NotContains(t, content, "```go")
}

func TestLoader_RenderBusinessReferencesIncludeRequestLanguageRouting(t *testing.T) {
	tests := []struct {
		locale       string
		indexChecks  []string
		detailChecks []string
	}{
		{
			locale: "zh-CN",
			indexChecks: []string{
				"用户动作",
				"规则策略",
				"不要只按代码包名或技术缩写匹配",
				"同义业务动作/状态变化",
			},
			detailChecks: []string{
				"同义业务动作",
				"入口渠道或外部依赖",
				"定位提示",
				"先按本模式的证据路径定位代码",
			},
		},
		{
			locale: "en-US",
			indexChecks: []string{
				"user actions",
				"rules or policies",
				"Do not match only by package names or technical abbreviations",
				"synonymous business actions or state changes",
			},
			detailChecks: []string{
				"synonymous business actions",
				"entry channels, or external dependencies",
				"Routing Hint",
				"locate code through this pattern's evidence path first",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := NewLoader(tt.locale)
			pattern := domain.NewPattern("resource-approval", "资源审批策略", domain.CategoryBusiness)
			pattern.Description = "需求提到审批、通过、拒绝或待处理状态时读取 internal/service/resource.go"
			pattern.Rule = "调整资源审批时先检查状态流转和外部依赖回调。"

			data := map[string]interface{}{
				"PatternCount":      1,
				"Confidence":        90.0,
				"LastUpdated":       "2026-06-26 00:00:00",
				"Summary":           "",
				"UsageScenes":       []string{},
				"Category":          "business",
				"CodeFenceLanguage": "go",
				"Groups": []map[string]interface{}{
					{
						"ID":    "resource",
						"Title": "Resource",
						"Path":  "./business/resource.md",
						"Summary": map[string]interface{}{
							"Description": "资源业务规则",
							"Keywords":    []string{"审批", "状态"},
							"PrimaryPath": "internal/service/resource.go",
							"IsFallback":  false,
						},
						"Patterns": []domain.Pattern{*pattern},
					},
				},
				"GroupTitle": "Resource",
				"GroupSummary": map[string]interface{}{
					"Description": "资源业务规则",
					"Keywords":    []string{"审批", "状态"},
					"PrimaryPath": "internal/service/resource.go",
					"IsFallback":  false,
				},
				"GroupLocations": []interface{}{},
				"GroupSignals":   []string{},
				"PatternObjects": []domain.Pattern{*pattern},
			}

			index, err := loader.RenderRelative("project/references/patterns/business-index", data)
			require.NoError(t, err)
			for _, text := range tt.indexChecks {
				require.Contains(t, index, text)
			}

			detail, err := loader.RenderRelative("project/references/patterns/business-detail", data)
			require.NoError(t, err)
			for _, text := range tt.detailChecks {
				require.Contains(t, detail, text)
			}
		})
	}
}

func TestLoader_RenderPatternFallsBackToDefaultTemplate(t *testing.T) {
	loader := NewLoader("zh-CN")

	content, err := loader.RenderPattern("custom-domain", map[string]interface{}{
		"Category":          "custom-domain",
		"Summary":           "自定义分类模式",
		"Patterns":          []string{"Custom Pattern"},
		"PatternObjects":    []domain.Pattern{*domain.NewPattern("p1", "Custom Pattern", domain.Category("custom-domain"))},
		"UsageScenes":       []string{"自定义场景"},
		"Priority":          3,
		"PatternCount":      1,
		"Confidence":        80.0,
		"LastUpdated":       "2026-05-21 00:00:00",
		"BusinessMethods":   []*domain.BusinessMethod{},
		"CodeFenceLanguage": "go",
	})

	require.NoError(t, err)
	assert.Contains(t, content, "custom-domain")
	assert.Contains(t, content, "Custom Pattern")
}

// TestLoader_TemplateExists 测试模板存在性检查
func TestLoader_TemplateExists(t *testing.T) {
	loader := NewLoader("zh-CN")

	// 测试存在的模板
	exists := loader.TemplateExists("project-skill")
	assert.True(t, exists)

	exists = loader.TemplateExists("skill")
	assert.False(t, exists)

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

					mainContent, err := loader.Render("project-skill", fullSkillData())
					require.NoError(t, err)
					require.NotEmpty(t, mainContent)
					require.NotContains(t, mainContent, "skills-seed learn history")
					require.NotContains(t, mainContent, "skills-seed generate skills")
					require.NotContains(t, mainContent, "skills-seed generate-skills")

					overview, err := loader.Render("project-reference-overview", projectOverviewData())
					require.NoError(t, err)
					require.NotEmpty(t, overview)

					for _, reference := range []string{"business-methods", "modules", "common-utils", "project-spec"} {
						referenceContent, err := loader.Render("project-reference-"+reference, projectOverviewData())
						require.NoError(t, err)
						require.NotEmpty(t, referenceContent)
						require.NotContains(t, referenceContent, "skills-seed learn current")
						require.NotContains(t, referenceContent, "skills-seed generate skills")
						require.NotContains(t, referenceContent, "skills-seed generate-skills")
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
		"../../../embedfs/templates/skills/claude/references/project-overview.md.tmpl",
		"../../../embedfs/templates/skills/claude/references/patterns",
		"../../../embedfs/templates/skills/claude/references/examples",
		"../../../embedfs/templates/skills/codex/references/project-overview.md.tmpl",
		"../../../embedfs/templates/skills/codex/references/project-overview.md.tmpl",
		"../../../embedfs/templates/skills/codex/references/patterns",
		"../../../embedfs/templates/skills/codex/references/examples",
	} {
		_, err := os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist, path)
	}
}

func TestLoader_RenderMissingMapKeyFails(t *testing.T) {
	loader := NewLoader("en-US")

	_, err := loader.Render("project-skill", map[string]interface{}{
		"ProgramVersion":      "v0.0.1",
		"SkillsTemplatesHash": "test-hash",
		"ProjectName":         "demo",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "SkillName")
}

func TestSkillTemplateCatalogMapsLogicalIDsToNormalizedOutputs(t *testing.T) {
	projectSkill, ok := TemplateCatalogEntry("project-skill")
	require.True(t, ok)
	require.Equal(t, "project/project-skill", projectSkill.RelativeName)
	require.Equal(t, "SKILL.md", projectSkill.OutputPath)
	require.ElementsMatch(t, []string{"common", "claude", "codex"}, projectSkill.Providers)

	workspaceSkill, ok := TemplateCatalogEntry("workspace-skill")
	require.True(t, ok)
	require.Equal(t, "workspace/workspace-skill", workspaceSkill.RelativeName)
	require.Equal(t, "SKILL.md", workspaceSkill.OutputPath)
	require.ElementsMatch(t, []string{"common"}, workspaceSkill.Providers)

	openAI, ok := TemplateCatalogEntry("codex-agent-openai")
	require.True(t, ok)
	require.Equal(t, "project/agents/codex-agent-openai", openAI.RelativeName)
	require.Equal(t, "agents/openai.yaml", openAI.OutputPath)
	require.Equal(t, ".yaml.tmpl", openAI.Ext)
}

func TestSkillTemplateCatalogFilesExistAndUsePrefixNames(t *testing.T) {
	kebabID := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	entries := SkillTemplateCatalog()
	require.NotEmpty(t, entries)

	for _, entry := range entries {
		t.Run(entry.ID, func(t *testing.T) {
			require.Regexp(t, kebabID, entry.ID)
			require.NotContains(t, entry.RelativeName, "_")
			require.NotContains(t, entry.OutputPath, "_")
			require.NotContains(t, entry.RelativeName, "SKILL")
			require.NotContains(t, entry.RelativeName, ".")

			for _, provider := range entry.Providers {
				path := metadata.SkillsTemplatePath(provider, entry.RelativeName, "", entry.Ext)
				_, err := embedfs.FS.ReadFile(path)
				require.NoError(t, err, path)
				require.NotContains(t, path, "_")

				enPath := metadata.SkillsTemplatePath(provider, entry.RelativeName, "en-US", entry.Ext)
				_, err = embedfs.FS.ReadFile(enPath)
				require.NoError(t, err, enPath)
				require.NotContains(t, enPath, "_")
			}
		})
	}
}

func TestSkillTemplateTreeUsesChineseDefaultsAndNoZhCNFilenames(t *testing.T) {
	err := fs.WalkDir(embedfs.FS, metadata.SkillsTemplatesRoot, func(path string, entry fs.DirEntry, err error) error {
		require.NoError(t, err)
		if entry.IsDir() {
			return nil
		}
		require.NotContains(t, path, ".zh-CN.")
		require.False(t, strings.Contains(filepath.Base(path), "_"), path)
		return nil
	})
	require.NoError(t, err)
}

func fullSkillData() map[string]interface{} {
	return map[string]interface{}{
		"ProgramVersion":      "v0.0.1",
		"SkillsTemplatesHash": "test-hash",
		"ProjectName":         "demo",
		"SkillName":           "demo-dev",
		"SkillDescription":    "修改、审查或扩展 demo go 代码且涉及项目特定约定时使用",
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
		"KeyInsights":         []string{"key insight"},
		"ImprovementSuggestions": []string{
			"improvement suggestion",
		},
		"OverviewReferences": []ReferenceItem{
			{Title: "业务方法", Path: "./references/business-methods.md", Description: "完整业务方法清单"},
		},
		"Workflows": []map[string]interface{}{
			{
				"Title":       "新增或调整 API",
				"AppliesWhen": "接口或生成层变化时",
				"Steps":       []string{"修改接口定义。", "运行生成命令。"},
			},
		},
		"WorkflowReferences": []map[string]string{},
		"ValidationCommands": []map[string]string{
			{"Command": "task verify", "When": "项目代码变化后", "Source": "Taskfile.yml"},
		},
		"StateSummaries": []string{"Task: 保持任务状态迁移。"},
		"References":     fullReferenceAvailability(),
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
		CodeLocation:  domain.CodeLocation{CurrentLocation: "internal/demo.go:10"},
		Description:   "demo business method",
		Usage:         "use in demo flow",
		Type:          "domain",
		Function:      "func DemoMethod(ctx context.Context) error",
		Prerequisites: "initialized service",
		Returns:       "error when demo fails",
	}
	pattern.BusinessMethod = method

	return map[string]interface{}{
		"Category":          category,
		"Summary":           "demo summary",
		"Patterns":          []string{"Demo Pattern"},
		"PatternObjects":    []domain.Pattern{*pattern},
		"UsageScenes":       []string{"demo usage"},
		"Priority":          4,
		"PatternCount":      1,
		"Confidence":        90.0,
		"LastUpdated":       "2026-05-19 00:00:00",
		"BusinessMethods":   []*domain.BusinessMethod{method},
		"SamplePatterns":    []string{"Demo Pattern"},
		"CodeFenceLanguage": "go",
	}
}

func projectOverviewData() map[string]interface{} {
	return map[string]interface{}{
		"ProjectName":         "demo",
		"Language":            "go",
		"GeneratedAt":         "2026-05-19 00:00:00",
		"Summary":             "demo summary",
		"Architecture":        "layered",
		"OverviewSummary":     "demo summary",
		"ArchitectureSummary": "layered",
		"Layers":              []domain.ArchitectureLayer{{Name: "service", Description: "business", Responsibilities: []string{"orchestrate"}, Files: []string{"internal/service/demo.go"}}},
		"DependencyGraph":     "command -> service -> domain",
		"DataFlow":            "request -> service -> repository",
		"Frameworks":          []string{"cobra"},
		"Dependencies":        []string{"bbolt"},
		"FrameworkPatterns":   []string{"cobra commands"},
		"Structure":           "internal/",
		"OverviewReferences": []ReferenceItem{
			{Title: "业务方法", Path: "./business-methods.md", Description: "完整业务方法清单"},
			{Title: "关键模块", Path: "./modules.md", Description: "完整模块清单"},
			{Title: "通用工具", Path: "./common-utils.md", Description: "工具方法清单"},
		},
		"References":      fullReferenceAvailability(),
		"KeyModules":      []domain.ModuleInfo{{Name: "service", Path: "internal/service", Description: "business layer", Responsibilities: []string{"orchestrate"}, Dependencies: []string{"domain"}, Dependents: []string{"command"}, KeyMethods: []string{"Run()"}}},
		"BusinessMethods": []domain.BusinessMethod{{Name: "Demo", CodeLocation: domain.CodeLocation{CurrentLocation: "internal/demo.go:10"}, Description: "demo", Function: "func Demo()", Usage: "demo", Type: "domain"}},
		"CommonUtils":     []domain.UtilityFunction{{Name: "DemoUtil", File: "internal/utils/demo.go", Signature: "func DemoUtil()", Description: "demo util", Usage: "demo"}},
		"ConfigPatterns":  []string{"yaml config"},
		"ValidationCommands": []domain.ValidationCommand{
			{Command: "task verify", When: "项目代码变化后", Source: "Taskfile.yml"},
		},
		"CodeFenceLanguage":   "go",
		"ProjectID":           "demo",
		"ScopePath":           "demo",
		"WorkspaceRole":       "backend",
		"HasBusinessPatterns": true,
		"HasUtilityPatterns":  true,
		"Boundaries": []domain.ProjectSpecBoundary{
			{Type: "module", Name: "service", Description: "business layer", Responsibilities: []string{"orchestrate"}, Paths: []string{"internal/service"}},
		},
		"PatternRules": []domain.ProjectSpecPatternRule{
			{Name: "Error Wrapping", Category: "error", Description: "wrap errors", Rule: "use %w", Confidence: 0.9, Frequency: 2},
		},
		"PatternGuidance": []domain.ProjectSpecPatternRule{
			{Name: "Naming Observation", Category: "naming", Description: "names align", Rule: "prefer local names", Confidence: 0.7, Frequency: 1},
		},
		"Touchpoints": []domain.ProjectSpecTouchpoint{
			{Kind: "business_method", Name: "Demo", Path: "internal/demo.go:10", Description: "demo"},
		},
		"SourceOfTruth": []map[string]string{
			{"Area": "业务规则", "Edit": "`internal/service`", "DoNotEdit": "generated files"},
		},
	}
}

func fullReferenceAvailability() map[string]bool {
	return map[string]bool{
		"Enabled":          true,
		"ProjectSpec":      true,
		"ProjectOverview":  true,
		"BusinessMethods":  true,
		"KeyModules":       true,
		"CommonUtils":      true,
		"BusinessPatterns": true,
	}
}
