package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestLoader_RenderAllBuiltInPrompts(t *testing.T) {
	for _, agentName := range []string{"claude", "codex"} {
		t.Run(agentName, func(t *testing.T) {
			for _, locale := range []string{"en-US", "zh-CN"} {
				t.Run(locale, func(t *testing.T) {
					loader := New(agentName, locale, "")
					for _, tc := range []struct {
						name string
						data interface{}
					}{
						{"learn-analyze", sampleAnalyzeRequest()},
						{"learn-batch", sampleBatchLearnData()},
						{"fix-generate", sampleGenerateFixesRequest()},
						{"pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest()},
						{"pattern-learn-current-batch", sampleAnalyzeCurrentCodebaseBatchRequest()},
						{"pattern-curate", sampleCuratePatternsData()},
						{"project-profile", sampleProjectAnalysisData()},
						{"skill-workspace-profile", workspacePromptData()},
						{"skill-workspace-spec", workspaceSpecPromptData()},
						{"workflow-optimize", sampleOptimizeWorkflowRequest()},
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

func TestLoader_DefaultLocaleRendersChinesePrompt(t *testing.T) {
	loader := New("loader", "", "")

	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())

	require.NoError(t, err)
	require.Contains(t, prompt, "你是一位专业的代码质量审查专家")
	require.NotContains(t, prompt, "You are a professional code quality review expert")
}

func TestLoader_RendersSkillsPromptsWithSkillsLocale(t *testing.T) {
	loader := NewWithLocales("loader", "zh-CN", "en-US", "")

	skillsPrompt, err := loader.Render("pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest())
	require.NoError(t, err)
	require.Contains(t, skillsPrompt, "All user-facing natural-language fields must be written in English")
	require.NotContains(t, skillsPrompt, "面向用户阅读的自然语言字段应优先使用简体中文")

	toolPrompt, err := loader.Render("fix-generate", sampleGenerateFixesRequest())
	require.NoError(t, err)
	require.Contains(t, toolPrompt, "You are a professional code fix expert")
	require.NotContains(t, toolPrompt, "你是一位专业的代码修复专家")
}

func TestLoader_RendersOutputContractGuardWithPromptLocale(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "instructions"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "instructions", "workflow-optimize.md"), []byte("USER WORKFLOW INSTRUCTIONS"), 0644))

	loader := NewWithLocales("loader", "zh-CN", "en-US", seedPath)
	prompt, err := loader.Render("workflow-optimize", sampleOptimizeWorkflowRequest())

	require.NoError(t, err)
	require.Contains(t, prompt, "# Mandatory Final Output Rules")
	require.Contains(t, prompt, "These rules have the highest priority and must be followed exactly")
	require.Contains(t, prompt, "The first non-whitespace character must be `{`")
	require.Contains(t, prompt, "All user-facing natural-language fields in the JSON must be written in English")
	require.NotContains(t, prompt, "不要使用 markdown 代码块包裹 JSON")
}

func TestLoader_RenderMergesProjectWorkspaceAndUserInstructions(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "project.md"), []byte("PROJECT CONTEXT"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "rules.md"), []byte("PROJECT RULES"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "glossary.md"), []byte("PROJECT GLOSSARY"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "workspace.md"), []byte("WORKSPACE CONTEXT"), 0644))

	loader := New("loader", "zh-CN", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	require.Contains(t, prompt, "PROJECT CONTEXT")
	require.Contains(t, prompt, "PROJECT RULES")
	require.Contains(t, prompt, "PROJECT GLOSSARY")
	require.Contains(t, prompt, "WORKSPACE CONTEXT")
	require.Less(t, strings.Index(prompt, "PROJECT CONTEXT"), strings.Index(prompt, "PROJECT RULES"))
	require.Less(t, strings.Index(prompt, "PROJECT RULES"), strings.Index(prompt, "PROJECT GLOSSARY"))
	require.Less(t, strings.Index(prompt, "PROJECT GLOSSARY"), strings.Index(prompt, "WORKSPACE CONTEXT"))
}

func TestLoader_RenderSkipsLegacyDefaultPromptScaffolds(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0755))

	projectContext := `<!-- generated-by: skills-seed v0.12.0 -->
<!-- prompt-template-sha256: old -->
<!-- context-type: project -->
<!-- editable: true -->

# 项目背景

- 项目名称: demo
- 主要语言: go
- 项目根目录: /tmp/demo

## 代码看不到的信息

在这里补充业务背景、外部系统、线上事实、灰度策略、兼容对象、人工流程、历史包袱等信息。
`
	rulesContext := `# 团队规则

在这里补充未来所有学习、检查和生成都应遵守的团队规则。
`
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "project.md"), []byte(projectContext), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "rules.md"), []byte(rulesContext), 0644))

	loader := New("loader", "zh-CN", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	require.NotContains(t, prompt, "prompt-template-sha256")
	require.NotContains(t, prompt, "在这里补充")
	require.NotContains(t, prompt, "项目名称: demo")
	require.NotContains(t, prompt, "主要语言: go")
}

