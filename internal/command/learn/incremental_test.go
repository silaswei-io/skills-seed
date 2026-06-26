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
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
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

func TestPrepareIncrementalFileChangesSkipsDocumentsButKeepsDocsSource(t *testing.T) {
	ctx := context.Background()
	projectRoot := initLearnGitRepo(t)
	writeLearnFile(t, projectRoot, "main.go", "package main\n")
	writeLearnFile(t, projectRoot, "README.MD", "# readme\n")
	writeLearnFile(t, projectRoot, "docs/Guide.MD", "# guide\n")
	writeLearnFile(t, projectRoot, "docs/examples/demo.go", "package examples\n")
	gitAddAll(t, projectRoot)

	repo := newLearnTracker(t, projectRoot)
	configRepo := newIncrementalConfig(t, projectRoot)

	changes, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, nil)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"main.go", "docs/examples/demo.go"}, changes.AddedOrModified)
	require.Contains(t, changes.Skipped, "README.MD")
	require.Contains(t, changes.Skipped, "docs/Guide.MD")
	require.Equal(t, 2, changes.SkippedCount(fileanalysis.SkipReasonDocument))
}

func TestPrepareIncrementalFileChangesDoesNotDeleteCurrentGitIgnoredRecords(t *testing.T) {
	ctx := context.Background()
	projectRoot := initLearnGitRepo(t)
	writeLearnFile(t, projectRoot, ".gitignore", "ignored/\n")
	writeLearnFile(t, projectRoot, "main.go", "package main\n")
	writeLearnFile(t, projectRoot, "ignored/generated.go", "package ignored\n")
	gitAddAll(t, projectRoot)

	repo := newLearnTracker(t, projectRoot)
	require.NoError(t, repo.SaveAnalyzedFiles(ctx, []domain.FileAnalysisRecord{
		{Path: "main.go", Hash: "old-main"},
		{Path: "ignored/generated.go", Hash: "old-ignored"},
	}))
	configRepo := newIncrementalConfig(t, projectRoot)

	changes, err := prepareIncrementalFileChanges(ctx, repo, configRepo, projectRoot, projectRoot, domain.FileAnalysisScope{}, nil)

	require.NoError(t, err)
	require.NotContains(t, changes.Deleted, "ignored/generated.go")
	require.Empty(t, changes.Deleted)
}

func TestIncrementalFileChangesApplyAISelectionMarksUnselectedRecords(t *testing.T) {
	changes := &incrementalFileChanges{
		Records: []domain.FileAnalysisRecord{
			{Path: "internal/logic/create.go", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
			{Path: "internal/types/types.go", AnalysisStatus: domain.FileAnalysisStatusAnalyzed},
		},
	}

	changes.ApplyAISelection([]string{"internal/logic/create.go"}, "generated contract")

	byPath := map[string]domain.FileAnalysisRecord{}
	for _, record := range changes.Records {
		byPath[record.Path] = record
	}
	require.Equal(t, domain.FileAnalysisStatusAnalyzed, byPath["internal/logic/create.go"].AnalysisStatus)
	require.Empty(t, byPath["internal/logic/create.go"].SelectionReason)
	require.Equal(t, domain.FileAnalysisStatusAISkipped, byPath["internal/types/types.go"].AnalysisStatus)
	require.Equal(t, "generated contract", byPath["internal/types/types.go"].SelectionReason)
}

func TestConfiguredLearnExcludesSeparatesBuiltinsFromConfigDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	configRepo := newIncrementalConfig(t, projectRoot)

	excludes := fileanalysis.ConfiguredLearnExcludes(configRepo, projectRoot)

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
	repo, err := boltdb.NewPatternRepository(filepath.Join(root, ".skills-seed", "store", "project.db"))
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
