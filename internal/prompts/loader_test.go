package prompts

import (
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
						{"analyze", sampleAnalyzeRequest()},
						{"batch-learn", sampleBatchLearnData()},
						{"generate_fixes", sampleGenerateFixesRequest()},
						{"generate_skills_summary", sampleGenerateSkillsData()},
						{"init-skills", sampleAnalyzeCurrentCodebaseRequest()},
						{"merge-patterns", sampleMergePatternsData()},
						{"project-analysis", sampleProjectAnalysisData()},
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

	_, err := loader.Render("generate_skills_summary", map[string]interface{}{
		"LANGUAGE":       "go",
		"PATTERNS_JSON":  "[]",
		"PATTERNS_COUNT": 0,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "PROJECT_NAME")
}

func TestLoader_RenderAddsCommonProjectPromptAndLegacyPromptSpecificOverlay(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "project"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "prompts", "custom"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "project-profile.md"), []byte("PROJECT PROFILE"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "common.md"), []byte("COMMON PROJECT RULES"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "project", "analyze.project.md"), []byte("LEGACY ANALYZE RULES"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "prompts", "custom", "analyze.override.md"), []byte("CUSTOM ANALYZE OVERRIDE"), 0644))

	loader := NewLoader("common", "zh-CN", seedPath)
	prompt, err := loader.Render("analyze", sampleAnalyzeRequest())
	require.NoError(t, err)

	require.Contains(t, prompt, "PROJECT PROFILE")
	require.Contains(t, prompt, "COMMON PROJECT RULES")
	require.Contains(t, prompt, "LEGACY ANALYZE RULES")
	require.Contains(t, prompt, "CUSTOM ANALYZE OVERRIDE")
	require.Less(t, strings.Index(prompt, "COMMON PROJECT RULES"), strings.Index(prompt, "LEGACY ANALYZE RULES"))
	require.Less(t, strings.Index(prompt, "LEGACY ANALYZE RULES"), strings.Index(prompt, "CUSTOM ANALYZE OVERRIDE"))
}

func TestRenderInitSkillsListsSamplePathsWithoutEmbeddedContent(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.SampleFiles = []agent.SampleFile{{
		Path: "webshell.go",
	}}

	prompt, err := loader.Render("init-skills", req)

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

	prompt, err := loader.Render("analyze", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "main.go")
	require.NotContains(t, prompt, "secretAnalyzeContent")
	require.NotContains(t, prompt, "secretGoodExample")
	require.NotContains(t, prompt, "secretBadExample")
}

func TestRenderGenerateFixesListsFilePathsWithoutEmbeddedContent(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleGenerateFixesRequest()
	req.Files = []domain.FileInfo{domain.NewFileInfo("main.go", "package main\nconst secretFixContent = true\n")}

	prompt, err := loader.Render("generate_fixes", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "main.go")
	require.NotContains(t, prompt, "secretFixContent")
}

func TestRenderProjectAnalysisListsReadmePathWithoutEmbeddedContent(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["ReadmePath"] = "README.md"

	prompt, err := loader.Render("project-analysis", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "README.md")
	require.NotContains(t, prompt, "secret readme content")
}

func TestRenderProjectAnalysisIncludesIncrementalProfileGuidance(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["ExistingProfileJSON"] = `{"architecture":"Clean Architecture"}`
	data["FocusPaths"] = []string{"internal/service"}

	prompt, err := loader.Render("project-analysis", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "已有项目画像")
	require.Contains(t, prompt, "internal/service")
	require.Contains(t, prompt, "Clean Architecture")
	require.Contains(t, prompt, "完整项目画像")
}

func TestRenderProjectAnalysisIncludesStructuralContext(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	data := sampleProjectAnalysisData()
	data["StructuralContext"] = "## CodeGraph Structural Context\n- handler calls service"

	prompt, err := loader.Render("project-analysis", data)

	require.NoError(t, err)
	require.Contains(t, prompt, "CodeGraph")
	require.Contains(t, prompt, "handler calls service")
	require.Contains(t, prompt, "结构化")
}

