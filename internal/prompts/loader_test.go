package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestLoader_RenderAllBuiltInPrompts(t *testing.T) {
	for _, agentName := range []string{"claude", "codex"} {
		t.Run(agentName, func(t *testing.T) {
			for _, locale := range []string{"en-US", "zh-CN"} {
				t.Run(locale, func(t *testing.T) {
					loader := NewLoader(agentName, locale, "")
					for _, tc := range []struct {
						name string
						data interface{}
					}{
						{"learn-analyze", sampleAnalyzeRequest()},
						{"learn-batch", sampleBatchLearnData()},
						{"fix-generate", sampleGenerateFixesRequest()},
						{"skill-project-summary", sampleGenerateSkillsData()},
						{"skill-project-init", sampleAnalyzeCurrentCodebaseRequest()},
						{"pattern-curate", sampleCuratePatternsData()},
						{"project-analyze", sampleProjectAnalysisData()},
						{"skill-workspace-profile", workspacePromptData()},
						{"skill-workspace-spec", workspaceSpecPromptData()},
					} {
						t.Run(tc.name, func(t *testing.T) {
							prompt, err := loader.Render(tc.name, tc.data)
							require.NoError(t, err)
							require.NotEmpty(t, prompt)
						})
					}
				})
			}
		})
	}
}

func TestLoader_RenderMissingMapKeyFails(t *testing.T) {
	loader := NewLoader("claude", "en-US", "")

	_, err := loader.Render("skill-project-summary", map[string]interface{}{
		"LANGUAGE":             "go",
		"PATTERNS_PATH":        "/tmp/skills-seed/patterns.json",
		"PATTERNS_COUNT":       0,
		"EXISTING_SKILLS_PATH": "",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "PROJECT_NAME")
}

func TestLoader_DefaultLocaleRendersChinesePrompt(t *testing.T) {
	loader := NewLoader("common", "", "")

	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())

	require.NoError(t, err)
	require.Contains(t, prompt, "你是一位专业的代码质量审查专家")
	require.NotContains(t, prompt, "You are a professional code quality review expert")
}

func TestLoader_RendersSkillsPromptsWithSkillsLocale(t *testing.T) {
	loader := NewLoaderWithLocales("common", "zh-CN", "en-US", "")

	skillsPrompt, err := loader.Render("skill-project-summary", sampleGenerateSkillsData())
	require.NoError(t, err)
	require.Contains(t, skillsPrompt, "You are a code pattern analysis expert")
	require.NotContains(t, skillsPrompt, "你是一位代码模式分析专家")

	toolPrompt, err := loader.Render("fix-generate", sampleGenerateFixesRequest())
	require.NoError(t, err)
	require.Contains(t, toolPrompt, "You are a professional code fix expert")
	require.NotContains(t, toolPrompt, "你是一位专业的代码修复专家")
}

func TestLoader_RendersOutputContractGuardWithPromptLocale(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "instructions"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "instructions", "skill-project-summary.md"), []byte("USER SUMMARY INSTRUCTIONS"), 0644))

	loader := NewLoaderWithLocales("common", "zh-CN", "en-US", seedPath)
	prompt, err := loader.Render("skill-project-summary", sampleGenerateSkillsData())

	require.NoError(t, err)
	require.Contains(t, prompt, "Do not return Markdown, comments, explanations, or code fences")
	require.Contains(t, prompt, "All user-facing natural-language fields in the JSON must be written in English")
	require.NotContains(t, prompt, "不要使用 markdown 代码块包裹 JSON")
}

func TestLoader_RenderMergesProjectWorkspaceAndUserInstructions(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "project"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "workspace"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "instructions"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "project-profile.md"), []byte("PROJECT PROFILE"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "common.md"), []byte("COMMON PROJECT RULES"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "learn-analyze.md"), []byte("PROJECT LEARN ANALYZE CONTEXT"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "workspace", "learn-analyze.md"), []byte("WORKSPACE LEARN ANALYZE CONTEXT"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "instructions", "learn-analyze.md"), []byte("USER LEARN ANALYZE INSTRUCTIONS"), 0644))

	loader := NewLoader("common", "zh-CN", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	require.Contains(t, prompt, "PROJECT PROFILE")
	require.Contains(t, prompt, "COMMON PROJECT RULES")
	require.Contains(t, prompt, "PROJECT LEARN ANALYZE CONTEXT")
	require.Contains(t, prompt, "WORKSPACE LEARN ANALYZE CONTEXT")
	require.Contains(t, prompt, "USER LEARN ANALYZE INSTRUCTIONS")
	require.Less(t, strings.Index(prompt, "COMMON PROJECT RULES"), strings.Index(prompt, "PROJECT LEARN ANALYZE CONTEXT"))
	require.Less(t, strings.Index(prompt, "PROJECT LEARN ANALYZE CONTEXT"), strings.Index(prompt, "WORKSPACE LEARN ANALYZE CONTEXT"))
	require.Less(t, strings.Index(prompt, "WORKSPACE LEARN ANALYZE CONTEXT"), strings.Index(prompt, "USER LEARN ANALYZE INSTRUCTIONS"))
}

