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

func TestAnalyzeCurrentCodebasePromptDataNormalizesStructureInputFile(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := AnalyzeCurrentCodebasePromptData(session, &AnalyzeCurrentCodebaseRequest{
		Structure: "demo\n\u00a0\u00a0cmd\n&nbsp;&nbsp;main.go   \n",
	})
	require.NoError(t, err)

	structurePath, ok := data["StructurePath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "project-structure.txt"), structurePath)

	content, err := os.ReadFile(structurePath)
	require.NoError(t, err)
	text := string(content)
	require.Equal(t, "demo\n  cmd\n  main.go\n", text)
	require.NotContains(t, text, "\u00a0")
	require.NotContains(t, text, "&nbsp;")
	require.NotContains(t, text, "main.go   ")
}

func TestCurrentLearningPromptDataIncludesLearningMode(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	planData, err := PlanAnalysisUnitsPromptData(session, &PlanAnalysisUnitsRequest{})
	require.NoError(t, err)
	require.Equal(t, config.LearningModeNormal, planData["LearningMode"])
	require.Equal(t, config.LearningScopeFlow, planData["LearningScope"])

	currentData, err := AnalyzeCurrentCodebasePromptData(session, &AnalyzeCurrentCodebaseRequest{
		LearningMode: config.LearningModeDeep,
	})
	require.NoError(t, err)
	require.Equal(t, config.LearningModeDeep, currentData["LearningMode"])
}

func TestPlanAnalysisUnitsPromptDataWritesFocusedPathList(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := PlanAnalysisUnitsPromptData(session, &PlanAnalysisUnitsRequest{
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

func TestSelectFilesPromptDataWritesRuntimeInputs(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := SelectFilesPromptData(session, &SelectFilesRequest{
		FileTree:   "cmd/server/main.go\ninternal/orders/handler.go\n",
		Candidates: []FileSelectionCandidate{{Path: "cmd/server/main.go", Kind: "source"}},
	})
	require.NoError(t, err)

	fileListPath, ok := data["FileListPath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "candidate-files.txt"), fileListPath)

	fileList, err := os.ReadFile(fileListPath)
	require.NoError(t, err)
	require.Contains(t, string(fileList), "cmd/server/main.go")
	require.Empty(t, data["StructuralContextPath"])
	require.NoFileExists(t, filepath.Join(session.dir, "structural-context.md"))
}

func TestSelectFilesPromptDataWritesStructuralContextWhenPresent(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	data, err := SelectFilesPromptData(session, &SelectFilesRequest{
		FileTree:          "cmd/server/main.go\n",
		StructuralContext: "## Structural Context\n- cmd/server/main.go defines main",
	})
	require.NoError(t, err)

	contextPath, ok := data["StructuralContextPath"].(string)
	require.True(t, ok)
	require.Equal(t, filepath.Join(session.dir, "structural-context.md"), contextPath)

	content, err := os.ReadFile(contextPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "cmd/server/main.go defines main")
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

func TestPromptInputSessionForContextKeepsRuntimeInputsForDebugging(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	session, err := NewPromptInputSessionForContext(ctx, "skills-seed-project-profile")
	require.NoError(t, err)
	inputPath, err := session.Write("project-structure.txt", "demo\n  main.go")
	require.NoError(t, err)

	session.Cleanup()

	require.FileExists(t, inputPath)
	require.Contains(t, filepath.ToSlash(inputPath), ".skills-seed/runtime")
	require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-prompt-input-skills-seed-project-profile-\d+$`, filepath.Base(filepath.Dir(inputPath)))
}
