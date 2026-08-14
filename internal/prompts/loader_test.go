package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func TestLoaderDoesNotTreatDeprecatedConstraintsContextAsAuthority(t *testing.T) {
	seedPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(seedPath, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "context", "constraints.md"), []byte("Use i18n for user-visible text."), 0o644))
	loader := New("codex", "en-US", seedPath)

	prompt, err := loader.Render("core-user-pattern", sampleUserPatternData())

	require.NoError(t, err)
	require.NotContains(t, prompt, "Use i18n for user-visible text.")
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
			require.Less(t, strings.Index(prompt, "# Skills Seed Knowledge Objective"), strings.Index(prompt, "# Mandatory Final Output Rules"))
		})
	}
}

func TestLearningPromptsUseRuntimeBoundaries(t *testing.T) {
	loader := New("codex", "en-US", "")

	plan, err := loader.Render("learning-pack-plan", samplePlanData(t))
	require.NoError(t, err)
	require.Contains(t, plan, "planning runtime call")
	require.Contains(t, plan, "self-contained evidence packs")
	require.Contains(t, plan, "Do not plan a directory inventory")
	require.Contains(t, plan, "Preserve these knowledge lanes")
	require.Contains(t, plan, "Product/domain specificity is not the admission test")
	require.Contains(t, plan, "overload, cancellation, retry, partial-result")
	require.Contains(t, plan, "Never group unrelated sibling services")
	require.Contains(t, plan, "No skipped-path reason dismisses a source as generic")
	require.Contains(t, plan, "Deterministic per-file source facts")
	require.Contains(t, plan, "Never label a file empty, placeholder-only")
	require.Contains(t, plan, "requires direct file evidence")
	require.Contains(t, plan, "decision-bearing cues")
	require.Contains(t, plan, "Stateless or pure calculation does not mean decision-free")
	require.Contains(t, plan, "absence of external effects as proof")
	require.Contains(t, plan, "verified callable or declarative entries")
	require.Contains(t, plan, "Learning strategy guidance")
	require.Contains(t, plan, "minimum source-evidence set")
	require.Contains(t, plan, "not a permanent taxonomy, Workflow, work procedure, or development instruction")
	require.Contains(t, plan, "Do not create focuses for test sources")
	require.Contains(t, plan, "testing, deployment, release, and acceptance responsibilities")
	require.Contains(t, plan, "Those are user-maintained Workflow concerns")

	batch, err := loader.Render("learning-pack-analyze", sampleCurrentBatchData())
	require.NoError(t, err)
	require.Contains(t, batch, "isolated pack-analysis runtime call")
	require.Contains(t, batch, "Run a decision-value discovery pass")
	require.Contains(t, batch, "Pack-local refinement")
	require.Contains(t, batch, "Return a pattern only when all of these are true")
	require.Contains(t, batch, "verified reusable capability entry")
	require.Contains(t, batch, "decision-bearing calculations")
	require.Contains(t, batch, "attaching `business_method` to the candidate owned by that entry is required")
	require.Contains(t, batch, "Do not present an entry as the reuse route while omitting the corresponding capability contract")
	require.Contains(t, batch, "prefer one complete candidate for that entry")
	require.Contains(t, batch, "mutually exclusive")
	require.Contains(t, batch, "concrete operation or payload identity")
	require.Contains(t, batch, "performs no compensation or rollback")
	require.Contains(t, batch, "No retained candidate instructs a future Agent to use a named entry while omitting")
	require.Contains(t, batch, "fallback/base selection")
	require.Contains(t, batch, "The mode changes exploration breadth only")
	require.Contains(t, batch, "not a work procedure, a user-maintained Workflow, or development instructions")
	require.NotContains(t, batch, "pattern-evidence-rules")

	delta, err := loader.Render("learning-delta-pack-analyze", sampleCurrentDeltaData())
	require.NoError(t, err)
	require.Contains(t, delta, "isolated diff-pack analysis runtime call")
	require.Contains(t, delta, "Changed hunks are the source anchor")
	require.Contains(t, delta, "proposal.business_method")
	require.Contains(t, delta, "changed decision-bearing calculations")
	require.Contains(t, delta, "pure or stateless decision-bearing calculation")
	require.Contains(t, delta, "filling `proposal.business_method`")
	require.Contains(t, delta, "one canonical entry")
	require.Contains(t, delta, "concrete operation or payload identity")
	require.Contains(t, delta, "without compensation or rollback")
	require.Contains(t, delta, "Do not infer a new language, framework, architecture, or domain inventory")
	require.Contains(t, delta, "permission to learn outside the diff boundary")
	require.Contains(t, delta, "not a work procedure, a user-maintained Workflow, or development instructions")

	profile, err := loader.Render("learning-profile-refresh", sampleProjectProfileData(t))
	require.NoError(t, err)
	require.Contains(t, profile, "bounded profile-sync runtime call")
	require.Contains(t, profile, "Do not inventory the project for completeness")
	require.Contains(t, profile, "Avoid exact capability counts")
	require.NotContains(t, profile, "authority section catalog")

	authority, err := loader.Render("learning-authority-extract", sampleAuthorityExtractionData(t))
	require.NoError(t, err)
	require.Contains(t, authority, "bounded authority-extraction runtime call")
	require.Contains(t, authority, "complete one-to-one coverage")
	require.Contains(t, authority, "never implies `allowed`")

	review, err := loader.Render("learning-knowledge-review", sampleKnowledgeReviewData(t))
	require.NoError(t, err)
	require.Contains(t, review, "skeptical maintainer")
	require.Contains(t, review, "current implementation, design intent, proven guarantee, and known risk")
	require.Contains(t, review, "Removing the capability entry does not require rejecting")
	require.Contains(t, review, "every retained entry must be self-contained")
	require.Contains(t, review, "Do not accept routeable guidance while silently deleting or omitting")
	require.Contains(t, review, "decision-bearing operands")
	require.Contains(t, review, "one canonical entry")
	require.Contains(t, review, "no compensation or rollback path")

	normalize, err := loader.Render("learning-pattern-normalize", sampleNormalizePatternsData(t))
	require.NoError(t, err)
	require.Contains(t, normalize, "Treat each nonempty `capability_entry` identity as an ownership boundary")
	require.Contains(t, normalize, "No output pattern combines distinct nonempty `capability_entry` identities")

}