func TestLoader_RenderSkipsLegacyDefaultPromptScaffolds(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "project"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "instructions"), 0755))

	legacyCommon := `<!-- generated-by: skills-seed v0.7.0 -->
<!-- prompt-template-sha256: old -->
<!-- prompt-type: common -->
<!-- editable: true -->

# 项目专属约束

处理这个项目时，优先遵循当前仓库的真实结构、命名风格和已有模式

## 项目画像来源

请结合 ` + "`project-profile.md`" + ` 中记录的项目背景理解代码，不要输出适用于任意项目的泛化建议

## 额外要求

- 先遵循本项目现有结构
- 优先复用现有模式
- 仅在必要时引入新抽象
- 输出必须具体到当前项目
`
	legacyInstructions := `<!-- generated-by: skills-seed v0.7.0 -->
<!-- prompt-template-sha256: old -->
<!-- prompt-type: learn-analyze -->
<!-- editable: true -->
<!-- prompt-merge: append -->
<!-- priority: user-instructions -->

# 用户补充指令

这些内容会追加到内置 ` + "`learn-analyze`" + ` 提示词之后，不会替换内置任务定义、输入约定或输出格式。

在此补充团队约束、编码偏好或特定场景规则。不要修改内置提示词要求的 JSON/Markdown 输出结构。
`
	profile := `<!-- generated-by: skills-seed v0.7.0 -->
<!-- prompt-template-sha256: old -->
<!-- prompt-type: project-profile -->
<!-- editable: true -->

# 项目画像

- 项目名称: demo
- 主要语言: go
- 项目根目录: /tmp/demo

## 目录结构摘要

` + "```text\n/tmp/demo\n├── README.md\n└── main.go\n```\n" + `
## 架构摘要

未记录

## 关键模块

未记录

## 团队编码风格

未记录
`
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "common.md"), []byte(legacyCommon), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "instructions", "learn-analyze.md"), []byte(legacyInstructions), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "project-profile.md"), []byte(profile), 0644))

	loader := NewLoader("common", "zh-CN", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	require.NotContains(t, prompt, "这些内容会追加到内置")
	require.NotContains(t, prompt, "在此补充团队约束")
	require.NotContains(t, prompt, "处理这个项目时，优先遵循当前仓库")
	require.NotContains(t, prompt, "prompt-template-sha256")
	require.NotContains(t, prompt, "未记录")
	require.NotContains(t, prompt, "README.md")
	require.Contains(t, prompt, "项目名称: demo")
	require.Contains(t, prompt, "主要语言: go")
}

func TestLoader_RenderAppendsOutputContractAfterUserInstructions(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "instructions"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "instructions", "learn-analyze.md"), []byte("USER SAYS RETURN MARKDOWN"), 0644))

	loader := NewLoader("common", "en-US", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)
	guard, err := NewLoader("common", "en-US", "").Render("output-contract-guard", map[string]interface{}{})
	require.NoError(t, err)
	guard = strings.TrimSpace(guard)

	require.Contains(t, prompt, "USER SAYS RETURN MARKDOWN")
	require.Contains(t, prompt, guard)
	require.Less(t, strings.Index(prompt, "USER SAYS RETURN MARKDOWN"), strings.LastIndex(prompt, guard))
	require.True(t, strings.HasSuffix(prompt, guard))
}

