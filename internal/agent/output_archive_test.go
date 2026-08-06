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
		Agent:     "claude",
		Operation: "AnalyzeCurrentCodebase",
		Attempt:   1,
		Content:   `{"patterns":[]}`,
		RawOutput: `{"type":"result","result":"{\"patterns\":[]}"}`,
		Stderr:    "warning",
	})

	require.Contains(t, filepath.ToSlash(archive.ContentPath), ".skills-seed/runtime/agent-outputs/")
	require.Contains(t, filepath.ToSlash(archive.RawPath), ".skills-seed/runtime/agent-outputs/")
	require.Contains(t, filepath.ToSlash(archive.StderrPath), ".skills-seed/runtime/agent-outputs/")
	require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-claude-analyzecurrentcodebase\.md$`, filepath.Base(archive.ContentPath))
	require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-claude-analyzecurrentcodebase\.raw\.txt$`, filepath.Base(archive.RawPath))
	require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-claude-analyzecurrentcodebase\.stderr\.txt$`, filepath.Base(archive.StderrPath))

	content, err := os.ReadFile(archive.ContentPath)
	require.NoError(t, err)
	require.Equal(t, "```json\n{\n  \"patterns\": []\n}\n```\n", string(content))

	entries, err := os.ReadDir(filepath.Join(seedPath, "runtime", "agent-outputs"))
	require.NoError(t, err)
	var manifestPath string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".manifest.json") {
			require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-claude-analyzecurrentcodebase\.manifest\.json$`, entry.Name())
			manifestPath = filepath.Join(seedPath, "runtime", "agent-outputs", entry.Name())
		}
	}
	require.NotEmpty(t, manifestPath)

	var manifest struct {
		Agent       string `json:"agent"`
		Operation   string `json:"operation"`
		ContentPath string `json:"content_path"`
		RawPath     string `json:"raw_path"`
		StderrPath  string `json:"stderr_path"`
	}
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, "claude", manifest.Agent)
	require.Equal(t, "AnalyzeCurrentCodebase", manifest.Operation)
	require.Equal(t, archive.ContentPath, manifest.ContentPath)
	require.Equal(t, archive.RawPath, manifest.RawPath)
	require.Equal(t, archive.StderrPath, manifest.StderrPath)
	require.NotContains(t, string(data), "token_usage")
	require.NotContains(t, string(data), "tokens")
}

func TestSaveAgentOutputForContextPreservesRetryAttempts(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)
	first := SaveAgentOutputForContext(ctx, AgentOutputArchiveOptions{
		Agent: "claude", Operation: "Analyze", RuntimeID: "20260722-120000", Attempt: 1, RawOutput: "first",
	})
	second := SaveAgentOutputForContext(ctx, AgentOutputArchiveOptions{
		Agent: "claude", Operation: "Analyze", RuntimeID: "20260722-120000", Attempt: 2, RawOutput: "second",
	})

	require.NotEqual(t, first.RawPath, second.RawPath)
	require.Equal(t, "20260722-120000-claude-analyze.raw.txt", filepath.Base(first.RawPath))
	require.Equal(t, "20260722-120000-claude-analyze-attempt-002.raw.txt", filepath.Base(second.RawPath))
	require.FileExists(t, first.RawPath)
	require.FileExists(t, second.RawPath)
}

func TestSaveAgentOutputForContextLabelsUnitOperation(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	archive := SaveAgentOutputForContext(ctx, AgentOutputArchiveOptions{
		Agent:     "claude",
		Operation: "AnalyzeCurrentCodebase/focus-auth",
		Attempt:   1,
		Content:   `{"patterns":[]}`,
	})

	require.Regexp(t, `^\d{8}-\d{6}(?:-\d{3,})?-claude-analyzecurrentcodebase-focus-auth\.md$`, filepath.Base(archive.ContentPath))

	entries, err := os.ReadDir(filepath.Join(seedPath, "runtime", "agent-outputs"))
	require.NoError(t, err)
	var manifestPath string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".manifest.json") {
			manifestPath = filepath.Join(seedPath, "runtime", "agent-outputs", entry.Name())
		}
	}
	require.NotEmpty(t, manifestPath)

	var manifest struct {
		Operation string `json:"operation"`
		Label     string `json:"label"`
	}
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, "AnalyzeCurrentCodebase/focus-auth", manifest.Operation)
	require.Equal(t, "focus-auth", manifest.Label)
}

func TestSaveAgentOutputForContextUsesSharedRuntimeTask(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	archive := SaveAgentOutputForContext(ctx, AgentOutputArchiveOptions{
		Agent:     "claude",
		Operation: "AnalyzeCurrentCodebaseBatch",
		RuntimeID: "20260626-183633",
		Slug:      "learning-pack-analyze-core",
		Attempt:   1,
		Content:   `{"patterns":[],"dropped":[]}`,
	})

	require.Equal(t, "20260626-183633-claude-learning-pack-analyze-core.md", filepath.Base(archive.ContentPath))
	entries, err := os.ReadDir(filepath.Join(seedPath, "runtime", "agent-outputs"))
	require.NoError(t, err)
	var manifestPath string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".manifest.json") {
			manifestPath = filepath.Join(seedPath, "runtime", "agent-outputs", entry.Name())
		}
	}
	var manifest struct {
		RuntimeID string `json:"runtime_id"`
		Slug      string `json:"slug"`
	}
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, "20260626-183633", manifest.RuntimeID)
	require.Equal(t, "learning-pack-analyze-core", manifest.Slug)
}

func TestSaveAgentOutputForContextKeepsReadableUnitSlug(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	archive := SaveAgentOutputForContext(ctx, AgentOutputArchiveOptions{
		Agent:     "claude",
		Operation: "AnalyzeCurrentCodebaseBatch/focus-auth-admin-login",
		RuntimeID: "20260626-192529",
		Slug:      RuntimeSlug("learning-pack-analyze", "focus-auth-admin-login"),
		Attempt:   1,
		Content:   `{"patterns":[]}`,
	})

	require.Equal(t, "20260626-192529-claude-learning-pack-analyze-focus-auth-admin-login.md", filepath.Base(archive.ContentPath))
}

func TestSaveAgentOutputForContextKeepsNonJSONContentAsMarkdown(t *testing.T) {
	projectRoot := t.TempDir()
	seedPath := filepath.Join(projectRoot, ".skills-seed")
	ctx := runtimecontext.WithSeedPath(context.Background(), seedPath)

	archive := SaveAgentOutputForContext(ctx, AgentOutputArchiveOptions{
		Agent:     "claude",
		Operation: "Explain",
		RuntimeID: "20260626-192529",
		Content:   "plain text",
	})

	require.Equal(t, "20260626-192529-claude-explain.md", filepath.Base(archive.ContentPath))
	content, err := os.ReadFile(archive.ContentPath)
	require.NoError(t, err)
	require.Equal(t, "plain text\n", string(content))
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
