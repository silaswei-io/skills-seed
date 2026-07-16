package sourcefiles

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDocumentMatchesCaseInsensitively(t *testing.T) {
	for _, path := range []string{
		"README.MD",
		"docs/Guide.MDX",
		"SECURITY.TXT",
		"Contributing",
		"CHANGELOG.RST",
	} {
		require.True(t, IsDocument(path), path)
	}
}

func TestIsAnalyzableKeepsSourcesUnderDocs(t *testing.T) {
	require.True(t, IsAnalyzable("docs/examples/main.go"))
	require.True(t, IsAnalyzable("docs/src/demo.TSX"))
	require.True(t, IsAnalyzable("examples/README.go"))
	require.False(t, IsAnalyzable("docs/Guide.MD"))
}

func TestIsAnalyzableKeepsDependencyFiles(t *testing.T) {
	require.True(t, IsAnalyzable("go.mod"))
	require.True(t, IsAnalyzable("requirements.txt"))
	require.True(t, IsAnalyzable("docs/package.json"))
	require.False(t, IsAnalyzable("LICENSE"))
}

func TestIsEngineeringKnowledge(t *testing.T) {
	tests := map[string]bool{
		"AGENTS.md":                           true,
		"docs/AGENTS.md":                      true,
		"Taskfile.yml":                        true,
		"Makefile":                            true,
		".github/workflows/verify.yaml":       true,
		".skills-seed/context/release.md":     true,
		"internal/service/service.go":         false,
		".github/workflows/notes.md":          false,
		"docs/build-and-test-instructions.md": false,
	}

	for path, expected := range tests {
		t.Run(path, func(t *testing.T) {
			require.Equal(t, expected, IsEngineeringKnowledge(path))
		})
	}
}
