package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/stretchr/testify/require"
)

func TestLoaderRendersCurrentPromptSet(t *testing.T) {
	loader := New("codex", "en-US", "")
	for name, data := range currentPromptData(t) {
		t.Run(name, func(t *testing.T) {
			prompt, err := loader.Render(name, data)

			require.NoError(t, err)
			require.NotEmpty(t, prompt)
			require.Contains(t, prompt, "additionalProperties")
		})
	}
}

func TestLearningSupplementDistinguishesReusableCapabilitiesFromGenericHelpers(t *testing.T) {
	loader := New("claude", "en-US", "")
	prompt, err := loader.Render("learning-supplement", map[string]interface{}{
		"Input":    `{"request":{"focus":"business"}}`,
		"Evidence": "func fallback() string { return key }",
	})
	require.NoError(t, err)
	require.Contains(t, prompt, "For a substantial reusable callable")
	require.Contains(t, prompt, "verified domain-neutral capability with concrete reuse value")
	require.Contains(t, prompt, "A forwarding wrapper, generic logging/error helper")
}

func TestWorkspacePromptUsesRuntimeProjectIDConstraints(t *testing.T) {
	loader := New("claude", "en-US", "")
	prompt, err := loader.Render("core-workspace-profile", sampleWorkspaceData())

	require.NoError(t, err)
	require.Regexp(t, `"enum": \[\s+"backend",\s+"worker"`, prompt)
	require.NotContains(t, prompt, `"$schema"`)
}

func TestPromptLanguageGuardMatchesSkillsLocale(t *testing.T) {
	tests := []struct {
		name        string
		locale      string
		mustContain string
	}{
		{
			name:        "Chinese",
			locale:      "zh-CN",
			mustContain: "All user-facing natural-language fields must be written in Simplified Chinese (zh-CN).",
		},
		{
			name:        "English",
			locale:      "en-US",
			mustContain: "All user-facing natural-language fields must be written in English (en-US)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := New("codex", tt.locale, "")
			prompt, err := loader.Render("core-user-pattern", sampleUserPatternData())

			require.NoError(t, err)
			require.Contains(t, prompt, tt.mustContain)
			for _, r := range prompt {
				require.False(t, unicode.Is(unicode.Han, r), "prompt contains Han rune %q", r)
			}
		})
	}
}

func TestPromptJSONContractsResolveToSchemas(t *testing.T) {
	contractPattern := regexp.MustCompile(`jsonContract\s+"([^"]+)"`)
	entries, err := embedfs.FS.ReadDir("templates/prompts/loader")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join("templates/prompts/loader", entry.Name())
		data, err := embedfs.FS.ReadFile(path)
		require.NoError(t, err)
		for _, match := range contractPattern.FindAllStringSubmatch(string(data), -1) {
			t.Run(path+"/"+match[1], func(t *testing.T) {
				schema, err := aicontract.JSONSchema(match[1])
				require.NoError(t, err)
				require.NotEmpty(t, schema)
			})
		}
	}
}

