package metadata

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedTreeHashDeterministic(t *testing.T) {
	fsys := fstest.MapFS{
		"root/b.txt": {Data: []byte("beta")},
		"root/a.txt": {Data: []byte("alpha")},
	}

	first, err := EmbeddedTreeHash(fsys, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash() error = %v", err)
	}
	second, err := EmbeddedTreeHash(fsys, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash() second error = %v", err)
	}

	if first != second {
		t.Fatalf("hash must be deterministic, first=%q second=%q", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("hash length = %d, want 64", len(first))
	}
}

func TestTemplateProviderFallbacks(t *testing.T) {
	require.Equal(t, []string{"codex", "common"}, TemplateProviderFallbacks(" CODEX "))
	require.Equal(t, []string{"common"}, TemplateProviderFallbacks("common"))
	require.Equal(t, []string{"common"}, TemplateProviderFallbacks(""))
	require.Equal(t, []string{"claude", "loader"}, PromptTemplateProviderFallbacks("Claude"))
	require.Equal(t, []string{"loader"}, PromptTemplateProviderFallbacks("loader"))
}

func TestTemplatePaths(t *testing.T) {
	require.Equal(t, "templates/prompts/codex/analyze.txt.tmpl", PromptTemplatePath("codex", "analyze", ""))
	require.Equal(t, "templates/prompts/codex/analyze.en-US.txt.tmpl", PromptTemplatePath("codex", "analyze", "en-US"))
	require.Equal(t, "templates/prompts/append/guard.txt.tmpl", PromptAppendTemplatePath("guard", ""))
	require.Equal(t, "templates/prompts/append/guard.en-US.txt.tmpl", PromptAppendTemplatePath("guard", "en-US"))
	require.Equal(t, "templates/skills/codex/SKILL.md.tmpl", SkillsTemplatePath("codex", "SKILL", "", ""))
	require.Equal(t, "templates/skills/codex/SKILL.en-US.txt.tmpl", SkillsTemplatePath("codex", "SKILL", "en-US", ".txt.tmpl"))
	require.Equal(t, "templates/skills/codex/agents", SkillsAgentMetadataDir("codex"))
}

func TestTemplateTreeHashWrappers(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/prompts/a.txt": {Data: []byte("prompt")},
		"templates/seed/a.txt":    {Data: []byte("seed")},
		"templates/skills/a.txt":  {Data: []byte("skill")},
	}
	for _, hash := range []func(fstest.MapFS) (string, error){
		func(f fstest.MapFS) (string, error) { return PromptTemplatesHash(f) },
		func(f fstest.MapFS) (string, error) { return SeedTemplatesHash(f) },
		func(f fstest.MapFS) (string, error) { return SkillsTemplatesHash(f) },
	} {
		value, err := hash(fsys)
		require.NoError(t, err)
		require.Len(t, value, 64)
	}

	_, err := EmbeddedTreeHash(fsys, "missing")
	require.Error(t, err)
}

func TestHashOrUnavailable(t *testing.T) {
	require.Equal(t, "hash", HashOrUnavailable("hash", nil))
	require.Equal(t, UnavailableHash, HashOrUnavailable("", nil))
	require.Equal(t, UnavailableHash, HashOrUnavailable("hash", errors.New("failure")))
}

func TestEmbeddedTreeHashTracksContentAndPath(t *testing.T) {
	base := fstest.MapFS{
		"root/a.txt": {Data: []byte("alpha")},
	}
	changedContent := fstest.MapFS{
		"root/a.txt": {Data: []byte("beta")},
	}
	changedPath := fstest.MapFS{
		"root/b.txt": {Data: []byte("alpha")},
	}

	baseHash, err := EmbeddedTreeHash(base, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash(base) error = %v", err)
	}
	changedContentHash, err := EmbeddedTreeHash(changedContent, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash(changedContent) error = %v", err)
	}
	changedPathHash, err := EmbeddedTreeHash(changedPath, "root")
	if err != nil {
		t.Fatalf("EmbeddedTreeHash(changedPath) error = %v", err)
	}

	if baseHash == changedContentHash {
		t.Fatal("hash should change when file content changes")
	}
	if baseHash == changedPathHash {
		t.Fatal("hash should change when file path changes")
	}
}

func TestSeedContextTemplatePath(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{
			name: "default locale",
			want: "templates/seed/context/background.md.tmpl",
		},
		{
			name:   "localized",
			locale: "en-US",
			want:   "templates/seed/context/background.en-US.md.tmpl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SeedContextTemplatePath("background", tt.locale)
			if got != tt.want {
				t.Fatalf("SeedContextTemplatePath() = %q, want %q", got, tt.want)
			}
		})
	}
}
