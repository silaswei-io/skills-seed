package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/stretchr/testify/require"
)

func TestCurrentLearningPromptDataIncludesLearningMode(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	planData, err := PlanLearningAgendaPromptData(session, &PlanLearningAgendaRequest{})
	require.NoError(t, err)
	require.Equal(t, config.LearningModeNormal, planData["LearningMode"])
	require.Equal(t, config.LearningScopeFlow, planData["LearningScope"])

	currentData, err := AnalyzeCurrentCodebaseBatchPromptData(session, &AnalyzeCurrentCodebaseBatchRequest{
		LearningMode: config.LearningModeDeep,
	})
	require.NoError(t, err)
	require.Equal(t, config.LearningModeDeep, currentData["LearningMode"])
}

func TestPlanLearningAgendaPromptDataWritesFocusedPathList(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := PlanLearningAgendaPromptData(session, &PlanLearningAgendaRequest{
		FocusPaths: []string{"internal/key/create.go", "internal/auth/login.go", "internal/auth/login.go"},
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
}

func TestSelectLearningCandidatesPromptDataWritesCandidateAndRequiredPathLists(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := SelectLearningCandidatesPromptData(session, &SelectLearningCandidatesRequest{
		CandidatePaths: []string{"internal/key/create.go", "internal/auth/login.go", "internal/auth/login.go"},
		RequiredPaths:  []string{"internal/auth/login.go"},
	})
	require.NoError(t, err)

	candidatePath, ok := data["CandidatePathsPath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "candidate-files.txt"), candidatePath)
	require.Equal(t, 2, data["CandidatePathCount"])

	requiredPath, ok := data["RequiredPathsPath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "required-files.txt"), requiredPath)
	require.Equal(t, 1, data["RequiredPathCount"])

	content, err := os.ReadFile(candidatePath)
	require.NoError(t, err)
	require.Equal(t, "internal/auth/login.go\ninternal/key/create.go\n", string(content))
	require.NotContains(t, data, "CandidatePaths")
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

func TestAnalyzeProjectPromptDataWritesEngineeringKnowledgeList(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := AnalyzeProjectPromptData(session, &AnalyzeProjectRequest{
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
