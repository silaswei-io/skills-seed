package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/stretchr/testify/require"
)

func TestResetAllScopeMovesKnowledgeAndPreservesConfiguration(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	seedLayout := layout.New(seedPath)
	writeResetFixture(t, seedPath, seedLayout)

	result, err := Reset(seedPath, AllScope())
	require.NoError(t, err)
	require.NotEmpty(t, result.BackupPath)
	require.ElementsMatch(t, []string{
		"patterns",
		"rules",
		"workflows",
		"project-profile",
		"workspace-profile",
		"workspace-spec",
		"child-project-profiles",
		"file-snapshots",
		"command-checkpoints",
		"learning-history",
	}, result.Resources)

	for _, path := range []string{
		seedLayout.ProjectDB(),
		seedLayout.Rules(),
		seedLayout.Workflows(),
		seedLayout.ProjectProfile(),
		seedLayout.WorkspaceProfile(),
		seedLayout.WorkspaceSpec(),
		seedLayout.StoreDocuments("projects"),
		seedLayout.Snapshots(),
		seedLayout.CommandStates(),
		seedLayout.ChangeLog(),
	} {
		require.NoFileExists(t, path)
	}
	require.FileExists(t, filepath.Join(seedPath, "config.yaml"))
	require.FileExists(t, filepath.Join(seedPath, "context", "team.md"))
	require.FileExists(t, filepath.Join(result.BackupPath, "store", "project.db"))
	require.FileExists(t, filepath.Join(result.BackupPath, "rules", "security", "RULE.md"))
	require.FileExists(t, filepath.Join(result.BackupPath, "workflows", "release", "WORKFLOW.md"))
	require.FileExists(t, filepath.Join(result.BackupPath, "store", "documents", "project-profile.json"))
	require.FileExists(t, filepath.Join(result.BackupPath, "cache", "snapshots", "files.json"))
}

func TestResetSelectedScopeLeavesOtherKnowledgeActive(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	seedLayout := layout.New(seedPath)
	writeResetFixture(t, seedPath, seedLayout)

	result, err := Reset(seedPath, Scope{Patterns: true, Cache: true})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"patterns", "file-snapshots", "command-checkpoints"}, result.Resources)
	require.NoFileExists(t, seedLayout.ProjectDB())
	require.NoDirExists(t, seedLayout.Snapshots())
	require.NoDirExists(t, seedLayout.CommandStates())
	require.FileExists(t, filepath.Join(seedLayout.Rules(), "security", "RULE.md"))
	require.FileExists(t, filepath.Join(seedLayout.Workflows(), "release", "WORKFLOW.md"))
	require.FileExists(t, seedLayout.ProjectProfile())
	require.FileExists(t, seedLayout.ChangeLog())
}

func TestResetRequiresScopeAndInitializedState(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	_, err := Reset(seedPath, Scope{})
	require.ErrorIs(t, err, ErrScopeRequired)

	_, err = Reset(seedPath, Scope{Patterns: true})
	require.ErrorIs(t, err, ErrSeedNotFound)
}

func TestResetAllowsAnAlreadyEmptyScope(t *testing.T) {
	seedPath := filepath.Join(t.TempDir(), ".skills-seed")
	require.NoError(t, os.MkdirAll(seedPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "config.yaml"), []byte("project: {}\n"), 0o644))

	result, err := Reset(seedPath, Scope{Rules: true})
	require.NoError(t, err)
	require.Empty(t, result.Resources)
	require.Empty(t, result.BackupPath)
}

func writeResetFixture(t *testing.T, seedPath string, seedLayout layout.Layout) {
	t.Helper()
	for path, content := range map[string]string{
		filepath.Join(seedPath, "config.yaml"):                                              "project: {}\n",
		filepath.Join(seedPath, "context", "team.md"):                                       "team context\n",
		seedLayout.ProjectDB():                                                              "patterns\n",
		filepath.Join(seedLayout.Rules(), "security", "RULE.md"):                            "rule\n",
		filepath.Join(seedLayout.Workflows(), "release", "WORKFLOW.md"):                     "workflow\n",
		seedLayout.ProjectProfile():                                                         "{}\n",
		seedLayout.WorkspaceProfile():                                                       "{}\n",
		seedLayout.WorkspaceSpec():                                                          "{}\n",
		filepath.Join(seedLayout.StoreDocuments("projects"), "api", "project-profile.json"): "{}\n",
		filepath.Join(seedLayout.Snapshots(), "files.json"):                                 "{}\n",
		filepath.Join(seedLayout.CommandStates(), "sync", "state.json"):                     "{}\n",
		seedLayout.ChangeLog():                                                              "[]\n",
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}
