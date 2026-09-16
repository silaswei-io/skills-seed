package initcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/stretchr/testify/require"
)

func TestInitCommandCustomSkillsName(t *testing.T) {
	for _, target := range []string{"claude", "codex", "custom"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			initGitDir(t, root)
			t.Chdir(root)
			cmd := Cmd()
			cmd.SetArgs([]string{"--skills-name", "team-guide", "--skills", target, "--no-interactive"})
			require.NoError(t, cmd.Execute())
			repo, err := config.NewRepository(filepath.Join(root, ".skills-seed"), "en-US")
			require.NoError(t, err)
			require.Equal(t, filepath.Base(root), repo.GetProjectConfig().Name)
			require.Equal(t, "team-guide", repo.GetSkillsConfig().Name)
			for _, outputTarget := range []string{"claude", "codex", target} {
				require.Equal(t, skillsPathForTargetAndName(outputTarget, "team-guide"), repo.GetSkillsConfig().Paths[outputTarget])
			}
		})
	}
}

func TestInitRejectsInvalidSkillsNameBeforeWriting(t *testing.T) {
	root := t.TempDir()
	initGitDir(t, root)
	t.Chdir(root)
	cmd := Cmd()
	cmd.SetArgs([]string{"--skills-name", "../escape", "--no-interactive"})
	require.Error(t, cmd.Execute())
	require.NoDirExists(t, filepath.Join(root, ".skills-seed"))
	require.Error(t, initializeSkillWithOptions(root, "en-US", domain.ModeProject, initializeSkillOptions{skillsName: "../escape"}))
	require.NoDirExists(t, filepath.Join(root, ".skills-seed"))

	require.NoError(t, initializeSkillWithOptions(root, "en-US", domain.ModeProject, initializeSkillOptions{}))
	path := filepath.Join(root, ".skills-seed", "config.yaml")
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Error(t, resetSkillWithOptions("en-US", domain.ModeProject, initializeSkillOptions{skillsName: "../escape"}))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.NoDirExists(t, filepath.Join(root, ".skills-seed.backup"))
}

func TestWorkspaceCustomSkillsNameIsNotInherited(t *testing.T) {
	root := t.TempDir()
	initGitDir(t, root)
	for _, child := range []string{"new-child", "existing-child"} {
		childRoot := filepath.Join(root, child)
		require.NoError(t, os.MkdirAll(childRoot, 0o755))
		initGitDir(t, childRoot)
		require.NoError(t, os.WriteFile(filepath.Join(childRoot, "go.mod"), []byte("module example.com/"+child+"\n"), 0o644))
	}
	require.NoError(t, initializeSkillWithOptions(filepath.Join(root, "existing-child"), "en-US", domain.ModeProject, initializeSkillOptions{skillsName: "child-guide"}))
	require.NoError(t, initializeSkillWithOptions(root, "en-US", domain.ModeWorkspace, initializeSkillOptions{skillsName: "team-guide", skillsTarget: "codex"}))
	for _, tt := range []struct{ path, name, output string }{
		{"", "team-guide", ".agents/skills/team-guide"},
		{"new-child", "", ".agents/skills/new-child-dev"},
		{"existing-child", "child-guide", ".claude/skills/child-guide"},
	} {
		repo, err := config.NewRepository(filepath.Join(root, tt.path, ".skills-seed"), "en-US")
		require.NoError(t, err)
		require.Equal(t, tt.name, repo.GetSkillsConfig().Name)
		require.Equal(t, tt.output, config.EffectiveSkillsPath(repo.GetSkillsConfig().Target, repo.GetSkillsConfig()))
	}
}

func TestSkillsNameFlagSelectsNonInteractiveInit(t *testing.T) {
	cmd := Cmd()
	require.NoError(t, cmd.Flags().Set("skills-name", "team-guide"))
	require.True(t, hasAnyChangedInitFlag(cmd))
	require.False(t, shouldRunInteractiveInit(cmd, commandOptions{}))
}
