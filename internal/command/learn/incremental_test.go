package learn

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
	"github.com/stretchr/testify/require"
)

func TestPrepareIncrementalFileChangesDetectsAddedModifiedAndDeleted(t *testing.T) {
	ctx := context.Background()
	projectRoot := initLearnGitRepo(t)
	writeLearnFile(t, projectRoot, "main.go", "package main\n")
	gitAddAll(t, projectRoot)

	repo := newLearnTracker(t, projectRoot)
	configRepo := newIncrementalConfig(t, projectRoot)
	scope := domain.FileAnalysisScope{}

	first, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, scope, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"main.go"}, first.AddedOrModified)
	require.NoError(t, commitIncrementalFileChanges(ctx, repo, first))

	writeLearnFile(t, projectRoot, "main.go", "package main\nconst changed = true\n")
	writeLearnFile(t, projectRoot, "internal/app.go", "package internal\n")
	gitAddAll(t, projectRoot)

	second, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, scope, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"main.go", "internal/app.go"}, second.AddedOrModified)
	require.Empty(t, second.Deleted)
	require.Equal(t, []string{"internal/app.go", "main.go"}, second.FocusPaths())

	require.NoError(t, commitIncrementalFileChanges(ctx, repo, second))
	require.NoError(t, os.Remove(filepath.Join(projectRoot, "internal", "app.go")))
	gitAddAll(t, projectRoot)

	third, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, scope, nil)
	require.NoError(t, err)
	require.Empty(t, third.AddedOrModified)
	require.Equal(t, []string{"internal/app.go"}, third.Deleted)
	require.Equal(t, []string{"internal/app.go"}, third.FocusPaths())
}

func TestPrepareIncrementalFileChangesExcludesGeneratedSkills(t *testing.T) {
	ctx := context.Background()
	projectRoot := initLearnGitRepo(t)
	writeLearnFile(t, projectRoot, "main.go", "package main\n")
	writeLearnFile(t, projectRoot, ".agents/skills/skills-seed-skills/SKILL.md", "# generated\n")
	writeLearnFile(t, projectRoot, ".claude/skills/skills-seed-skills/SKILL.md", "# generated\n")
	gitAddAll(t, projectRoot)

	repo := newLearnTracker(t, projectRoot)
	configRepo := newIncrementalConfig(t, projectRoot)

	changes, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"main.go"}, changes.AddedOrModified)
	require.ElementsMatch(t, []string{".agents/skills/skills-seed-skills", ".claude/skills/skills-seed-skills"}, changes.ExcludedGeneratedSkillDirs)
}

func TestConfiguredLearnExcludesSeparatesBuiltinsFromConfigDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	configRepo := newIncrementalConfig(t, projectRoot)

	excludes := configuredLearnExcludes(configRepo, projectRoot)

	require.Contains(t, excludes, ".git/**")
	require.Contains(t, excludes, ".skills-seed/**")
	require.Contains(t, excludes, ".claude/**")
	require.Contains(t, excludes, ".agents/**")
	require.Contains(t, excludes, ".*")
	require.Contains(t, excludes, "vendor/**")
	require.Contains(t, excludes, "node_modules/**")
}

func initLearnGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", root, "init").Run())
	require.NoError(t, exec.Command("git", "-C", root, "config", "user.email", "test@example.com").Run())
	require.NoError(t, exec.Command("git", "-C", root, "config", "user.name", "Test User").Run())
	return root
}

func writeLearnFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func gitAddAll(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, exec.Command("git", "-C", root, "add", "-A").Run())
}

func newLearnTracker(t *testing.T, root string) *boltdb.PatternRepository {
	t.Helper()
	repo, err := boltdb.NewPatternRepository(filepath.Join(root, ".skills-seed", "memory", "project.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	return repo
}

func newIncrementalConfig(t *testing.T, root string) *config.Repository {
	t.Helper()
	repo, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "zh-CN")
	require.NoError(t, err)
	cfg := repo.Get()
	cfg.Project.RootPath = root
	cfg.Agent.Engine = "codex"
	cfg.Skills.Paths = map[string]string{
		"claude": ".claude/skills/skills-seed-skills",
		"codex":  ".agents/skills/skills-seed-skills",
	}
	require.NoError(t, repo.Update(cfg))
	return repo
}