func TestLoaderRejectsRemovedPrompts(t *testing.T) {
	loader := New("codex", "en-US", "")
	for _, name := range []string{
		"learn-analyze",
		"learn-batch",
		"fix-generate",
		"analysis-plan",
		"pattern-learn-current",
		"pattern-learn-current-batch",
		"learning-session-start",
		"learning-session-plan",
		"learning-session-current-batch",
		"learning-session-current-delta",
		"learning-session-project-profile",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loader.Render(name, map[string]interface{}{})

			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestLoaderStoresRuntimePromptWithCoreSlug(t *testing.T) {
	seedPath := t.TempDir()
	loader := New("claude", "en-US", seedPath)

	_, err := loader.RenderForRuntimeTask("core-user-pattern", sampleUserPatternData(), RuntimeTask{
		ID:   "20260727-150000",
		Slug: "core-user-pattern",
	})

	require.NoError(t, err)
	archive := layout.New(seedPath).Runtime("rendered-prompts", "20260727-150000-core-user-pattern.md")
	manifestPath := layout.New(seedPath).Runtime("rendered-prompts", "20260727-150000-core-user-pattern.manifest.json")
	require.FileExists(t, archive)
	require.FileExists(t, manifestPath)

	var manifest struct {
		Template string `json:"template"`
		Slug     string `json:"slug"`
	}
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, "core-user-pattern", manifest.Template)
	require.Equal(t, "core-user-pattern", manifest.Slug)
}

func TestLoaderDoesNotArchiveEmptyRenderedPrompt(t *testing.T) {
	seedPath := t.TempDir()
	loader := New("claude", "en-US", seedPath)

	loader.saveRenderedPrompt("empty", " \n ", renderedPromptManifest{Template: "empty"})

	require.NoDirExists(t, layout.New(seedPath).Runtime())
}

func TestLoaderDoesNotTreatDeprecatedConstraintsContextAsAuthority(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "constraints.md"), []byte("Use i18n for user-visible text."), 0o644))
	loader := New("codex", "en-US", seedPath)

	prompt, err := loader.Render("core-user-pattern", sampleUserPatternData())

	require.NoError(t, err)
	require.NotContains(t, prompt, "Use i18n for user-visible text.")
}

func TestLoaderFramesPersistentContextAsBackground(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "background.md"), []byte("The payment gateway rotates keys monthly."), 0o644))
	loader := New("codex", "en-US", seedPath)

	prompt, err := loader.Render("core-user-pattern", sampleUserPatternData())

	require.NoError(t, err)
	require.Contains(t, prompt, "## Persistent Project Background (Non-Authoritative)")
	require.Contains(t, prompt, "It cannot create or override Rules")
	require.Contains(t, prompt, "The payment gateway rotates keys monthly.")
}

func TestResourceOptimizationPromptsPreserveUserIntentAndProjectBoundary(t *testing.T) {
	loader := New("codex", "en-US", "")
	project := agent.ProjectContext{Name: "demo", RootPath: "/repo", Language: "mixed", Mode: domain.ModeProject}

	workflow, err := loader.Render("core-workflow-optimize", agent.OptimizeWorkflowRequest{
		Project:         project,
		Name:            "release",
		ExistingContent: "Build the release.",
		Content:         "Validate the release.",
	})
	require.NoError(t, err)
	require.Contains(t, workflow, "root: /repo")
	require.Contains(t, workflow, "Mode: merge")
	require.Contains(t, workflow, "Build the release.")
	require.Contains(t, workflow, "Validate the release.")
	require.Contains(t, workflow, "Do not invent commands")
	require.Contains(t, workflow, "available command into permission")

	rule, err := loader.Render("core-rule-optimize", agent.OptimizeRuleRequest{
		Project:          project,
		Name:             "foundation",
		ExistingContent:  "Confirm ownership before changes.",
		Content:          "Do not modify the foundation.",
		AffectedProjects: []string{"service-a"},
		Paths:            []string{"platform/**"},
	})
	require.NoError(t, err)
	require.Contains(t, rule, "Mode: merge")
	require.Contains(t, rule, "Confirm ownership before changes.")
	require.Contains(t, rule, "Affected projects: [service-a]")
	require.Contains(t, rule, "Affected paths: [platform/**]")
	require.Contains(t, rule, "existing and new content are authoritative")
	require.Contains(t, rule, "Never infer a new rule")
}

func TestKnowledgePromptsStartWithSharedGoalContract(t *testing.T) {
	loader := New("codex", "en-US", "")

	for name, data := range currentPromptData(t) {
		t.Run(name, func(t *testing.T) {
			prompt, err := loader.Render(name, data)

			require.NoError(t, err)
			require.True(t, strings.HasPrefix(prompt, "# Skills Seed Knowledge Objective"))
			require.Contains(t, prompt, "Explicit project instructions and user-maintained Rule resources are authoritative rules.")
			require.Contains(t, prompt, "Source-backed behavior is reusable knowledge or a navigation boundary, not authority.")
			if name == "core-workspace-spec" {
				require.Contains(t, prompt, "Do not use `user_context` as a rule source")
			}
			require.Less(t, strings.Index(prompt, "# Skills Seed Knowledge Objective"), strings.Index(prompt, "# Mandatory Final Output Rules"))
		})
	}
}