func TestLoader_RenderStoresSuccessfulPromptUnderRuntimeMemory(t *testing.T) {
	seedPath := t.TempDir()
	loader := NewLoader("common", "en-US", seedPath)

	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	runtimeDir := filepath.Join(seedPath, "memory", "runtime", "rendered-prompts")
	entries, err := os.ReadDir(runtimeDir)
	require.NoError(t, err)
	var renderedName string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			renderedName = entry.Name()
		}
	}
	require.NotEmpty(t, renderedName)
	require.True(t, strings.HasPrefix(renderedName, "learn-analyze-"))

	content, err := os.ReadFile(filepath.Join(runtimeDir, renderedName))
	require.NoError(t, err)
	require.Equal(t, prompt, strings.TrimSuffix(string(content), "\n"))
	require.Contains(t, string(content), "You are a professional code quality review expert")
}

func TestLoader_RenderStoresPromptDebugManifest(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "instructions"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "instructions", "learn-analyze.md"), []byte(`# 用户补充指令

这些内容会追加到内置 `+"`learn-analyze`"+` 提示词之后，不会替换内置任务定义、输入约定或输出格式。
`), 0644))

	loader := NewLoader("common", "zh-CN", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	runtimeDir := filepath.Join(seedPath, "memory", "runtime", "rendered-prompts")
	entries, err := os.ReadDir(runtimeDir)
	require.NoError(t, err)
	var manifestPath string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".manifest.json") {
			manifestPath = filepath.Join(runtimeDir, entry.Name())
		}
	}
	require.NotEmpty(t, manifestPath)
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, "learn-analyze", manifest["template"])
	require.EqualValues(t, len(prompt), manifest["final_length"])
	require.NotEmpty(t, manifest["parts"])
}

func TestRenderInitSkillsListsSamplePathsWithoutEmbeddedContent(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.SampleFiles = []agent.SampleFile{{
		Path: "webshell.go",
	}}

	prompt, err := loader.Render("skill-project-init", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "webshell.go")
	require.NotContains(t, prompt, "secretEmbeddedContent")
}

func TestRenderAnalyzeListsFilePathsWithoutEmbeddedContent(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleAnalyzeRequest()
	req.Files = []domain.FileInfo{domain.NewFileInfo("main.go", "package main\nconst secretAnalyzeContent = true\n")}
	req.Patterns[0].GoodExample = "const secretGoodExample = true"
	req.Patterns[0].BadExample = "const secretBadExample = true"

	prompt, err := loader.Render("learn-analyze", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "main.go")
	require.NotContains(t, prompt, "secretAnalyzeContent")
	require.NotContains(t, prompt, "secretGoodExample")
	require.NotContains(t, prompt, "secretBadExample")
}

func TestRenderAnalyzeListsDiffFilePaths(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleAnalyzeRequest()
	req.Files = []domain.FileInfo{domain.NewFileInfo("new.go", "package main\n")}
	req.DiffFiles = []agent.DiffFileRef{{
		Path:     "changed.go",
		DiffPath: "/tmp/skills-seed/runtime/diffs/changed.go.diff",
	}}

	prompt, err := loader.Render("learn-analyze", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "变更文件 Diff (1 个)")
	require.Contains(t, prompt, "changed.go")
	require.Contains(t, prompt, "/tmp/skills-seed/runtime/diffs/changed.go.diff")
}

func TestRenderInitSkillsListsDiffFilePaths(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.DiffFiles = []agent.DiffFileRef{{
		Path:     "internal/service.go",
		DiffPath: "/tmp/skills-seed/runtime/diffs/internal/service.go.diff",
	}}

	prompt, err := loader.Render("skill-project-init", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "变更文件 Diff (1 个)")
	require.Contains(t, prompt, "internal/service.go")
	require.Contains(t, prompt, "/tmp/skills-seed/runtime/diffs/internal/service.go.diff")
}

func TestRenderGenerateFixesListsFilePathsWithoutEmbeddedContent(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleGenerateFixesRequest()
	req.Files = []domain.FileInfo{domain.NewFileInfo("main.go", "package main\nconst secretFixContent = true\n")}

	prompt, err := loader.Render("fix-generate", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "main.go")
	require.NotContains(t, prompt, "secretFixContent")
}

func TestRenderProjectAnalysisListsReadmePathWithoutEmbeddedContent(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["ReadmePath"] = "README.md"

	prompt, err := loader.Render("project-analyze", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "README.md")
	require.NotContains(t, prompt, "secret readme content")
}

