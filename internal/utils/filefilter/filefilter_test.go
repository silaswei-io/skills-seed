package filefilter

import (
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestMatchExcludedSupportsDefaultPatterns(t *testing.T) {
	patterns := []string{
		"vendor/**",
		"node_modules/**",
		"**/*.pb.go",
		"**/*.gen.go",
		"**/mocks/**",
		"**/testdata/**",
	}

	for _, path := range []string{
		"vendor/mod/file.go",
		"node_modules/pkg/index.ts",
		"api/user.pb.go",
		"internal/api/user.gen.go",
		"internal/service/mocks/repo.go",
		"pkg/testdata/input.json",
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