func TestLoader_RenderAppendsOutputContractAfterUserInstructions(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "rules.md"), []byte("USER SAYS RETURN MARKDOWN"), 0644))

	loader := New("loader", "en-US", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)
	guardData, err := readAppendTemplateWithLocale("output-contract-guard", "en-US")
	require.NoError(t, err)
	guard := strings.TrimSpace(string(guardData))

	require.Contains(t, prompt, "USER SAYS RETURN MARKDOWN")
	require.Contains(t, prompt, guard)
	require.Less(t, strings.Index(prompt, "USER SAYS RETURN MARKDOWN"), strings.LastIndex(prompt, guard))
	require.True(t, strings.HasSuffix(prompt, guard))
}

func TestLoader_RenderStoresSuccessfulPromptUnderRuntimeMemory(t *testing.T) {
	seedPath := t.TempDir()
	loader := New("loader", "en-US", seedPath)

	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	runtimeDir := filepath.Join(seedPath, "runtime", "rendered-prompts")
	entries, err := os.ReadDir(runtimeDir)
	require.NoError(t, err)
	var renderedName string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			renderedName = entry.Name()
		}
	}
	require.NotEmpty(t, renderedName)
	require.Regexp(t, `^\d{8}-\d{6}-learn-analyze\.md$`, renderedName)

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

	loader := New("loader", "zh-CN", seedPath)
	prompt, err := loader.Render("learn-analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	runtimeDir := filepath.Join(seedPath, "runtime", "rendered-prompts")
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

func TestLoader_RenderStoresRuntimeLabelInPromptNameAndManifest(t *testing.T) {
	seedPath := t.TempDir()
	loader := New("loader", "zh-CN", seedPath)
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.RuntimeLabel = "unit-auth"
	session, err := agent.NewPromptInputSession("loader-test")
	require.NoError(t, err)
	defer session.Cleanup()
	data, err := agent.AnalyzeCurrentCodebasePromptData(session, req)
	require.NoError(t, err)

	_, err = loader.Render("pattern-learn-current", data)
	require.NoError(t, err)

	runtimeDir := filepath.Join(seedPath, "runtime", "rendered-prompts")
	entries, err := os.ReadDir(runtimeDir)
	require.NoError(t, err)
	var manifestPath string
	var promptName string
	for _, entry := range entries {
		switch {
		case strings.HasSuffix(entry.Name(), ".manifest.json"):
			manifestPath = filepath.Join(runtimeDir, entry.Name())
		case strings.HasSuffix(entry.Name(), ".md"):
			promptName = entry.Name()
		}
	}
	require.Regexp(t, `^\d{8}-\d{6}-pattern-learn-current-unit-auth\.md$`, promptName)
	require.NotEmpty(t, manifestPath)
	dataBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest map[string]interface{}
	require.NoError(t, json.Unmarshal(dataBytes, &manifest))
	require.Equal(t, "unit-auth", manifest["label"])
}

func TestLoader_RenderStoresSharedRuntimeTaskName(t *testing.T) {
	seedPath := t.TempDir()
	loader := New("loader", "zh-CN", seedPath)
	req := sampleAnalyzeCurrentCodebaseRequest()
	session, err := agent.NewPromptInputSession("loader-test")
	require.NoError(t, err)
	defer session.Cleanup()
	data, err := agent.AnalyzeCurrentCodebasePromptData(session, req)
	require.NoError(t, err)

	_, err = loader.RenderForRuntimeTask("pattern-learn-current", data, RuntimeTask{
		ID:   "20260626-183633",
		Slug: "pattern-learn-current-unit-auth",
	})
	require.NoError(t, err)

	runtimeDir := filepath.Join(seedPath, "runtime", "rendered-prompts")
	require.FileExists(t, filepath.Join(runtimeDir, "20260626-183633-pattern-learn-current-unit-auth.md"))
	manifestPath := filepath.Join(runtimeDir, "20260626-183633-pattern-learn-current-unit-auth.manifest.json")
	dataBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest map[string]interface{}
	require.NoError(t, json.Unmarshal(dataBytes, &manifest))
	require.Equal(t, "20260626-183633", manifest["runtime_id"])
	require.Equal(t, "pattern-learn-current-unit-auth", manifest["slug"])
}

func TestRenderInitSkillsListsSamplePathsWithoutEmbeddedContent(t *testing.T) {
	loader := New("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.SampleFiles = []agent.SampleFile{{
		Path: "webshell.go",
	}}

	prompt, err := loader.Render("pattern-learn-current", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "webshell.go")
	require.NotContains(t, prompt, "secretEmbeddedContent")
}