func TestRenderProjectAnalysisIncludesIncrementalProfileGuidance(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["ExistingProfilePath"] = "/tmp/skills-seed/existing-profile.json"
	data["FocusPaths"] = []string{"internal/service"}

	prompt, err := loader.Render("project-analyze", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "已有项目画像")
	require.Contains(t, prompt, "internal/service")
	require.Contains(t, prompt, "/tmp/skills-seed/existing-profile.json")
	require.NotContains(t, prompt, "Clean Architecture")
	require.Contains(t, prompt, "完整项目画像")
}

func TestRenderProjectAnalysisIncludesStructuralContext(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["StructuralContextPath"] = "/tmp/skills-seed/structural-context.md"

	prompt, err := loader.Render("project-analyze", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "结构化上下文")
	require.Contains(t, prompt, "/tmp/skills-seed/structural-context.md")
	require.NotContains(t, prompt, "handler calls service")
	require.Contains(t, prompt, "结构化")
}

func TestRenderInitSkillsIncludesStructuralContext(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.StructuralContextPath = "/tmp/skills-seed/structural-context.md"

	prompt, err := loader.Render("skill-project-init", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "结构化上下文")
	require.Contains(t, prompt, "/tmp/skills-seed/structural-context.md")
	require.NotContains(t, prompt, "service has callers")
	require.Contains(t, prompt, "结构化")
}

func TestRenderInitSkillsIncludesKnownPatterns(t *testing.T) {
	tests := []struct {
		locale string
		label  string
	}{
		{locale: "zh-CN", label: "已有模式"},
		{locale: "en-US", label: "Existing Patterns"},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := NewLoader("codex", tt.locale, "")
			req := sampleAnalyzeCurrentCodebaseRequest()
			req.KnownPatternsPath = "/tmp/skills-seed/known-patterns.json"
			req.KnownPatternsCount = 1

			prompt, err := loader.Render("skill-project-init", req)

			require.NoError(t, err)
			require.Contains(t, prompt, tt.label)
			require.Contains(t, prompt, "/tmp/skills-seed/known-patterns.json")
			require.NotContains(t, prompt, `"name":"Known Pattern"`)
		})
	}
}

func TestLoader_RenderLearningPromptsIncludeRichBusinessExtractionGuidance(t *testing.T) {
	tests := []struct {
		locale       string
		requiredText []string
	}{
		{
			locale: "zh-CN",
			requiredText: []string{
				"业务候选清单",
				"业务模式展开",
				"不要把多个业务细节压缩成一个泛化模式",
				"允许中英文混合表达技术概念",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"business candidate inventory",
				"business pattern expansion",
				"Do not compress multiple business details into one generic pattern",
				"All user-facing natural-language fields must be written in English",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := NewLoader("common", tt.locale, "")

			for _, tc := range []struct {
				name string
				data interface{}
			}{
				{"skill-project-init", sampleAnalyzeCurrentCodebaseRequest()},
				{"learn-batch", sampleBatchLearnData()},
			} {
				t.Run(tc.name, func(t *testing.T) {
					prompt, err := loader.Render(tc.name, tc.data)
					require.NoError(t, err)

					lowerPrompt := strings.ToLower(prompt)
					for _, text := range tt.requiredText {
						require.Contains(t, lowerPrompt, strings.ToLower(text))
					}
				})
			}
		})
	}
}

func TestLoader_RenderLearningPromptsPreferSpecificCategoriesOverBusinessFallback(t *testing.T) {
	tests := []struct {
		locale       string
		requiredText []string
		forbidden    []string
	}{
		{
			locale: "zh-CN",
			requiredText: []string{
				"具体分类优先",
				"business 不是默认分类",
				"不要为每个候选强行生成 business 模式",
			},
			forbidden: []string{
				"对每个业务候选至少尝试提取 1 个 `business` 模式",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"Prefer specific categories",
				"business is not the default category",
				"Do not force a business pattern for every candidate",
			},
			forbidden: []string{
				"try to extract at least one `business` pattern",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := NewLoader("common", tt.locale, "")

			for _, tc := range []struct {
				name string
				data interface{}
			}{
				{"skill-project-init", sampleAnalyzeCurrentCodebaseRequest()},
				{"learn-batch", sampleBatchLearnData()},
			} {
				t.Run(tc.name, func(t *testing.T) {
					prompt, err := loader.Render(tc.name, tc.data)
					require.NoError(t, err)

					lowerPrompt := strings.ToLower(prompt)
					for _, text := range tt.requiredText {
						require.Contains(t, lowerPrompt, strings.ToLower(text))
					}
					for _, text := range tt.forbidden {
						require.NotContains(t, lowerPrompt, strings.ToLower(text))
					}
				})
			}
		})
	}
}

