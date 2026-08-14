package repositoryscope

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScopeClassifiesKnowledgeBoundaries(t *testing.T) {
	scope := New([]string{".*", "vendor/**", "node_modules/**", "build/**", "private/**"})

	tests := []struct {
		path  string
		class PathClass
	}{
		{path: "internal/service/app.go", class: PathClassProject},
		{path: ".skills-seed/cache/state.json", class: PathClassExcluded},
		{path: "vendor/example/module.go", class: PathClassExcluded},
		{path: ".test/quality/cache/go-mod/dependency/go.mod", class: PathClassExcluded},
		{path: "build/generated.go", class: PathClassExcluded},
		{path: "private/config.go", class: PathClassExcluded},
	}

	for _, tt := range tests {
		require.Equal(t, tt.class, scope.Classify(tt.path), tt.path)
		require.Equal(t, tt.class == PathClassProject, scope.AllowsKnowledge(tt.path), tt.path)
	}
}

func TestMatchExcludedSupportsConfiguredPatterns(t *testing.T) {
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
	require.True(t, matchExcluded("logs/app.log", []string{"*.log"}))
	require.True(t, matchExcluded("tmp/archive.tar.gz", []string{"*.tar.gz"}))
	require.False(t, matchExcluded("logs/app.txt", []string{"*.log"}))
}