func TestRenderAnalyzeListsFilePathsWithoutEmbeddedContent(t *testing.T) {
	loader := New("codex", "zh-CN", "")
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
	loader := New("codex", "zh-CN", "")
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
	loader := New("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.DiffFiles = []agent.DiffFileRef{{
		Path:     "internal/service.go",
		DiffPath: "/tmp/skills-seed/runtime/diffs/internal/service.go.diff",
	}}

	prompt, err := loader.Render("pattern-learn-current", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "变更文件 Diff (1 个)")
	require.Contains(t, prompt, "internal/service.go")
	require.Contains(t, prompt, "/tmp/skills-seed/runtime/diffs/internal/service.go.diff")
}

func TestRenderGenerateFixesListsFilePathsWithoutEmbeddedContent(t *testing.T) {
	loader := New("codex", "zh-CN", "")
	req := sampleGenerateFixesRequest()
	req.Files = []domain.FileInfo{domain.NewFileInfo("main.go", "package main\nconst secretFixContent = true\n")}

	prompt, err := loader.Render("fix-generate", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "main.go")
	require.NotContains(t, prompt, "secretFixContent")
}

func TestRenderProjectAnalysisListsReadmePathWithoutEmbeddedContent(t *testing.T) {
	loader := New("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["ReadmePath"] = "README.md"

	prompt, err := loader.Render("project-profile", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "README.md")
	require.NotContains(t, prompt, "secret readme content")
}

func TestRenderProjectAnalysisIncludesIncrementalProfileGuidance(t *testing.T) {
	loader := New("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["ExistingProfilePath"] = "/tmp/skills-seed/existing-profile.json"
	data["FocusPaths"] = []string{"internal/service"}

	prompt, err := loader.Render("project-profile", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "已有项目画像")
	require.Contains(t, prompt, "internal/service")
	require.Contains(t, prompt, "/tmp/skills-seed/existing-profile.json")
	require.NotContains(t, prompt, "Clean Architecture")
	require.Contains(t, prompt, "完整项目画像")
}

func TestRenderProjectAnalysisBoundsStructureToFocusPaths(t *testing.T) {
	tests := []struct {
		locale string
		label  string
		bound  string
	}{
		{
			locale: "zh-CN",
			label:  "项目目录结构摘要",
			bound:  "不要因为该文件中出现路径线索而扩展到未列入范围的文件",
		},
		{
			locale: "en-US",
			label:  "Project structure summary",
			bound:  "do not expand into files outside those paths",
		},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("codex", tt.locale, "")
			data := sampleProjectAnalysisData()
			data["FocusPaths"] = []string{"internal/service"}

			prompt, err := loader.Render("project-profile", data)

			require.NoError(t, err)
			require.Contains(t, prompt, tt.label)
			require.Contains(t, prompt, tt.bound)
		})
	}
}

func TestRenderProjectAnalysisIncludesStructuralContext(t *testing.T) {
	loader := New("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["StructuralContextPath"] = "/tmp/skills-seed/structural-context.md"

	prompt, err := loader.Render("project-profile", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "结构化上下文")
	require.Contains(t, prompt, "/tmp/skills-seed/structural-context.md")
	require.NotContains(t, prompt, "handler calls service")
	require.Contains(t, prompt, "结构化")
}

func TestRenderInitSkillsBoundsStructureToFocusPaths(t *testing.T) {
	tests := []struct {
		locale string
		label  string
		bound  string
	}{
		{
			locale: "zh-CN",
			label:  "项目目录结构摘要",
			bound:  "不要因为该文件中出现路径线索而扩展到未列入范围的文件",
		},
		{
			locale: "en-US",
			label:  "Project structure summary",
			bound:  "do not expand into files outside those paths",
		},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("codex", tt.locale, "")
			req := sampleAnalyzeCurrentCodebaseRequest()
			req.FocusPaths = []string{"internal/service"}

			prompt, err := loader.Render("pattern-learn-current", req)

			require.NoError(t, err)
			require.Contains(t, prompt, tt.label)
			require.Contains(t, prompt, tt.bound)
		})
	}
}

