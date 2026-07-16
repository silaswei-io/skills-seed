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
		".skills-seed/context/domain.md",
		"internal/service/service.go",
		".agents/skills/generated/AGENTS.md",
		".skills-seed/cache/snapshots/AGENTS.md",
		".skills-seed/context/node_modules/dependency/AGENTS.md",
		".skills-seed/store/Makefile",
		".skills-seed/workflows/release/AGENTS.md",
		".skills-seed/runtime/AGENTS.md",
		"vendor/dependency/AGENTS.md",
		"packages/example/node_modules/dependency/AGENTS.md",
		"nested/project/.skills-seed/context/AGENTS.md",
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
		".skills-seed/context/domain.md",
		"AGENTS.md",
		"Taskfile.yml",
	}, paths)
}
