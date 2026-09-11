package repositoryscopeconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestGeneratedSkillDirsUsesConfiguredProjectRelativePaths(t *testing.T) {
	projectRoot := t.TempDir()
	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "en-US")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Skills.Paths = map[string]string{
		"claude": ".claude/skills/demo",
		"codex":  ".agents/skills/demo",
		"empty":  "",
		"escape": "../outside",
	}
	require.NoError(t, configRepo.Update(cfg))

	require.Equal(t, []string{".agents/skills/demo", ".claude/skills/demo"}, GeneratedSkillDirs(configRepo, projectRoot))
	require.Empty(t, GeneratedSkillDirs(nil, t.TempDir()))
}

func TestKnowledgeExcludesIncludesConfiguredAndGeneratedPaths(t *testing.T) {
	projectRoot := t.TempDir()
	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "en-US")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Exclude.Paths = []string{"private/**"}
	cfg.Skills.Paths = map[string]string{"codex": ".agents/skills/demo"}
	require.NoError(t, configRepo.Update(cfg))

	excludes := KnowledgeExcludes(configRepo, projectRoot)
	require.Contains(t, excludes, "private/**")
	require.Contains(t, excludes, ".agents/skills/demo/**")
	require.NotEmpty(t, KnowledgeExcludes(nil, projectRoot))
}

func TestKnowledgeScopeCombinesConfigAndGitIgnore(t *testing.T) {
	projectRoot := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = projectRoot
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, ".gitignore"), []byte("ignored.go\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "ignored.go"), []byte("package ignored\n"), 0o644))

	configRepo, err := config.NewRepository(filepath.Join(projectRoot, ".skills-seed"), "en-US")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Exclude.GitIgnore = true
	cfg.Exclude.Paths = []string{"private/**"}
	require.NoError(t, configRepo.Update(cfg))

	scope := KnowledgeScope(configRepo, projectRoot)
	require.False(t, scope.AllowsKnowledge("ignored.go"))
	require.False(t, scope.AllowsKnowledge("private/value.go"))
	require.True(t, scope.AllowsKnowledge("internal/value.go"))

	cfg.Exclude.GitIgnore = false
	require.NoError(t, configRepo.Update(cfg))
	require.True(t, KnowledgeScope(configRepo, projectRoot).AllowsKnowledge("ignored.go"))
	require.False(t, KnowledgeScope(nil, projectRoot).AllowsKnowledge(".git/config"))
	require.False(t, DefaultKnowledgeScope().AllowsKnowledge("vendor/module.go"))
}

func TestKnowledgeScopeFallsBackWhenGitRepositoryIsUnavailable(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	configRepo, err := config.NewRepository(seedPath, "en-US")
	require.NoError(t, err)
	cfg := configRepo.Get()
	cfg.Exclude.GitIgnore = true
	require.NoError(t, configRepo.Update(cfg))

	scope := KnowledgeScope(configRepo, filepath.Join(t.TempDir(), "missing"))
	require.True(t, scope.AllowsKnowledge("ignored.go"))
}
