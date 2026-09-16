package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestResolveChildSkillTargetUsesRootConfig(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(projectRoot, 0755))
	rootConfig := &mocks.MockConfigReader{SkillsCfg: config.SkillsConfig{
		Target: "codex",
		Paths:  map[string]string{"codex": ".agents/skills/shared"},
	}}

	target, err := ResolveChildSkillTarget(workspaceRoot, config.WorkspaceProjectConfig{ID: "backend", Path: "backend"}, rootConfig)

	require.NoError(t, err)
	require.Equal(t, filepath.Join(projectRoot, ".agents/skills/shared"), target.OutputPath)
	require.False(t, target.UsesChildConfig)
	require.NoFileExists(t, target.ConfigPath)
}

func TestResolveChildSkillTargetUsesChildConfig(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "backend")
	childSeedPath := filepath.Join(projectRoot, ".skills-seed")
	require.NoError(t, os.MkdirAll(childSeedPath, 0755))
	childConfig, err := config.NewRepository(childSeedPath, "zh-CN")
	require.NoError(t, err)
	cfg := childConfig.Get()
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": ".agents/skills/child"}
	require.NoError(t, childConfig.Update(cfg))

	target, err := ResolveChildSkillTarget(workspaceRoot, config.WorkspaceProjectConfig{ID: "backend", Path: "backend"}, &mocks.MockConfigReader{})

	require.NoError(t, err)
	require.Equal(t, filepath.Join(projectRoot, ".agents/skills/child"), target.OutputPath)
	require.True(t, target.UsesChildConfig)
}

func TestResolveChildSkillTargetNormalizesLegacyDefault(t *testing.T) {
	workspaceRoot := t.TempDir()
	projectRoot := filepath.Join(workspaceRoot, "backend")
	require.NoError(t, os.MkdirAll(projectRoot, 0755))
	rootConfig := &mocks.MockConfigReader{SkillsCfg: config.SkillsConfig{
		Target: "codex",
		Paths:  map[string]string{"codex": ".agents/skills/skills-seed-skills"},
	}}

	target, err := ResolveChildSkillTarget(workspaceRoot, config.WorkspaceProjectConfig{ID: "backend", Path: "backend"}, rootConfig)

	require.NoError(t, err)
	require.Equal(t, filepath.Join(projectRoot, ".agents/skills/backend-dev"), target.OutputPath)
}

func TestResolveChildSkillTargetDoesNotInheritRootName(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "backend"), 0o755))
	cfg := &mocks.MockConfigReader{SkillsCfg: config.SkillsConfig{
		Name: "team-guide", Target: "codex", Paths: map[string]string{"codex": ".agents/skills/team-guide"},
	}}
	target, err := ResolveChildSkillTarget(root, config.WorkspaceProjectConfig{ID: "backend", Path: "backend"}, cfg)
	require.NoError(t, err)
	require.Equal(t, "backend-dev", target.SkillName)
	require.Equal(t, filepath.Join(root, "backend", ".agents", "skills", "backend-dev"), target.OutputPath)
}

func TestResolveChildSkillTargetPreservesExplicitLegacyName(t *testing.T) {
	root := t.TempDir()
	repo, err := config.NewRepository(filepath.Join(root, "backend", ".skills-seed"), "en-US")
	require.NoError(t, err)
	cfg := repo.Get()
	cfg.Project.Name = "backend"
	cfg.Skills = config.SkillsConfig{Name: "skills-seed-skills", Target: "codex"}
	require.NoError(t, repo.Update(cfg))
	target, err := ResolveChildSkillTarget(root, config.WorkspaceProjectConfig{ID: "backend", Path: "backend"}, &mocks.MockConfigReader{})
	require.NoError(t, err)
	require.Equal(t, "skills-seed-skills", target.SkillName)
	require.Equal(t, filepath.Join(root, "backend", ".agents", "skills", "skills-seed-skills"), target.OutputPath)
}