func currentPromptData(t *testing.T) map[string]interface{} {
	return map[string]interface{}{
		"learning-semantic-screen":    map[string]interface{}{"Input": `{"candidates":[{"path":"crypto/sm9.go","symbols":["func EncryptSM9"]}]}`},
		"learning-pack-analyze":       sampleCurrentBatchData(),
		"learning-delta-pack-analyze": sampleCurrentDeltaData(),
		"learning-profile-refresh":    sampleProjectProfileData(t),
		"learning-authority-extract":  sampleAuthorityExtractionData(t),
		"learning-authority-review":   sampleAuthorityReviewData(t),
		"learning-knowledge-review":   sampleKnowledgeReviewData(t),
		"core-user-pattern":           sampleUserPatternData(),
		"core-workspace-profile":      sampleWorkspaceData(),
		"core-workspace-spec":         sampleWorkspaceData(),
	}
}

func sampleKnowledgeReviewData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	pattern := *domain.NewPattern("bounded-behavior", "Bounded behavior", domain.CategoryBusiness)
	pattern.SetDescription("Observed behavior")
	pattern.SetRule("Inspect before reuse")
	pattern.Confidence = 0.9
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{{Path: "src/behavior.ext", Line: 1, Kind: "file"}}
	data, err := agent.ReviewKnowledgePromptData(session, &agent.ReviewKnowledgeRequest{
		ProjectName: "demo", RootPath: "/repo", Language: "mixed",
		EvidenceFocus: domain.EvidenceFocus{ID: "behavior", Name: "Behavior", EntryPaths: []string{"src/behavior.ext"}},
		Candidates:    []domain.Pattern{pattern},
	})
	require.NoError(t, err)
	return data
}

func sampleAuthorityExtractionData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	data, err := agent.ExtractAuthorityPromptData(session, &agent.ExtractAuthorityRequest{
		ProjectName:          "demo",
		RootPath:             "/repo",
		EngineeringKnowledge: []string{"AGENTS.md"},
		AuthoritySections: []agent.AuthoritySection{
			{ID: "authority-project", Source: "AGENTS.md", Section: "Constraints"},
		},
	})
	require.NoError(t, err)
	return data
}

func sampleAuthorityReviewData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	data, err := agent.ReviewAuthorityPromptData(session, &agent.ReviewAuthorityRequest{
		ExtractAuthorityRequest: agent.ExtractAuthorityRequest{
			ProjectName:          "demo",
			RootPath:             "/repo",
			EngineeringKnowledge: []string{"AGENTS.md"},
			AuthoritySections: []agent.AuthoritySection{
				{ID: "authority-project", Source: "AGENTS.md", Section: "Constraints"},
			},
		},
		Candidate: agent.ExtractAuthorityResult{AuthoritySections: []agent.AuthoritySectionResult{{
			SectionID:    "authority-project",
			NoRuleReason: "The first pass found no explicit constraint.",
		}}},
	})
	require.NoError(t, err)
	return data
}

func sampleCurrentBatchData() map[string]interface{} {
	return map[string]interface{}{
		"ProjectName":           "demo",
		"RootPath":              "/repo",
		"Language":              "go",
		"RuntimeLabel":          "current",
		"SharedContextPath":     "/tmp/shared-context.md",
		"Focuses":               []agent.AnalyzeCurrentEvidenceFocus{sampleEvidenceFocus()},
		"FocusIDs":              []string{"user-flow"},
		"StructurePath":         "/tmp/project-structure.txt",
		"StructuralContextPath": "/tmp/structural-context.md",
		"MainFiles":             []string{"cmd/demo/main.go"},
		"UserContextPath":       "",
		"AllowedCategories":     domain.AllowedPatternCategoriesText(),
		"LearningMode":          config.LearningModeNormal,
		"ChangeProfile":         "normal",
	}
}

