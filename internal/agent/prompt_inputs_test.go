package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/knowledge/maintained"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/stretchr/testify/require"
)

func TestNormalizePatternsPromptDataWritesCapabilityEntryIdentity(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}
	pattern := *domain.NewPattern("dispatcher-submit", "Submit work", domain.CategoryBusiness)
	pattern.BusinessMethod = &domain.BusinessMethod{
		Name: " Submit ",
		CodeLocation: domain.CodeLocation{
			CurrentLocation:    " internal/job/dispatcher.go:41 ",
			HistoricalLocation: " internal/job/dispatcher.go:17 ",
		},
	}

	data, err := NormalizePatternsPromptData(session, &NormalizePatternsRequest{Candidates: []domain.Pattern{pattern}})

	require.NoError(t, err)
	path, ok := data["CandidatePatternsPath"].(string)
	require.True(t, ok)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"dispatcher-submit","name":"Submit work","category":"business","capability_entry":{"name":"Submit","current_location":"internal/job/dispatcher.go:41","historical_location":"internal/job/dispatcher.go:17"}}]`, string(content))
}

func TestReviewKnowledgePromptDataWritesEvidenceFocus(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}
	focus := domain.EvidenceFocus{
		ID:          "lifecycle",
		Name:        "Lifecycle",
		EntryPaths:  []string{"internal/lifecycle.ext"},
		ScopeReason: "The transition and persistence boundary must be reviewed together.",
	}

	data, err := ReviewKnowledgePromptData(session, &ReviewKnowledgeRequest{EvidenceFocus: focus})

	require.NoError(t, err)
	path, ok := data["EvidenceFocusPath"].(string)
	require.True(t, ok)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"lifecycle","name":"Lifecycle","entry_paths":["internal/lifecycle.ext"],"scope_reason":"The transition and persistence boundary must be reviewed together."}`, string(content))
}

func TestReviewKnowledgePromptDataIncludesExactCandidateIDs(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}
	data, err := ReviewKnowledgePromptData(session, &ReviewKnowledgeRequest{Candidates: []domain.Pattern{
		*domain.NewPattern(" state-transition ", "State transition", domain.CategoryBusiness),
		*domain.NewPattern("", "Ignored", domain.CategoryBusiness),
	}})

	require.NoError(t, err)
	require.Equal(t, []string{"state-transition"}, data["CandidateIDs"])
}