func TestRenderInitSkillsIncludesStructuralContext(t *testing.T) {
	loader := New("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.StructuralContextPath = "/tmp/skills-seed/structural-context.md"

	prompt, err := loader.Render("pattern-learn-current", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "结构化上下文")
	require.Contains(t, prompt, "/tmp/skills-seed/structural-context.md")
	require.NotContains(t, prompt, "service has callers")
	require.Contains(t, prompt, "结构化")
}

func TestRenderCurrentLearningOmitsKnownPatterns(t *testing.T) {
	tests := []struct {
		locale string
		label  string
	}{
		{locale: "zh-CN", label: "已有模式"},
		{locale: "en-US", label: "Existing Patterns"},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("codex", tt.locale, "")
			req := sampleAnalyzeCurrentCodebaseRequest()
			req.KnownPatternsPath = "/tmp/skills-seed/known-patterns.json"
			req.KnownPatternsCount = 1

			prompt, err := loader.Render("pattern-learn-current", req)

			require.NoError(t, err)
			require.NotContains(t, prompt, tt.label)
			require.NotContains(t, prompt, "/tmp/skills-seed/known-patterns.json")
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
				"业务覆盖矩阵",
				"非请求接口",
				"资源生命周期",
				"外部依赖交互",
				"允许中英文混合表达技术概念",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"business coverage matrix",
				"resource lifecycles",
				"external dependency interactions",
				"All user-facing natural-language fields must be written in English",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")

			for _, tc := range []struct {
				name string
				data interface{}
			}{
				{"pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest()},
				{"pattern-learn-current-batch", sampleAnalyzeCurrentCodebaseBatchRequest()},
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
			},
			forbidden: []string{
				"对每个业务候选至少尝试提取 1 个 `business` 模式",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"Prefer specific categories",
			},
			forbidden: []string{
				"try to extract at least one `business` pattern",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")

			for _, tc := range []struct {
				name string
				data interface{}
			}{
				{"pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest()},
				{"pattern-learn-current-batch", sampleAnalyzeCurrentCodebaseBatchRequest()},
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

func TestLoader_RenderLearningPromptsPreserveDomainOperationBusinessSignals(t *testing.T) {
	tests := []struct {
		locale       string
		requiredText []string
	}{
		{
			locale: "zh-CN",
			requiredText: []string{
				"领域操作",
				"业务覆盖矩阵",
				"非请求接口",
				"资源生命周期",
				"外部依赖交互",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"Domain operations",
				"business coverage matrix",
				"resource lifecycles",
				"external dependency interactions",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")

			for _, tc := range []struct {
				name string
				data interface{}
			}{
				{"pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest()},
				{"pattern-learn-current-batch", sampleAnalyzeCurrentCodebaseBatchRequest()},
				{"learn-batch", sampleBatchLearnData()},
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

func TestLoader_RenderCurrentLearningPromptsIncludeModeStrategy(t *testing.T) {
	tests := []struct {
		locale       string
		name         string
		data         interface{}
		requiredText []string
	}{
		{
			locale: "zh-CN",
			name:   "analysis-plan",
			data: func() map[string]interface{} {
				data := sampleAnalysisPlanData()
				data["LearningMode"] = "fast"
				data["LearningScope"] = "domain"
				return data
			}(),
			requiredText: []string{
				"学习模式: fast",
				"切分范围: domain",
				"切分范围策略",
				"`domain`",
				"`flow`",
				"`module`",
				"模式策略",
				"完整的语义闭环",
				"如果需要频繁依赖另一个 unit 才能讲清楚，就合并",
				"`deep`",
			},
		},
		{
			locale: "en-US",
			name:   "analysis-plan",
			data: func() map[string]interface{} {
				data := sampleAnalysisPlanData()
				data["LearningMode"] = "deep"
				data["LearningScope"] = "module"
				return data
			}(),
			requiredText: []string{
				"Learning mode: deep",
				"Scope: module",
				"Scope Strategy",
				"`domain`",
				"`flow`",
				"`module`",
				"Mode Strategy",
				"complete semantic loop",
				"merge units that would need frequent cross-reading",
				"`fast`",
			},
		},
		{
			locale: "zh-CN",
			name:   "pattern-learn-current",
			data: func() *agent.AnalyzeCurrentCodebaseRequest {
				req := sampleAnalyzeCurrentCodebaseRequest()
				req.LearningMode = config.LearningModeDeep
				return req
			}(),
			requiredText: []string{
				"学习模式策略",
				"`fast`",
				"`normal`",
				"`deep`",
				"推荐分析顺序",
				"业务子域",
				"可选拆解方向",
				"通用命名目录、公共包、适配层或外部依赖封装",
				"路由价值和代码证据",
			},
		},
		{
			locale: "en-US",
			name:   "pattern-learn-current",
			data: func() *agent.AnalyzeCurrentCodebaseRequest {
				req := sampleAnalyzeCurrentCodebaseRequest()
				req.LearningMode = config.LearningModeFast
				return req
			}(),
			requiredText: []string{
				"Learning Mode Strategy",
				"`fast`",
				"`normal`",
				"`deep`",
				"Recommended analysis order",
				"business subdomains",
				"Optional expansion directions",
				"generally named directories, shared packages, adapters, or external-dependency wrappers",
				"routing value and source evidence",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale+"/"+tt.name, func(t *testing.T) {
			loader := New("loader", tt.locale, "")
			prompt, err := loader.Render(tt.name, tt.data)
			require.NoError(t, err)
			for _, text := range tt.requiredText {
				require.Contains(t, prompt, text)
			}
		})
	}
}

func TestLoader_ProjectProfilePromptKeepsDomainEntriesOutOfCommonUtils(t *testing.T) {
	tests := []struct {
		locale       string
		requiredText []string
	}{
		{
			locale: "zh-CN",
			requiredText: []string{
				"由项目源码词表确认的核心领域操作",
				"不要仅因其位于通用命名目录、公共包、适配层或外部依赖封装中就放入 `common_utils`",
				"承载产品领域行为的领域操作、协议/命令封装或外部依赖交互入口应优先放入 business_methods",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"Core domain operations, resource lifecycles, policy execution",
				"Do not place them in `common_utils` solely because they live in generally named directories, shared packages, adapters, or external-dependency wrappers",
				"external dependency interaction entries that carry product-domain behavior should prefer business_methods",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")

			prompt, err := loader.Render("project-profile", sampleProjectAnalysisData())
			require.NoError(t, err)
			for _, text := range tt.requiredText {
				require.Contains(t, prompt, text)
			}
		})
	}
}

func TestLoader_FileSelectPromptPrefersBusinessCoverage(t *testing.T) {
	tests := []struct {
		locale       string
		requiredText []string
	}{
		{
			locale: "zh-CN",
			requiredText: []string{
				"业务覆盖优先",
				"候选业务子域",
				"业务覆盖矩阵",
				"后台/异步/周期性流程",
				"产品运行期行为",
				"规则策略",
				"外部依赖约束",
				"相同候选输入",
				"稳定选择",
				"selected_paths",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"business coverage first",
				"candidate business subdomains",
				"business coverage matrix",
				"background/async/periodic flows",
				"runtime product behavior",
				"rules or policies",
				"external dependency constraints",
				"same candidate input",
				"stable selection",
				"selected_paths",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")
			prompt, err := loader.Render("file-select", map[string]interface{}{
				"CandidateNum":    3,
				"FileTree":        "internal/order/create.go\ninternal/billing/rule.go\ninternal/worker/task.go",
				"CandidatesPath":  "/tmp/skills-seed/candidates.json",
				"UserContextPath": "",
			})
			require.NoError(t, err)

			lowerPrompt := strings.ToLower(prompt)
			for _, text := range tt.requiredText {
				require.Contains(t, lowerPrompt, strings.ToLower(text))
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
			loader := New("loader", tt.locale, "")
			for _, tc := range []struct {
				name string
				data interface{}
			}{
				{"learn-batch", sampleBatchLearnData()},
				{"pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest()},
				{"pattern-learn-current-batch", sampleAnalyzeCurrentCodebaseBatchRequest()},
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

func TestLoader_RenderPatternPromptsRequireEvidenceLocations(t *testing.T) {
	tests := []struct {
		locale string
		checks map[string][]string
	}{
		{
			locale: "zh-CN",
			checks: map[string][]string{
				"pattern-learn-current": {
					"`evidence_locations`",
					"模式级源码证据位置",
					"不要编造证据路径或行号",
				},
				"pattern-learn-current-batch": {
					"`evidence_locations`",
					"模式级源码证据位置",
					"不要编造证据路径或行号",
				},
				"learn-batch": {
					"`evidence_locations`",
					"模式级源码证据位置",
					"不要编造证据路径或行号",
				},
				"pattern-curate": {
					"`evidence_locations`",
					"只能保留输入中真实存在的证据位置",
				},
				"user-define-pattern": {
					"`evidence_locations`",
					"如果有关联文件，填入真实证据位置",
				},
			},
		},
		{
			locale: "en-US",
			checks: map[string][]string{
				"pattern-learn-current": {
					"`evidence_locations`",
					"pattern-level source evidence locations",
					"Do not invent evidence paths or line numbers",
				},
				"pattern-learn-current-batch": {
					"`evidence_locations`",
					"pattern-level source evidence locations",
					"Do not invent evidence paths or line numbers",
				},
				"learn-batch": {
					"`evidence_locations`",
					"pattern-level source evidence locations",
					"Do not invent evidence paths or line numbers",
				},
				"pattern-curate": {
					"`evidence_locations`",
					"may preserve only evidence locations present in the input",
				},
				"user-define-pattern": {
					"`evidence_locations`",
					"fill real evidence locations when related files are provided",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")
			for name, requiredText := range tt.checks {
				t.Run(name, func(t *testing.T) {
					var data interface{}
					switch name {
					case "learn-batch":
						data = sampleBatchLearnData()
					case "pattern-learn-current":
						data = sampleAnalyzeCurrentCodebaseRequest()
					case "pattern-learn-current-batch":
						data = sampleAnalyzeCurrentCodebaseBatchRequest()
					case "user-define-pattern":
						data = sampleUserDefinePatternData()
					case "pattern-curate":
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
					"只有在确有项目代码依据时才声称来自源码",
					"不要编造文件路径、行号、方法签名、业务方法或源码证据",
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
					"claim source-code origin only when real project-code evidence is available",
					"Do not invent file paths, line numbers, method signatures, business methods, or source evidence",
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
			loader := New("loader", tt.locale, "")
			for name, requiredText := range tt.checks {
				t.Run(name, func(t *testing.T) {
					var data interface{}
					switch name {
					case "learn-batch":
						data = sampleBatchLearnData()
					case "pattern-learn-current":
						data = sampleAnalyzeCurrentCodebaseRequest()
					case "pattern-learn-current-batch":
						data = sampleAnalyzeCurrentCodebaseBatchRequest()
					case "pattern-curate":
						data = sampleCuratePatternsData()
					case "user-define-pattern":
						data = sampleUserDefinePatternData()
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

func TestLoader_RenderPatternPromptsUseSharedAllowedCategories(t *testing.T) {
	allowedCategories := "naming, error, structure, concurrency, testing, business, api, database, utils, middleware, config"
	tests := []struct {
		locale string
		checks map[string]string
	}{
		{
			locale: "zh-CN",
			checks: map[string]string{
				"learn-batch":                 "可用分类：" + allowedCategories,
				"pattern-learn-current":       "可用分类：" + allowedCategories,
				"pattern-learn-current-batch": "可用分类：" + allowedCategories,
				"user-define-pattern":         "可用分类：" + allowedCategories,
				"pattern-curate":              "可用分类：" + allowedCategories,
			},
		},
		{
			locale: "en-US",
			checks: map[string]string{
				"learn-batch":                 "Allowed categories: " + allowedCategories,
				"pattern-learn-current":       "Allowed categories: " + allowedCategories,
				"pattern-learn-current-batch": "Allowed categories: " + allowedCategories,
				"user-define-pattern":         "Allowed categories: " + allowedCategories,
				"pattern-curate":              "Allowed categories: " + allowedCategories,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")
			for name, requiredText := range tt.checks {
				t.Run(name, func(t *testing.T) {
					var data interface{}
					switch name {
					case "learn-batch":
						data = sampleBatchLearnData()
					case "pattern-learn-current":
						data = sampleAnalyzeCurrentCodebaseRequest()
					case "pattern-learn-current-batch":
						data = sampleAnalyzeCurrentCodebaseBatchRequest()
					case "user-define-pattern":
						data = sampleUserDefinePatternData()
					case "pattern-curate":
						data = sampleCuratePatternsData()
					}

					prompt, err := loader.Render(name, data)
					require.NoError(t, err)
					require.Contains(t, prompt, requiredText)
					require.NotContains(t, prompt, "security")
				})
			}
		})
	}
}

func TestLoader_RenderProjectInitPromptUsesConcreteCategoryInJSONExample(t *testing.T) {
	tests := []struct {
		locale    string
		forbidden []string
	}{
		{
			locale: "zh-CN",
			forbidden: []string{
				`"category": "从可用分类中选择一个：`,
			},
		},
		{
			locale: "en-US",
			forbidden: []string{
				`"category": "choose one allowed category:`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")
			prompt, err := loader.Render("pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest())
			require.NoError(t, err)
			require.Contains(t, prompt, `"category": "error"`)
			for _, text := range tt.forbidden {
				require.NotContains(t, prompt, text)
			}
		})
	}
}

func TestLoader_RenderZhProjectAnalysisRequiresChineseNaturalLanguage(t *testing.T) {
	loader := New("codex", "zh-CN", "")

	prompt, err := loader.Render("project-profile", sampleProjectAnalysisData())
	require.NoError(t, err)

	require.Contains(t, prompt, "面向用户阅读的自然语言字段应优先使用简体中文")
	require.Contains(t, prompt, "允许中英文混合表达技术概念")
	require.Contains(t, prompt, "不要从模板示例推断具体技术栈")
}

func TestLoader_RenderEnProjectAnalysisRequiresEnglishNaturalLanguage(t *testing.T) {
	loader := New("codex", "en-US", "")

	prompt, err := loader.Render("project-profile", sampleProjectAnalysisData())
	require.NoError(t, err)

	require.Contains(t, prompt, "All user-facing natural-language fields must be written in English")
	require.Contains(t, prompt, "`framework_patterns` must describe concrete framework or library usage in English")
	require.Contains(t, prompt, "Do not infer a concrete technology stack from template examples")
}

func TestLoader_RenderEnPersistentPromptsRequireEnglishNaturalLanguage(t *testing.T) {
	loader := New("loader", "en-US", "")

	for _, tc := range []struct {
		name string
		data interface{}
	}{
		{"learn-batch", sampleBatchLearnData()},
		{"pattern-curate", sampleCuratePatternsData()},
		{"pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest()},
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

func TestRenderWorkspacePromptsIncludeLearnUserContextPathWhenProvided(t *testing.T) {
	profileData := workspacePromptData()
	profileData["UserContextPath"] = "/tmp/skills-seed/user-context.md"
	specData := workspaceSpecPromptData()
	specData["UserContextPath"] = "/tmp/skills-seed/user-context.md"

	loader := New("loader", "zh-CN", "")
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
	loader := New("loader", "zh-CN", "")

	prompt, err := loader.Render("learn-batch", sampleBatchLearnData())
	require.NoError(t, err)

	require.Contains(t, prompt, "abcdef123456")
	require.NotContains(t, prompt, "diff --git a/internal/demo.go b/internal/demo.go")
	require.NotContains(t, prompt, "func Demo() error")
	require.Contains(t, prompt, "/tmp/skills-seed/known-patterns.json")
	require.NotContains(t, prompt, `"name":"Known Pattern"`)
}

func TestLoader_RuntimePromptsFenceJSONExamplesAndAppendOutputContract(t *testing.T) {
	loader := New("loader", "zh-CN", "")
	for _, tc := range []struct {
		name string
		data interface{}
	}{
		{"learn-analyze", sampleAnalyzeRequest()},
		{"learn-batch", sampleBatchLearnData()},
		{"fix-generate", sampleGenerateFixesRequest()},
		{"pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest()},
		{"pattern-learn-current-batch", sampleAnalyzeCurrentCodebaseBatchRequest()},
		{"workflow-optimize", sampleOptimizeWorkflowRequest()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompt, err := loader.Render(tc.name, tc.data)
			require.NoError(t, err)
			require.Contains(t, prompt, "```json")
			require.Contains(t, prompt, "# 最终输出硬约束")
			require.Contains(t, prompt, "最终响应必须只包含一个 JSON 对象")
			require.True(t, strings.LastIndex(prompt, "# 最终输出硬约束") > strings.LastIndex(prompt, "```json"))
		})
	}
}

func TestLoader_RuntimePromptsRequireFinalJSONSelfCheck(t *testing.T) {
	promptData := map[string]interface{}{
		"analysis-plan":               sampleAnalysisPlanData(),
		"file-select":                 map[string]interface{}{"CandidateNum": 1, "FileTree": "main.go", "CandidatesPath": "", "UserContextPath": ""},
		"learn-analyze":               sampleAnalyzeRequest(),
		"learn-batch":                 sampleBatchLearnData(),
		"fix-generate":                sampleGenerateFixesRequest(),
		"pattern-learn-current":       sampleAnalyzeCurrentCodebaseRequest(),
		"pattern-learn-current-batch": sampleAnalyzeCurrentCodebaseBatchRequest(),
		"project-profile":             sampleProjectAnalysisData(),
		"pattern-curate":              sampleCuratePatternsData(),
		"skill-workspace-profile":     workspacePromptData(),
		"skill-workspace-spec":        workspaceSpecPromptData(),
		"user-define-pattern":         sampleUserDefinePatternData(),
		"workflow-optimize":           sampleOptimizeWorkflowRequest(),
	}
	require.ElementsMatch(t, loaderRuntimePromptNames(t), sortedMapKeys(promptData))

	tests := []struct {
		locale       string
		requiredText string
	}{
		{
			locale:       "zh-CN",
			requiredText: "最终回复前必须在内部完成 JSON 自检",
		},
		{
			locale:       "en-US",
			requiredText: "Before the final response, internally validate the JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			loader := New("loader", tt.locale, "")
			for _, name := range sortedMapKeys(promptData) {
				t.Run(name, func(t *testing.T) {
					prompt, err := loader.Render(name, promptData[name])
					require.NoError(t, err)
					require.Contains(t, prompt, tt.requiredText)
				})
			}
		})
	}
}

func TestLoader_OutputContractGuardLivesInAppendTemplates(t *testing.T) {
	_, err := embedfs.FS.ReadFile(metadata.PromptTemplatePath(metadata.LoaderTemplateProvider, "output-contract-guard", "zh-CN"))
	require.Error(t, err)

	guard, err := readAppendTemplateWithLocale("output-contract-guard", "zh-CN")
	require.NoError(t, err)
	require.Contains(t, string(guard), "本节规则优先级最高，必须逐条遵守")
	require.Contains(t, string(guard), "第一个非空字符必须是 `{`")
}

func loaderRuntimePromptNames(t *testing.T) []string {
	t.Helper()

	entries, err := embedfs.FS.ReadDir(filepath.ToSlash(filepath.Join(metadata.PromptTemplatesRoot, metadata.LoaderTemplateProvider)))
	require.NoError(t, err)

	names := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, metadata.PromptTemplateExt) {
			continue
		}
		base := strings.TrimSuffix(name, metadata.PromptTemplateExt)
		base = strings.TrimSuffix(base, ".en-US")
		names[base] = struct{}{}
	}
	return sortedSetKeys(names)
}

func sortedMapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestLoader_RuntimePromptsBoundFileReadingScope(t *testing.T) {
	loader := New("loader", "zh-CN", "")
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
			name: "pattern-learn-current",
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
	loader := New("loader", "zh-CN", "")

	prompt, err := loader.Render("pattern-learn-current", sampleAnalyzeCurrentCodebaseRequest())

	require.NoError(t, err)
	require.NotContains(t, prompt, "Gin/Echo/Beego/Fiber/Spring Boot/Express/Django")
	require.NotContains(t, prompt, "GORM/Ent/XORM/SQLAlchemy/TypeORM/MyBatis")
	require.Contains(t, prompt, "只提取项目实际使用的框架")
}

func TestLoader_PromptResponsibilityContracts(t *testing.T) {
	tests := []struct {
		name       string
		data       interface{}
		requiredZH []string
		requiredEN []string
		forbid     []string
	}{
		{
			name: "project-profile",
			data: sampleProjectAnalysisData(),
			requiredZH: []string{
				"本模板只负责项目画像",
				"不要在这里学习或归纳业务模式",
				"`business_methods` 不是业务模式库",
			},
			requiredEN: []string{
				"This template is only for the project profile",
				"Do not learn or summarize business patterns here",
				"`business_methods` is not the business pattern store",
			},
			forbid: []string{
				`"patterns"`,
				`"profile_delta"`,
			},
		},
		{
			name: "pattern-learn-current",
			data: sampleAnalyzeCurrentCodebaseRequest(),
			requiredZH: []string{
				`"patterns"`,
				`"profile_delta"`,
				`"profile_refresh_recommended"`,
				"项目结构、架构说明和技术栈概览不是本模板的主要产物",
			},
			requiredEN: []string{
				`"patterns"`,
				`"profile_delta"`,
				`"profile_refresh_recommended"`,
				"Project structure, architecture narrative, and tech-stack overview are context for this template",
			},
			forbid: []string{
				`"category_summaries"`,
				`"business_rules"`,
				`"best_practices"`,
				`"common_patterns"`,
			},
		},
		{
			name: "learn-batch",
			data: sampleBatchLearnData(),
			requiredZH: []string{
				`"patterns"`,
				"只输出可执行、可复用、可验证的 patterns",
			},
			requiredEN: []string{
				`"patterns"`,
				"Only output executable, reusable, verifiable patterns",
			},
			forbid: []string{
				`"profile_delta"`,
				`"project_name"`,
				`"architecture"`,
			},
		},
	}

	for _, locale := range []string{"zh-CN", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			loader := New("loader", locale, "")
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					prompt, err := loader.Render(tc.name, tc.data)
					require.NoError(t, err)
					required := tc.requiredZH
					if locale == "en-US" {
						required = tc.requiredEN
					}
					for _, text := range required {
						require.Contains(t, prompt, text)
					}
					for _, text := range tc.forbid {
						require.NotContains(t, prompt, text)
					}
				})
			}
		})
	}
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
		"AllowedCategories":  domain.AllowedPatternCategoriesText(),
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

func sampleAnalyzeCurrentCodebaseBatchRequest() *agent.AnalyzeCurrentCodebaseBatchRequest {
	return &agent.AnalyzeCurrentCodebaseBatchRequest{
		ProjectName:   "demo",
		RootPath:      "/tmp/demo",
		Language:      "go",
		StructurePath: "/tmp/skills-seed/project-structure.txt",
		MainFiles:     []string{"cmd/demo/main.go"},
		Units: []agent.AnalyzeCurrentCodebaseBatchUnit{
			{
				AnalysisUnit: domain.AnalysisUnit{
					ID:         "auth",
					Name:       "认证登录",
					EntryPaths: []string{"internal/auth/login.go"},
				},
				FocusPaths:  []string{"internal/auth/login.go"},
				SampleFiles: []agent.SampleFile{{Path: "internal/auth/login.go"}},
			},
			{
				AnalysisUnit: domain.AnalysisUnit{
					ID:         "key",
					Name:       "密钥创建",
					EntryPaths: []string{"internal/key/create.go"},
				},
				FocusPaths:  []string{"internal/key/create.go"},
				SampleFiles: []agent.SampleFile{{Path: "internal/key/create.go"}},
			},
		},
	}
}

func sampleAnalysisPlanData() map[string]interface{} {
	return map[string]interface{}{
		"ProjectName":           "demo",
		"RootPath":              "/tmp/demo",
		"Language":              "go",
		"LearningMode":          "normal",
		"LearningScope":         "flow",
		"FocusPaths":            []string{"internal/auth/login.go", "internal/key/create.go"},
		"StructuralContextPath": "/tmp/skills-seed/structural-context.md",
		"UserContextPath":       "",
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
		"AllowedCategories":   domain.AllowedPatternCategoriesText(),
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
		"Language":          "go",
		"Files":             []string{"internal/demo.go"},
		"UserContext":       "团队希望把中文说明改写为目标语言",
		"Description":       "新增接口时复用现有 mapper",
		"Category":          "structure",
		"AllowedCategories": domain.AllowedPatternCategoriesText(),
	}
}

func sampleOptimizeWorkflowRequest() *agent.OptimizeWorkflowRequest {
	return &agent.OptimizeWorkflowRequest{
		ID:       "deploy",
		Name:     "deploy",
		Context:  "发布前检查环境变量和构建产物，发布后执行 smoke test",
		Language: "go",
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
