package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoryLoadAndReplaceSnapshots(t *testing.T) {
	seedPath := t.TempDir()
	repo := NewRepository(seedPath)

	require.NoError(t, repo.Replace(map[string]string{
		"main.go":                 "package main\n",
		"internal/service/app.go": "package service\n",
	}))

	loaded, err := repo.Load()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"main.go":                 "package main\n",
		"internal/service/app.go": "package service\n",
	}, loaded)

	require.NoError(t, repo.Replace(map[string]string{
		"main.go": "package changed\n",
	}))

	loaded, err = repo.Load()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"main.go": "package changed\n",
	}, loaded)
	require.NoFileExists(t, filepath.Join(seedPath, "cache", "snapshots", "internal", "service", "app.go"))
}

func TestRepositoryLoadMissingDirectoryReturnsEmptyMap(t *testing.T) {
	repo := NewRepository(t.TempDir())

	loaded, err := repo.Load()

	require.NoError(t, err)
	require.Empty(t, loaded)
}

func TestRepositoryRejectsUnsafeSnapshotPaths(t *testing.T) {
	repo := NewRepository(t.TempDir())

	err := repo.Replace(map[string]string{
		"../outside.go": "package outside\n",
	})

	require.Error(t, err)
}

func TestRepositoryWritesSnapshotsUnderMemorySnapshots(t *testing.T) {
	seedPath := t.TempDir()
	repo := NewRepository(seedPath)

	require.NoError(t, repo.Replace(map[string]string{
		"main.go": "package main\n",
	}))

	content, err := os.ReadFile(filepath.Join(seedPath, "cache", "snapshots", "main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(content))
}