func TestLoader_RenderPatternPromptsIncludePreOutputValidation(t *testing.T) {
	tests := []struct {
		locale       string
		requiredText []string
	}{
		{
			locale: "zh-CN",
			requiredText: []string{
				"输出前验证清单",
				"证据校验",
				"分类校验",
				"任何候选未通过必需校验时，不要输出",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"Pre-Output Validation Checklist",
				"Evidence check",
				"Category check",
				"If a candidate fails any required validation check, do not output it",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := NewLoader("common", tt.locale, "")
			for _, tc := range []struct {
				name string
				data interface{}
			}{
				{"learn-batch", sampleBatchLearnData()},
				{"skill-project-init", sampleAnalyzeCurrentCodebaseRequest()},
			} {
				t.Run(tc.name, func(t *testing.T) {
					prompt, err := loader.Render(tc.name, tc.data)
					require.NoError(t, err)
					for _, text := range tt.requiredText {
						require.Contains(t, prompt, text)
					}
				})
			}
		})
	}
}

func TestLoader_RenderUserPatternAndMergePromptsIncludePreOutputValidation(t *testing.T) {
	tests := []struct {
		locale string
		checks map[string][]string
	}{
		{
			locale: "zh-CN",
			checks: map[string][]string{
				"user-define-pattern": {
					"输出前验证清单",
					"如果提供了关联文件",
					"不要填写虚假的 `business_method`",
				},
				"pattern-curate": {
					"输出前验证清单",
					"每个候选模式都必须被 `patterns[].merged_from` 覆盖",
					"`summary.total_candidates",
				},
			},
		},
		{
			locale: "en-US",
			checks: map[string][]string{
				"user-define-pattern": {
					"Pre-Output Validation Checklist",
					"If related files are provided",
					"do not fill a fake `business_method`",
				},
				"pattern-curate": {
					"Pre-Output Validation Checklist",
					"Every candidate pattern must be covered by `patterns[].merged_from`",
					"`summary.total_candidates",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := NewLoader("common", tt.locale, "")
			for name, requiredText := range tt.checks {
				t.Run(name, func(t *testing.T) {
					data := interface{}(sampleUserDefinePatternData())
					if name == "pattern-curate" {
						data = sampleCuratePatternsData()
					}
					prompt, err := loader.Render(name, data)
					require.NoError(t, err)
					for _, text := range requiredText {
						require.Contains(t, prompt, text)
					}
				})
			}
		})
	}
}

func TestLoader_RenderZhProjectAnalysisRequiresChineseNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")

	prompt, err := loader.Render("project-analyze", sampleProjectAnalysisData())
	require.NoError(t, err)

	require.Contains(t, prompt, "面向用户阅读的自然语言字段应优先使用简体中文")
	require.Contains(t, prompt, "允许中英文混合表达技术概念")
	require.Contains(t, prompt, "Cobra、Viper、goctl")
}

func TestLoader_RenderZhGenerateSkillsSummaryRequiresChineseNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")

	prompt, err := loader.Render("skill-project-summary", sampleGenerateSkillsData())
	require.NoError(t, err)

	require.Contains(t, prompt, "面向用户阅读的自然语言字段应优先使用简体中文")
	require.Contains(t, prompt, "允许中英文混合表达技术概念")
	require.Contains(t, prompt, "英文专有名词可保留原文")
	require.NotContains(t, prompt, "用户提供的上下文")
	require.NotContains(t, prompt, "USER_CONTEXT_PATH")
}

func TestLoader_RenderEnProjectAnalysisRequiresEnglishNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "en-US", "")

	prompt, err := loader.Render("project-analyze", sampleProjectAnalysisData())
	require.NoError(t, err)

	require.Contains(t, prompt, "All user-facing natural-language fields must be written in English")
	require.Contains(t, prompt, "`framework_patterns` must describe framework usage in English")
	require.Contains(t, prompt, "Do not output Chinese sentences such as “Cobra 命令模式”")
}

