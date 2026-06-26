package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/stretchr/testify/require"
)

func TestSaveAgentOutputForContextStoresFilesUnderRuntimeMemory(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	archive := SaveAgentOutputForContext(ctx, AgentOutputArchiveOptions{
		Agent:           "claude",
		Operation:       "AnalyzeCurrentCodebase",
		Attempt:         1,
		Content:         `{"patterns":[]}`,
		RawOutput:       `{"type":"result","result":"{\"patterns\":[]}"}`,
		Stderr:          "warning",
		TokenUsageKnown: true,
	})

	require.Contains(t, filepath.ToSlash(archive.ContentPath), ".skills-seed/runtime/agent-outputs/")
	require.Contains(t, filepath.ToSlash(archive.RawPath), ".skills-seed/runtime/agent-outputs/")
	require.Contains(t, filepath.ToSlash(archive.StderrPath), ".skills-seed/runtime/agent-outputs/")
	require.Regexp(t, `^\d{8}-\d{6}\.\d{9}-agent-output-claude-analyzecurrentcodebase\.md$`, filepath.Base(archive.ContentPath))
	require.Regexp(t, `^\d{8}-\d{6}\.\d{9}-agent-output-claude-analyzecurrentcodebase\.raw\.txt$`, filepath.Base(archive.RawPath))
	require.Regexp(t, `^\d{8}-\d{6}\.\d{9}-agent-output-claude-analyzecurrentcodebase\.stderr\.txt$`, filepath.Base(archive.StderrPath))

	content, err := os.ReadFile(archive.ContentPath)
	require.NoError(t, err)
	require.Equal(t, "{\"patterns\":[]}\n", string(content))

	entries, err := os.ReadDir(filepath.Join(seedPath, "runtime", "agent-outputs"))
	require.NoError(t, err)
	var manifestPath string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".manifest.json") {
			require.Regexp(t, `^\d{8}-\d{6}\.\d{9}-agent-output-claude-analyzecurrentcodebase\.manifest\.json$`, entry.Name())
			manifestPath = filepath.Join(seedPath, "runtime", "agent-outputs", entry.Name())
		}
	}
	require.NotEmpty(t, manifestPath)

	var manifest struct {
		Agent           string `json:"agent"`
		Operation       string `json:"operation"`
		ContentPath     string `json:"content_path"`
		RawPath         string `json:"raw_path"`
		StderrPath      string `json:"stderr_path"`
		TokenUsageKnown bool   `json:"token_usage_known"`
	}
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, "claude", manifest.Agent)
	require.Equal(t, "AnalyzeCurrentCodebase", manifest.Operation)
	require.Equal(t, archive.ContentPath, manifest.ContentPath)
	require.Equal(t, archive.RawPath, manifest.RawPath)
	require.Equal(t, archive.StderrPath, manifest.StderrPath)
	require.True(t, manifest.TokenUsageKnown)
}

func TestSaveAgentOutputForContextSkipsWhenSeedPathMissing(t *testing.T) {
	archive := SaveAgentOutputForContext(context.Background(), AgentOutputArchiveOptions{
		Agent:     "claude",
		Operation: "AnalyzeCurrentCodebase",
		Content:   `{"patterns":[]}`,
	})

	require.Empty(t, archive.ContentPath)
	require.Empty(t, archive.RawPath)
	require.Empty(t, archive.StderrPath)
}
