package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSkillsName(t *testing.T) {
	for _, name := range []string{"", "a", "7", "my-team-2", "skills-seed-skills", strings.Repeat("a", 64)} {
		t.Run("valid/"+name, func(t *testing.T) {
			require.NoError(t, ValidateSkillsName(name))
		})
	}
	for _, name := range []string{"../escape", "/tmp/escape", `a\b`, "UPPER", "a_b", "a.b", "中文", " ", " a", "a ", "a\nb", "-a", "a-", "a--b", strings.Repeat("a", 65)} {
		t.Run("invalid/"+name, func(t *testing.T) {
			require.Error(t, ValidateSkillsName(name))
		})
	}
}

func TestSkillsNamePersistenceAndValidation(t *testing.T) {
	seedPath := t.TempDir()
	repo, err := NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	cfg := *repo.Get()
	cfg.Skills.Name = "team-guide"
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = nil
	require.NoError(t, repo.Update(&cfg))

	loaded, err := NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	require.Equal(t, "team-guide", loaded.GetSkillsConfig().Name)
	require.Equal(t, ".agents/skills/team-guide", loaded.GetSkillsConfig().Paths["codex"])

	path := filepath.Join(seedPath, "config.yaml")
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	cfg.Skills.Name = "../escape"
	require.Error(t, loaded.Update(&cfg))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, "team-guide", loaded.GetSkillsConfig().Name)

	require.NoError(t, os.WriteFile(path, []byte("skills:\n  name: '../escape'\n"), 0o644))
	_, err = NewRepository(seedPath, "en-US")
	require.Error(t, err)
}