func TestLoader_RenderEnGenerateSkillsSummaryRequiresEnglishNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "en-US", "")

	prompt, err := loader.Render("skill-project-summary", sampleGenerateSkillsData())
	require.NoError(t, err)

	require.Contains(t, prompt, "All user-facing natural-language fields must be written in English")
	require.Contains(t, prompt, "If input patterns, existing Skills, README text, code comments, or user-provided context contain Chinese")
	require.Contains(t, prompt, "Do not output Chinese phrases such as “业务流程”")
}

func TestLoader_RenderEnPersistentPromptsRequireEnglishNaturalLanguage(t *testing.T) {
	loader := NewLoader("common", "en-US", "")

	for _, tc := range []struct {
		name string
		data interface{}
	}{
		{"learn-batch", sampleBatchLearnData()},
		{"pattern-curate", sampleCuratePatternsData()},
		{"skill-project-init", sampleAnalyzeCurrentCodebaseRequest()},
		{"skill-project-summary", sampleGenerateSkillsData()},
		{"skill-workspace-profile", workspacePromptData()},
		{"skill-workspace-spec", workspaceSpecPromptData()},
		{"user-define-pattern", sampleUserDefinePatternData()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompt, err := loader.Render(tc.name, tc.data)
			require.NoError(t, err)
			require.Contains(t, prompt, "All user-facing natural-language fields must be written in English")
		})
	}
}

func TestRenderWorkspacePromptsDoNotIncludeRuntimeInputFilePaths(t *testing.T) {
	data := WorkspacePromptData{
		WorkspaceName: "hsm-workspace",
		WorkspaceRoot: "/tmp/hsm-workspace",
		Projects: []WorkspacePromptProject{
			{ID: "hsmwebapi", Path: "hsmwebapi", Type: "backend", Language: "go"},
		},
		Locale: "zh-CN",
	}

	profile, err := renderWorkspaceTemplate("skill-workspace-profile", "zh-CN", data)
	require.NoError(t, err)
	spec, err := renderWorkspaceTemplate("skill-workspace-spec", "zh-CN", data)
	require.NoError(t, err)

	require.Contains(t, profile, "`hsmwebapi`: `hsmwebapi`")
	require.NotContains(t, profile, "<workspace-input-file>")
	require.NotContains(t, profile, "<workspace-profile-file>")
	require.NotContains(t, profile, "<user-context-file>")
	require.NotContains(t, profile, "workspace-input.json")
	require.NotContains(t, profile, "workspace-profile.json")
	require.NotContains(t, profile, "user-context.md")
	require.NotContains(t, profile, "hsmwebapi 是主后端")
	require.NotContains(t, spec, "<workspace-input-file>")
	require.NotContains(t, spec, "<workspace-profile-file>")
	require.NotContains(t, spec, "<user-context-file>")
	require.NotContains(t, spec, "workspace-input.json")
	require.NotContains(t, spec, "workspace-profile.json")
	require.NotContains(t, spec, "user-context.md")
	require.NotContains(t, spec, "kmip-go 提供 KMIP 能力")
}

func TestRenderWorkspacePromptsIncludeLearnUserContextPathWhenProvided(t *testing.T) {
	profileData := workspacePromptData()
	profileData["UserContextPath"] = "/tmp/skills-seed/user-context.md"
	specData := workspaceSpecPromptData()
	specData["UserContextPath"] = "/tmp/skills-seed/user-context.md"

	loader := NewLoader("common", "zh-CN", "")
	profile, err := loader.Render("skill-workspace-profile", profileData)
	require.NoError(t, err)
	spec, err := loader.Render("skill-workspace-spec", specData)
	require.NoError(t, err)

	require.Contains(t, profile, "/tmp/skills-seed/user-context.md")
	require.Contains(t, profile, "不要把说明文件原文")
	require.Contains(t, spec, "/tmp/skills-seed/user-context.md")
	require.Contains(t, spec, "不要把说明文件原文")
}

func TestLoader_RenderBatchLearnUsesCommitHashesWithoutDiffs(t *testing.T) {
	loader := NewLoader("common", "zh-CN", "")

	prompt, err := loader.Render("learn-batch", sampleBatchLearnData())
	require.NoError(t, err)

	require.Contains(t, prompt, "abcdef123456")
	require.NotContains(t, prompt, "diff --git a/internal/demo.go b/internal/demo.go")
	require.NotContains(t, prompt, "func Demo() error")
	require.Contains(t, prompt, "/tmp/skills-seed/known-patterns.json")
	require.NotContains(t, prompt, `"name":"Known Pattern"`)
}

