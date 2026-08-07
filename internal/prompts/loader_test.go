package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

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

func TestLoaderMergesContextFragments(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "constraints.md"), []byte("Use i18n for user-visible text."), 0o644))
	loader := New("codex", "en-US", seedPath)

	prompt, err := loader.Render("core-user-pattern", sampleUserPatternData())

	require.NoError(t, err)
	require.Contains(t, prompt, "Use i18n for user-visible text.")
}

func TestLearningPromptsUseRuntimeBoundaries(t *testing.T) {
	loader := New("codex", "en-US", "")

	plan, err := loader.Render("learning-pack-plan", samplePlanData(t))
	require.NoError(t, err)
	require.Contains(t, plan, "planning runtime call")
	require.Contains(t, plan, "self-contained evidence packs")
	require.Contains(t, plan, "Do not plan a directory inventory")
	require.Contains(t, plan, "Preserve these knowledge lanes")
	require.Contains(t, plan, "decision-bearing cues")
	require.Contains(t, plan, "verified callable or declarative entries")
	require.Contains(t, plan, "Learning strategy guidance")
	require.Contains(t, plan, "Learning scope guidance")

	selectPrompt, err := loader.Render("learning-candidate-select", sampleCandidateSelectionData(t))
	require.NoError(t, err)
	require.Contains(t, selectPrompt, "Select the repository-relative files")
	require.Contains(t, selectPrompt, "required paths must appear in `selected_paths`")
	require.Contains(t, selectPrompt, "must come from the exact candidate file list")
	require.Contains(t, selectPrompt, "must not duplicate selected_paths")
	require.Contains(t, selectPrompt, "schema field descriptions are part of the output contract")

	batch, err := loader.Render("learning-pack-analyze", sampleCurrentBatchData())
	require.NoError(t, err)
	require.Contains(t, batch, "isolated pack-analysis runtime call")
	require.Contains(t, batch, "Run a decision-value discovery pass")
	require.Contains(t, batch, "Pack-local refinement")
	require.Contains(t, batch, "product/domain patterns and code patterns")
	require.Contains(t, batch, "Fill `business_method` only when")
	require.Contains(t, batch, "verified reusable capability entry")
	require.Contains(t, batch, "The mode changes recall/precision priority only")
	require.NotContains(t, batch, "pattern-evidence-rules")

	delta, err := loader.Render("learning-delta-pack-analyze", sampleCurrentDeltaData())
	require.NoError(t, err)
	require.Contains(t, delta, "isolated diff-pack analysis runtime call")
	require.Contains(t, delta, "Changed hunks are the source anchor")
	require.Contains(t, delta, "proposal.business_method")
	require.Contains(t, delta, "Do not infer a new language, framework, architecture, or domain inventory")
	require.Contains(t, delta, "permission to learn outside the diff boundary")

	profile, err := loader.Render("learning-profile-refresh", sampleProjectProfileData(t))
	require.NoError(t, err)
	require.Contains(t, profile, "bounded profile-sync runtime call")
	require.Contains(t, profile, "Do not inventory the project for completeness")
	require.Contains(t, profile, "Avoid exact capability counts")

}

func TestWorkflowPromptAllowsTaskSpecificStructure(t *testing.T) {
	loader := New("codex", "en-US", "")

	prompt, err := loader.Render("core-workflow-optimize", sampleWorkflowData())
	require.NoError(t, err)

	require.Contains(t, prompt, "Use a structure that fits the workflow")
	require.Contains(t, prompt, "Do not force validation or rollback sections")
	require.Contains(t, prompt, "Treat structural differences such as section names, optional sections, and flexible document content formats as mergeable")
	require.NotContains(t, prompt, "Use only these sections")
}

func currentPromptData(t *testing.T) map[string]interface{} {
	return map[string]interface{}{
		"learning-candidate-select":   sampleCandidateSelectionData(t),
		"learning-pack-plan":          samplePlanData(t),
		"learning-pack-analyze":       sampleCurrentBatchData(),
		"learning-delta-pack-analyze": sampleCurrentDeltaData(),
		"learning-profile-refresh":    sampleProjectProfileData(t),
		"core-user-pattern":           sampleUserPatternData(),
		"core-workflow-optimize":      sampleWorkflowData(),
		"core-workspace-profile":      sampleWorkspaceData(),
		"core-workspace-spec":         sampleWorkspaceData(),
	}
}

func sampleCandidateSelectionData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	data, err := agent.SelectLearningCandidatesPromptData(session, &agent.SelectLearningCandidatesRequest{
		ProjectName:       "demo",
		RootPath:          "/repo",
		Language:          "go",
		CandidatePaths:    []string{"internal/service/user.go", "internal/model/user.go"},
		RequiredPaths:     []string{"internal/service/user.go"},
		StructuralContext: "UserService -> UserRepo",
		LearningMode:      config.LearningModeNormal,
		LearningScope:     config.LearningScopeFlow,
	})
	require.NoError(t, err)
	return data
}

func samplePlanData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	data, err := agent.PlanLearningAgendaPromptData(session, &agent.PlanLearningAgendaRequest{
		ProjectName:       "demo",
		RootPath:          "/repo",
		Language:          "go",
		FocusPaths:        []string{"internal/service/user.go"},
		StructuralContext: "UserService -> UserRepo",
		LearningMode:      config.LearningModeNormal,
		LearningScope:     config.LearningScopeFlow,
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
		DiffFiles:   []agent.DiffFileRef{{Path: "internal/service/user.go", DiffPath: ".skills-seed/runtime/diffs/user.diff"}},
	}
}

func sampleDeltaFocus() agent.AnalyzeCurrentDeltaFocus {
	pattern := *samplePattern("existing-user-flow", "Existing User Flow")
	return agent.AnalyzeCurrentDeltaFocus{
		EvidenceFocus: domain.EvidenceFocus{ID: "user-flow", Name: "User Flow", RouteTerms: []string{"user"}},
		FocusPaths:    []string{"internal/service/user.go"},
		DiffFiles:     []agent.DiffFileRef{{Path: "internal/service/user.go", DiffPath: ".skills-seed/runtime/diffs/user.diff"}},
		ContextFiles:  []agent.SampleFile{{Path: "internal/repo/user.go"}},
		RelatedPatterns: []domain.Pattern{
			pattern,
		},
	}
}

func sampleProjectProfileData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	data, err := agent.AnalyzeProjectPromptData(session, &agent.AnalyzeProjectRequest{
		ProjectName:          "demo",
		RootPath:             "/repo",
		Language:             "go",
		Structure:            "cmd/demo/main.go\ninternal/service/user.go",
		StructuralContext:    "main calls user service",
		ReadmePath:           "README.md",
		MainFiles:            []string{"cmd/demo/main.go"},
		EngineeringKnowledge: []string{"AGENTS.md"},
		FocusPaths:           []string{"internal/service/user.go"},
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

func sampleWorkflowData() agent.OptimizeWorkflowRequest {
	return agent.OptimizeWorkflowRequest{
		ID:       "release",
		Name:     "release",
		Language: "go",
		Context:  "Run go test ./... before release.",
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
