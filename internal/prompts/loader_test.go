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
						{"learn-analyze", sampleAnalyzeRequest()},
						{"learn-batch", sampleBatchLearnData()},
						{"fix-generate", sampleGenerateFixesRequest()},
						{"skill-project-summary", sampleGenerateSkillsData()},
						{"skill-project-init", sampleAnalyzeCurrentCodebaseRequest()},
						{"pattern-merge", sampleMergePatternsData()},
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
		"USER_CONTEXT_PATH":    "",
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
	require.Contains(t, prompt, "CodeGraph")
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
	require.Contains(t, prompt, "CodeGraph")
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

func TestLoader_RenderZhProjectAnalysisRequiresChineseNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")

	prompt, err := loader.Render("project-analyze", sampleProjectAnalysisData())
	require.NoError(t, err)

	require.Contains(t, prompt, "所有面向用户阅读的自然语言字段必须使用简体中文")
	require.Contains(t, prompt, "`framework_patterns` 必须用中文描述框架使用方式")
	require.Contains(t, prompt, "不要输出 “Cobra command pattern”")
}

func TestLoader_RenderZhGenerateSkillsSummaryRequiresChineseNaturalLanguage(t *testing.T) {
	loader := NewLoader("codex", "zh-CN", "")

	prompt, err := loader.Render("skill-project-summary", sampleGenerateSkillsData())
	require.NoError(t, err)

	require.Contains(t, prompt, "所有面向用户阅读的自然语言字段必须使用简体中文")
	require.Contains(t, prompt, "如果输入模式里包含英文说明，请改写成中文")
	require.Contains(t, prompt, "不要输出 “Repository pattern”")
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

func TestLoader_UpdatedRuntimePromptsUseUnfencedJSONExamples(t *testing.T) {
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
			require.NotContains(t, prompt, "```json")
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
				"优先读取 CodeGraph 结构化上下文",
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
		"USER_CONTEXT_PATH":    "",
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

func sampleMergePatternsData() map[string]interface{} {
	return map[string]interface{}{
		"Category": "api",
		"Patterns": []domain.Pattern{*samplePattern()},
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
