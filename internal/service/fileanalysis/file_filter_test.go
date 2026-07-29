package fileanalysis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchExcludedSupportsDefaultPatterns(t *testing.T) {
	patterns := []string{
		".*",
		"vendor/**",
		"node_modules/**",
		"dist/**",
		"coverage/**",
		"*.log",
		"*.tar.gz",
		"*.png",
	}

	for _, path := range []string{
		".env",
		".github/workflows/ci.yml",
		".cursor/rules/codegraph.mdc",
		"pkg/.cache/state.json",
		"vendor/mod/file.go",
		"node_modules/pkg/index.ts",
		"dist/app.js",
		"coverage/index.html",
		"logs/app.log",
		"tmp/archive.tar.gz",
		"assets/logo.png",
	} {
		require.True(t, matchExcluded(path, patterns), path)
	}

	require.False(t, matchExcluded("internal/service/user.go", patterns))
}

func TestMatchExcludedMatchesBasenameForPatternsWithoutSlash(t *testing.T) {
	require.True(t, matchExcluded("logs/app.log", []string{"*.log"}))
	require.True(t, matchExcluded("tmp/archive.tar.gz", []string{"*.tar.gz"}))
	require.False(t, matchExcluded("logs/app.txt", []string{"*.log"}))
}