func TestLoader_RuntimePromptsFenceJSONExamplesButRequireUnfencedResponses(t *testing.T) {
	loader := NewLoader("common", "zh-CN", "")
	for _, tc := range []struct {
		name string
		data interface{}
	}{
		{"learn-analyze", sampleAnalyzeRequest()},
		{"learn-batch", sampleBatchLearnData()},
		{"fix-generate", sampleGenerateFixesRequest()},
		{"skill-project-init", sampleAnalyzeCurrentCodebaseRequest()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompt, err := loader.Render(tc.name, tc.data)
			require.NoError(t, err)
			require.Contains(t, prompt, "```json")
			require.Contains(t, prompt, "不要使用 markdown 代码块（不要 ```)")
		})
	}
}

func TestLoader_RuntimePromptsBoundFileReadingScope(t *testing.T) {
	loader := NewLoader("common", "zh-CN", "")
	tests := []struct {
		name         string
		data         interface{}
		requiredText []string
		forbidden    []string
	}{
		{
			name: "learn-batch",
			data: sampleBatchLearnData(),
			requiredText: []string{
				"先读取变更文件",
				"仅在证据不足时扩展到直接调用方、被调用方或同目录测试",
				"不要全仓库扫描",
			},
			forbidden: []string{
				"查看提交涉及的完整代码与变更",
			},
		},
		{
			name: "learn-analyze",
			data: sampleAnalyzeRequest(),
			requiredText: []string{
				"先读取待分析文件",
				"判断模式违规所必需",
			},
			forbidden: []string{
				"读取每个文件的完整内容",
			},
		},
		{
			name: "skill-project-init",
			data: sampleAnalyzeCurrentCodebaseRequest(),
			requiredText: []string{
				"优先读取结构化上下文",
				"只扩展到能支持模式判断的直接相关文件",
				"避免全仓库扫描",
			},
			forbidden: []string{
				"逐个扫描示例文件中的 Service",
			},
		},
		{
			name: "fix-generate",
			data: sampleGenerateFixesRequest(),
			requiredText: []string{
				"只返回需要修改的文件",
				"无法安全完整重写",
				"warnings",
			},
			forbidden: []string{
				"读取相关文件的完整内容",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prompt, err := loader.Render(tc.name, tc.data)
			require.NoError(t, err)
			for _, text := range tc.requiredText {
				require.Contains(t, prompt, text)
			}
			for _, text := range tc.forbidden {
				require.NotContains(t, prompt, text)
			}
		})
	}
}

func TestLoader_ProjectInitPromptDoesNotHardCodeFrameworkCatalog(t *testing.T) {
	loader := NewLoader("common", "zh-CN", "")

	prompt, err := loader.Render("skill-project-init", sampleAnalyzeCurrentCodebaseRequest())

	require.NoError(t, err)
	require.NotContains(t, prompt, "Gin/Echo/Beego/Fiber/Spring Boot/Express/Django")
	require.NotContains(t, prompt, "GORM/Ent/XORM/SQLAlchemy/TypeORM/MyBatis")
	require.Contains(t, prompt, "只提取项目实际使用的框架")
}

func TestLoader_ZhSkillSummaryUsesCorrectConcurrencySpelling(t *testing.T) {
	loader := NewLoader("common", "zh-CN", "")

	prompt, err := loader.Render("skill-project-summary", sampleGenerateSkillsData())

	require.NoError(t, err)
	require.Contains(t, prompt, "concurrency")
	require.NotContains(t, prompt, "conurrency")
}

func sampleAnalyzeRequest() *agent.AnalyzeRequest {
	return &agent.AnalyzeRequest{
		Files: []domain.FileInfo{domain.NewFileInfo("main.go", "package main\nfunc main() {}\n")},
		Context: agent.ProjectContext{
			Name:         "demo",
			Language:     "go",
			Frameworks:   []string{"cobra"},
			Dependencies: []string{"bbolt"},
		},
		Patterns: []domain.Pattern{*samplePattern()},
		RecentCommits: []domain.CommitInfo{
			domain.NewCommitInfo("abcdef123456", "dev", "feat: add demo", time.Now()),
		},
	}
}

