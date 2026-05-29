package filefilter

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
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
		require.True(t, MatchExcluded(path, patterns), path)
	}

	require.False(t, MatchExcluded("internal/service/user.go", patterns))
}

func TestFilterFilesRemovesExcludedPaths(t *testing.T) {
	files := []domain.FileInfo{
		domain.NewFileInfo("internal/service/user.go", "package service"),
		domain.NewFileInfo("internal/service/mocks/repo.go", "package mocks"),
		domain.NewFileInfo("api/user.pb.go", "package api"),
	}

	filtered := FilterFiles(files, []string{"**/mocks/**", "**/*.pb.go"})

	require.Len(t, filtered, 1)
	require.Equal(t, "internal/service/user.go", filtered[0].Path)
}

func TestMatchExcludedMatchesBasenameForPatternsWithoutSlash(t *testing.T) {
	require.True(t, MatchExcluded("logs/app.log", []string{"*.log"}))
	require.True(t, MatchExcluded("tmp/archive.tar.gz", []string{"*.tar.gz"}))
	require.False(t, MatchExcluded("logs/app.txt", []string{"*.log"}))
}