func TestLearningPromptDataWritesMaintainedGuidanceSeparately(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}
	data, err := AnalyzeCurrentCodebaseBatchPromptData(session, &AnalyzeCurrentCodebaseBatchRequest{
		MaintainedGuidance: maintained.Snapshot{
			Rules:     []domain.Rule{{ID: "api-boundary", Content: "Keep API identifiers as text."}},
			Workflows: []domain.Workflow{{ID: "verify", Content: "Run the verification workflow."}},
		},
	})

	require.NoError(t, err)
	path, ok := data["MaintainedGuidancePath"].(string)
	require.True(t, ok)
	require.NotEmpty(t, path)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{"rules":[{"id":"api-boundary","content":"Keep API identifiers as text."}],"workflows":[{"id":"verify","content":"Run the verification workflow."}]}`, string(content))
}

func TestCurrentLearningPromptDataIncludesLearningMode(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	planData, err := PlanLearningAgendaPromptData(session, &PlanLearningAgendaRequest{})
	require.NoError(t, err)
	require.Equal(t, config.LearningModeNormal, planData["LearningMode"])
	require.NotContains(t, planData, "LearningScope")

	currentData, err := AnalyzeCurrentCodebaseBatchPromptData(session, &AnalyzeCurrentCodebaseBatchRequest{
		LearningMode: config.LearningModeDeep,
	})
	require.NoError(t, err)
	require.Equal(t, config.LearningModeDeep, currentData["LearningMode"])
}

func TestCurrentBatchPromptDataKeepsSharedContextPath(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}
	sharedPath := filepath.Join(t.TempDir(), "shared-context.md")

	currentData, err := AnalyzeCurrentCodebaseBatchPromptData(session, &AnalyzeCurrentCodebaseBatchRequest{
		SharedContextPath: sharedPath,
	})
	require.NoError(t, err)
	require.Equal(t, sharedPath, currentData["SharedContextPath"])

	deltaData, err := AnalyzeCurrentDeltaBatchPromptData(session, &AnalyzeCurrentDeltaBatchRequest{
		SharedContextPath: sharedPath,
	})
	require.NoError(t, err)
	require.Equal(t, sharedPath, deltaData["SharedContextPath"])
	require.NoFileExists(t, filepath.Join(session.dir, "shared-context.md"))
}

func TestPlanLearningAgendaPromptDataWritesFocusedPathList(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := PlanLearningAgendaPromptData(session, &PlanLearningAgendaRequest{
		FocusPaths: []string{"internal/key/create.go", "internal/auth/login.go", "internal/auth/login.go"},
		SourceFacts: []PlanningSourceFact{{
			Path: "internal/auth/login.go", SizeBytes: 128, LineCount: 8, NonBlankLines: 6, SymbolCount: 1,
			Symbols: []PlanningSymbolFact{{Name: "Login", Kind: "function", Line: 3}},
		}},
	})
	require.NoError(t, err)

	path, ok := data["FocusPathsPath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "analysis-files.txt"), path)
	require.Equal(t, 2, data["FocusPathCount"])

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "internal/auth/login.go\ninternal/key/create.go\n", string(content))
	require.NotContains(t, data, "FocusPaths")

	factsPath, ok := data["SourceFactsPath"].(string)
	require.True(t, ok)
	require.Equal(t, 1, data["SourceFactCount"])
	facts, err := os.ReadFile(factsPath)
	require.NoError(t, err)
	require.JSONEq(t, `[{"path":"internal/auth/login.go","size_bytes":128,"line_count":8,"non_blank_lines":6,"symbol_count":1,"symbols":[{"name":"Login","kind":"function","line":3}]}]`, string(facts))
}

func TestAnalyzeProjectPromptDataNormalizesStructureInputFile(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := AnalyzeProjectPromptData(session, &AnalyzeProjectRequest{
		Structure: "demo\n\u00a0\u00a0internal\n&nbsp;&nbsp;service.go   \n",
	})
	require.NoError(t, err)

	structurePath, ok := data["StructurePath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "project-structure.txt"), structurePath)

	content, err := os.ReadFile(structurePath)
	require.NoError(t, err)
	text := string(content)
	require.Equal(t, "demo\n  internal\n  service.go\n", text)
	require.NotContains(t, text, "\u00a0")
	require.NotContains(t, text, "&nbsp;")
	require.NotContains(t, text, "service.go   ")
}

func TestAnalyzeProjectPromptDataWritesFocusedPathList(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := AnalyzeProjectPromptData(session, &AnalyzeProjectRequest{
		FocusPaths: []string{"internal/profile/service.go", "cmd/demo/main.go"},
	})
	require.NoError(t, err)

	path, ok := data["FocusPathsPath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "focused-paths.txt"), path)
	require.Equal(t, 2, data["FocusPathCount"])

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "cmd/demo/main.go\ninternal/profile/service.go\n", string(content))
	require.NotContains(t, data, "FocusPaths")
}

func TestExtractAuthorityPromptDataWritesEngineeringKnowledgeList(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := ExtractAuthorityPromptData(session, &ExtractAuthorityRequest{
		EngineeringKnowledge: []string{"AGENTS.md", "Taskfile.yml"},
	})

	require.NoError(t, err)
	path, ok := data["EngineeringKnowledgePath"].(string)
	require.True(t, ok)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "AGENTS.md\nTaskfile.yml\n", string(content))
	require.Equal(t, 2, data["EngineeringKnowledgeCount"])
}

func TestExtractAuthorityPromptDataWritesAuthoritySectionCatalog(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}
	sections := []AuthoritySection{
		{ID: "authority-first", Source: "GUIDE.md", Section: "Contracts"},
		{ID: "authority-second", Source: "Taskfile.yml"},
	}

	data, err := ExtractAuthorityPromptData(session, &ExtractAuthorityRequest{AuthoritySections: sections})

	require.NoError(t, err)
	path, ok := data["AuthoritySectionsPath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "authority-sections.json"), path)
	require.Equal(t, 2, data["AuthoritySectionCount"])
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"section_id":"authority-first","source":"GUIDE.md","section":"Contracts"},
		{"section_id":"authority-second","source":"Taskfile.yml"}
	]`, string(content))
}

func TestReviewAuthorityPromptDataWritesInitialCandidate(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}
	request := &ReviewAuthorityRequest{
		ExtractAuthorityRequest: ExtractAuthorityRequest{
			EngineeringKnowledge: []string{"AGENTS.md"},
			AuthoritySections:    []AuthoritySection{{ID: "authority-project", Source: "AGENTS.md", Section: "Generated Files"}},
		},
		Candidate: ExtractAuthorityResult{AuthoritySections: []AuthoritySectionResult{{
			SectionID: "authority-project",
			Rules:     []domain.EngineeringRule{{Title: "Derived artifacts", Rule: "Do not edit derived files directly."}},
		}}},
	}

	data, err := ReviewAuthorityPromptData(session, request)

	require.NoError(t, err)
	path, ok := data["CandidatePath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "authority-candidate.json"), path)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"authority_sections":[{
			"section_id":"authority-project",
			"rules":[{"title":"Derived artifacts","rule":"Do not edit derived files directly.","source":""}]
		}]
	}`, string(content))
}

func TestPromptInputSessionForContextKeepsRuntimeInputsForDebugging(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	session, err := NewPromptInputSessionForContext(ctx, "skills-seed-learning-profile-refresh")
	require.NoError(t, err)
	inputPath, err := session.Write("project-structure.txt", "demo\n  main.go")
	require.NoError(t, err)

	session.Cleanup()

	require.FileExists(t, inputPath)
	require.Contains(t, filepath.ToSlash(inputPath), ".skills-seed/runtime")
	require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-prompt-input-skills-seed-learning-profile-refresh-\d+$`, filepath.Base(filepath.Dir(inputPath)))
}
