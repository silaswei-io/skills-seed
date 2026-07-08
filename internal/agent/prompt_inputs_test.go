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

func TestSelectFilesPromptDataInlinesStructuralContext(t *testing.T) {
	session := &PromptInputSession{dir: t.TempDir()}

	structuralContext := "## Structural Context\n\n### Entry Points\n- func main (cmd/server/main.go:3)\n"
	data, err := SelectFilesPromptData(session, &SelectFilesRequest{
		Candidates:        []FileSelectionCandidate{{Path: "cmd/server/main.go", Kind: "source"}},
		StructuralContext: structuralContext,
	})
	require.NoError(t, err)

	require.Equal(t, structuralContext, data["StructuralContext"])
	require.NotContains(t, data, "StructuralContextPath")
	require.NoFileExists(t, filepath.Join(session.dir, "structural-context.md"))
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
	require.Regexp(t, `^\d{8}-\d{6}-prompt-input-skills-seed-project-profile-\d+$`, filepath.Base(filepath.Dir(inputPath)))
}