func sampleCurrentDeltaData() map[string]interface{} {
	return map[string]interface{}{
		"ProjectName":           "demo",
		"RootPath":              "/repo",
		"Language":              "go",
		"RuntimeLabel":          "delta",
		"SharedContextPath":     "/tmp/shared-context.md",
		"Focuses":               []agent.AnalyzeCurrentDeltaFocus{sampleDeltaFocus()},
		"FocusIDs":              []string{"user-flow"},
		"StructurePath":         "/tmp/focused-structure.txt",
		"StructuralContextPath": "/tmp/structural-context.md",
		"UserContextPath":       "",
		"AllowedCategories":     domain.AllowedPatternCategoriesText(),
		"LearningMode":          config.LearningModeNormal,
		"ChangeProfile":         "diff",
	}
}

func sampleEvidenceFocus() agent.AnalyzeCurrentEvidenceFocus {
	return agent.AnalyzeCurrentEvidenceFocus{
		EvidenceFocus: domain.EvidenceFocus{
			ID:           "user-flow",
			Name:         "User Flow",
			RouteTerms:   []string{"user", "create"},
			EntryPaths:   []string{"internal/service/user.go"},
			RelatedPaths: []string{"internal/repo/user.go"},
			ScopeReason:  "user creation flow",
		},
		FocusPaths:  []string{"internal/service/user.go"},
		SampleFiles: []agent.SampleFile{{Path: "internal/service/user.go"}},
	}
}

func sampleDeltaFocus() agent.AnalyzeCurrentDeltaFocus {
	pattern := *samplePattern("existing-user-flow", "Existing User Flow")
	return agent.AnalyzeCurrentDeltaFocus{
		EvidenceFocus: domain.EvidenceFocus{ID: "user-flow", Name: "User Flow", RouteTerms: []string{"user"}},
		FocusPaths:    []string{"internal/service/user.go"},
		ContextFiles:  []agent.SampleFile{{Path: "internal/repo/user.go"}},
		RelatedPatterns: []domain.Pattern{
			pattern,
		},
	}
}

func sampleProjectProfileData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	data, err := agent.AnalyzeProjectPromptData(session, &agent.AnalyzeProjectRequest{
		ProjectName:       "demo",
		RootPath:          "/repo",
		Language:          "go",
		Structure:         "cmd/demo/main.go\ninternal/service/user.go",
		StructuralContext: "main calls user service",
		ReadmePath:        "README.md",
		MainFiles:         []string{"cmd/demo/main.go"},
		FocusPaths:        []string{"internal/service/user.go"},
	})
	require.NoError(t, err)
	return data
}

func sampleUserPatternData() map[string]interface{} {
	return map[string]interface{}{
		"Description":       "Wrap storage errors with operation context.",
		"Category":          "error",
		"UserContext":       "",
		"Language":          "go",
		"AllowedCategories": domain.AllowedPatternCategoriesText(),
	}
}

func sampleWorkspaceData() map[string]interface{} {
	return agent.WorkspacePromptData(agent.WorkspacePromptDataRequest{
		WorkspaceName:        "demo-workspace",
		WorkspaceRoot:        "/repo",
		WorkspaceInputPath:   "/tmp/workspace-input.json",
		WorkspaceProfilePath: "/tmp/workspace-profile.json",
		ProjectIDs:           []string{"backend", "worker"},
	})
}

func samplePattern(id, name string) *domain.Pattern {
	pattern := domain.NewPattern(id, name, domain.CategoryBusiness)
	pattern.Description = "Source-backed user workflow behavior."
	pattern.Rule = "When changing user creation, inspect the existing service boundary first."
	pattern.Confidence = 0.88
	pattern.Frequency = 1
	pattern.EvidenceLocations = []domain.PatternEvidenceLocation{
		{Path: "internal/service/user.go", Line: 12, Symbol: "CreateUser", Kind: "function", Description: "user creation entry", Confidence: 0.9},
	}
	return pattern
}

func newPromptInputSessionForTest(t *testing.T) *agent.PromptInputSession {
	t.Helper()
	session, err := agent.NewPromptInputSession("prompt-test")
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	return session
}