func sampleBatchLearnData() map[string]interface{} {
	return map[string]interface{}{
		"Commits": []domain.CommitInfo{
			domain.NewCommitInfo("abcdef123456", "dev", "feat: add demo", time.Now()),
		},
		"CommitFiles": []agent.CommitFileChange{
			{
				Commit: domain.NewCommitInfo("abcdef123456", "dev", "feat: add demo", time.Now()),
				Files:  []string{"internal/demo.go"},
			},
		},
		"KnownPatternsPath":  "/tmp/skills-seed/known-patterns.json",
		"KnownPatternsCount": 1,
	}
}

func sampleGenerateFixesRequest() *agent.GenerateFixesRequest {
	return &agent.GenerateFixesRequest{
		Issues: []domain.Issue{
			{File: "main.go", Line: 1, Severity: domain.SeverityWarning, Message: "demo issue", Suggestion: "fix it"},
		},
		Files: []domain.FileInfo{domain.NewFileInfo("main.go", "package main\n")},
		Context: agent.ProjectContext{
			Name:         "demo",
			Language:     "go",
			Frameworks:   []string{"cobra"},
			Dependencies: []string{"bbolt"},
		},
	}
}

func sampleGenerateSkillsData() map[string]interface{} {
	return map[string]interface{}{
		"PROJECT_NAME":         "demo",
		"LANGUAGE":             "go",
		"PATTERNS_PATH":        "/tmp/skills-seed/patterns.json",
		"PATTERNS_COUNT":       1,
		"EXISTING_SKILLS_PATH": "",
	}
}

func sampleAnalyzeCurrentCodebaseRequest() *agent.AnalyzeCurrentCodebaseRequest {
	return &agent.AnalyzeCurrentCodebaseRequest{
		ProjectName:   "demo",
		RootPath:      "/tmp/demo",
		Language:      "go",
		StructurePath: "/tmp/skills-seed/project-structure.txt",
		MainFiles:     []string{"cmd/demo/main.go"},
		SampleFiles:   []agent.SampleFile{{Path: "cmd/demo/main.go"}},
	}
}

func sampleCuratePatternsData() map[string]interface{} {
	candidate := *samplePattern()
	candidate.ID = "candidate-pattern"
	existing := *samplePattern()
	existing.ID = "existing-pattern"
	return map[string]interface{}{
		"Operation":           "learn_current",
		"CandidatePatterns":   []domain.Pattern{candidate},
		"ExistingPatterns":    []domain.Pattern{existing},
		"AllExisting":         false,
		"ExistingByCandidate": map[string][]string{"candidate-pattern": []string{"existing-pattern"}},
	}
}

func sampleProjectAnalysisData() map[string]interface{} {
	return map[string]interface{}{
		"ProjectName":           "demo",
		"RootPath":              "/tmp/demo",
		"Language":              "go",
		"StructurePath":         "/tmp/skills-seed/project-structure.txt",
		"StructuralContextPath": "",
		"ReadmePath":            "README.md",
		"MainFiles":             []string{"cmd/demo/main.go"},
		"ExistingProfilePath":   "",
		"FocusPaths":            []string{},
		"UserContextPath":       "",
	}
}

func sampleUserDefinePatternData() map[string]interface{} {
	return map[string]interface{}{
		"Language":    "go",
		"Files":       []string{"internal/demo.go"},
		"UserContext": "团队希望把中文说明改写为目标语言",
		"Description": "新增接口时复用现有 mapper",
		"Category":    "structure",
	}
}

func workspacePromptData() map[string]interface{} {
	return map[string]interface{}{
		"WorkspaceName":      "demo-workspace",
		"WorkspaceRoot":      "/tmp/demo-workspace",
		"WorkspaceInputPath": "/tmp/skills-seed/workspace-input.json",
		"UserContextPath":    "",
	}
}

func workspaceSpecPromptData() map[string]interface{} {
	return map[string]interface{}{
		"WorkspaceName":        "demo-workspace",
		"WorkspaceRoot":        "/tmp/demo-workspace",
		"WorkspaceInputPath":   "/tmp/skills-seed/workspace-input.json",
		"WorkspaceProfilePath": "/tmp/skills-seed/workspace-profile.json",
		"UserContextPath":      "",
	}
}

func samplePattern() *domain.Pattern {
	p := domain.NewPattern("p1", "Demo Pattern", domain.CategoryAPI)
	p.SetDescription("demo pattern")
	p.SetRule("use demo pattern")
	p.SetExamples("good()", "bad()")
	p.Confidence = 0.9
	p.Frequency = 2
	return p
}