func currentPromptData(t *testing.T) map[string]interface{} {
	return map[string]interface{}{
		"learning-pack-plan":          samplePlanData(t),
		"learning-pack-analyze":       sampleCurrentBatchData(),
		"learning-delta-pack-analyze": sampleCurrentDeltaData(),
		"learning-profile-refresh":    sampleProjectProfileData(t),
		"learning-authority-extract":  sampleAuthorityExtractionData(t),
		"learning-knowledge-review":   sampleKnowledgeReviewData(t),
		"core-user-pattern":           sampleUserPatternData(),
		"core-workspace-profile":      sampleWorkspaceData(),
		"core-workspace-spec":         sampleWorkspaceData(),
	}
}

func sampleNormalizePatternsData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	pattern := *samplePattern("dispatcher-submit", "Submit work")
	pattern.BusinessMethod = &domain.BusinessMethod{
		Name:         "Submit",
		CodeLocation: domain.CodeLocation{CurrentLocation: "internal/job/dispatcher.go:41"},
	}
	data, err := agent.NormalizePatternsPromptData(session, &agent.NormalizePatternsRequest{
		ProjectName: "demo", RootPath: "/repo", Language: "mixed", Candidates: []domain.Pattern{pattern},
	})
	require.NoError(t, err)
	return data
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

func samplePlanData(t *testing.T) map[string]interface{} {
	session := newPromptInputSessionForTest(t)
	data, err := agent.PlanLearningAgendaPromptData(session, &agent.PlanLearningAgendaRequest{
		ProjectName:       "demo",
		RootPath:          "/repo",
		Language:          "go",
		FocusPaths:        []string{"internal/service/user.go"},
		StructuralContext: "UserService -> UserRepo",
		LearningMode:      config.LearningModeNormal,
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
