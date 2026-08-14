package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEngineeringKnowledgePathsCollectsAuthorityFilesIndependently(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"AGENTS.md",
		"Taskfile.yml",
		".github/workflows/verify.yml",
		".skills-seed/rules/foundation/RULE.md",
		"plugins/logger_manager/CLAUDE.md",
		"services/api/AGENTS.md",
		"nested/Taskfile.yml",
		"internal/service/service.go",
		".agents/skills/generated/AGENTS.md",
		".skills-seed/cache/snapshots/AGENTS.md",
		".skills-seed/rules/node_modules/dependency/AGENTS.md",
		".skills-seed/store/Makefile",
		".skills-seed/workflows/release/AGENTS.md",
		".skills-seed/runtime/AGENTS.md",
		"vendor/dependency/AGENTS.md",
		"packages/example/node_modules/dependency/AGENTS.md",
		"nested/project/.skills-seed/rules/foundation/RULE.md",
	}
	for _, path := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(path), 0o644))
	}

	paths, err := engineeringKnowledgePaths(root)

	require.NoError(t, err)
	require.Equal(t, []string{
		".github/workflows/verify.yml",
		".skills-seed/rules/foundation/RULE.md",
		"AGENTS.md",
		"Taskfile.yml",
	}, paths)
}

func TestEngineeringKnowledgeRevisionTracksAuthorityContentAndSources(t *testing.T) {
	root := t.TempDir()

	empty, err := EngineeringKnowledgeRevision(root)
	require.NoError(t, err)
	require.Empty(t, empty)

	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("first"), 0o644))
	first, err := EngineeringKnowledgeRevision(root)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("second"), 0o644))
	second, err := EngineeringKnowledgeRevision(root)
	require.NoError(t, err)
	require.NotEqual(t, first, second)

	require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("another authority"), 0o644))
	third, err := EngineeringKnowledgeRevision(root)
	require.NoError(t, err)
	require.NotEqual(t, second, third)
}

func TestEngineeringKnowledgeRevisionForPathsUsesActualAuthorityInput(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("global"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "custom-rules.md"), []byte("custom"), 0o644))

	automatic, err := EngineeringKnowledgeRevision(root)
	require.NoError(t, err)
	explicit, err := engineeringKnowledgeRevisionForPaths(root, []string{"custom-rules.md"})
	require.NoError(t, err)
	explicitAgain, err := engineeringKnowledgeRevisionForPaths(root, []string{"./custom-rules.md", "custom-rules.md"})
	require.NoError(t, err)

	require.NotEqual(t, automatic, explicit)
	require.Equal(t, explicit, explicitAgain)

	_, err = engineeringKnowledgeRevisionForPaths(root, []string{"../outside.md"})
	require.ErrorContains(t, err, "invalid authority path")
}