func TestRenderInitSkillsIncludesStructuralContext(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")
	req := sampleAnalyzeCurrentCodebaseRequest()
	req.StructuralContext = "## CodeGraph Structural Context\n- service has callers"

	prompt, err := loader.Render("init-skills", req)

	require.NoError(t, err)
	require.Contains(t, prompt, "CodeGraph")
	require.Contains(t, prompt, "service has callers")
	require.Contains(t, prompt, "结构化")
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
				"所有面向用户阅读的自然语言字段必须使用简体中文",
			},
		},
		{
			locale: "en-US",
			requiredText: []string{
				"business candidate inventory",
				"business pattern expansion",
				"Do not compress multiple business details into one generic pattern",
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
				{"init-skills", sampleAnalyzeCurrentCodebaseRequest()},
				{"batch-learn", sampleBatchLearnData()},
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
				{"init-skills", sampleAnalyzeCurrentCodebaseRequest()},
				{"batch-learn", sampleBatchLearnData()},
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

func TestLoader_RenderZhProjectAnalysisRequiresChineseNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")

	prompt, err := loader.Render("project-analysis", sampleProjectAnalysisData())
	require.NoError(t, err)

	require.Contains(t, prompt, "所有面向用户阅读的自然语言字段必须使用简体中文")
	require.Contains(t, prompt, "`framework_patterns` 必须用中文描述框架使用方式")
	require.Contains(t, prompt, "不要输出 “Cobra command pattern”")
}

func TestLoader_RenderZhGenerateSkillsSummaryRequiresChineseNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")

	prompt, err := loader.Render("generate_skills_summary", sampleGenerateSkillsData())
	require.NoError(t, err)

	require.Contains(t, prompt, "所有面向用户阅读的自然语言字段必须使用简体中文")
	require.Contains(t, prompt, "如果输入模式里包含英文说明，请改写成中文")
	require.Contains(t, prompt, "不要输出 “Repository pattern”")
}

func TestLoader_RenderBatchLearnUsesCommitHashesWithoutDiffs(t *testing.T) {
	loader := NewLoader("common", "zh-CN", "")

	prompt, err := loader.Render("batch-learn", sampleBatchLearnData())
	require.NoError(t, err)

	require.Contains(t, prompt, "abcdef123456")
	require.NotContains(t, prompt, "diff --git a/internal/demo.go b/internal/demo.go")
	require.NotContains(t, prompt, "func Demo() error")
	require.Contains(t, prompt, `"name":"Known Pattern"`)
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
		"KnownPatternsJSON":  `[{"id":"known","name":"Known Pattern","category":"api"}]`,
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
		"PATTERNS_JSON":        `[{"id":"p1","name":"Demo","category":"api"}]`,
		"PATTERNS_COUNT":       1,
		"EXISTING_SKILLS_PATH": "",
	}
}

func sampleAnalyzeCurrentCodebaseRequest() *agent.AnalyzeCurrentCodebaseRequest {
	return &agent.AnalyzeCurrentCodebaseRequest{
		ProjectName: "demo",
		RootPath:    "/tmp/demo",
		Language:    "go",
		Structure:   "cmd/demo/main.go",
		MainFiles:   []string{"cmd/demo/main.go"},
		SampleFiles: []agent.SampleFile{{Path: "cmd/demo/main.go"}},
	}
}

func sampleMergePatternsData() map[string]interface{} {
	return map[string]interface{}{
		"Category": "api",
		"Patterns": []domain.Pattern{*samplePattern()},
	}
}

func sampleProjectAnalysisData() map[string]interface{} {
	return map[string]interface{}{
		"ProjectName":         "demo",
		"RootPath":            "/tmp/demo",
		"Language":            "go",
		"Structure":           "cmd/demo/main.go",
		"StructuralContext":   "",
		"ReadmePath":          "README.md",
		"MainFiles":           []string{"cmd/demo/main.go"},
		"ExistingProfileJSON": "",
		"FocusPaths":          []string{},
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
