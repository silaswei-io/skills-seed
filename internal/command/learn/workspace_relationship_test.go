package learn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/test/mocks"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceChildSkillPathUsesRootConfigWithoutCreatingChildConfig(t *testing.T) {
	projectRoot := t.TempDir()
	childSeedPath := filepath.Join(projectRoot, ".skills-seed")
	rootConfig := &mocks.MockConfigReader{
		SkillsCfg: config.SkillsConfig{
			Target: "codex",
			Paths:  map[string]string{"codex": ".agents/skills/shared"},
		},
	}

	skillPath, err := workspaceChildSkillPath(projectRoot, childSeedPath, rootConfig)

	require.NoError(t, err)
	require.Equal(t, ".agents/skills/shared", skillPath)
	require.NoFileExists(t, filepath.Join(childSeedPath, "config.yaml"))
}

func TestWorkspaceChildSkillPathUsesChildConfigWhenPresent(t *testing.T) {
	projectRoot := t.TempDir()
	childSeedPath := filepath.Join(projectRoot, ".skills-seed")
	require.NoError(t, os.MkdirAll(childSeedPath, 0755))
	childConfig, err := config.NewRepository(childSeedPath, "zh-CN")
	require.NoError(t, err)
	cfg := childConfig.Get()
	cfg.Skills.Target = "codex"
	cfg.Skills.Paths = map[string]string{"codex": ".agents/skills/child"}
	require.NoError(t, childConfig.Update(cfg))

	skillPath, err := workspaceChildSkillPath(projectRoot, childSeedPath, &mocks.MockConfigReader{})

	require.NoError(t, err)
	require.Equal(t, ".agents/skills/child", skillPath)
}
