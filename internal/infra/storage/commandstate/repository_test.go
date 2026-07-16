package commandstate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoryRejectsUnsupportedSchemaOnLoad(t *testing.T) {
	repo := NewRepository(t.TempDir(), "learn-current")
	data, err := json.Marshal(&State{SchemaVersion: schemaVersion + 1})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(repo.Path()), 0o755))
	require.NoError(t, os.WriteFile(repo.Path(), data, 0o600))

	_, err = repo.Load(context.Background())

	require.ErrorIs(t, err, ErrUnsupportedSchemaVersion)
}

func TestRepositoryRejectsUnsupportedSchemaOnSave(t *testing.T) {
	repo := NewRepository(t.TempDir(), "learn-current")

	err := repo.Save(context.Background(), &State{SchemaVersion: schemaVersion + 1})

	require.ErrorIs(t, err, ErrUnsupportedSchemaVersion)
	require.NoFileExists(t, repo.Path())
}

func TestRepositoryRejectsNilState(t *testing.T) {
	repo := NewRepository(t.TempDir(), "learn-current")

	err := repo.Save(context.Background(), nil)

	require.EqualError(t, err, "command state is nil")
}
