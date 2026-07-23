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
