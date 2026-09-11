package jsonfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type document struct {
	Name string `json:"name"`
}

func TestStoreSaveAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "document.json")
	store := Store[document]{Path: path}

	require.NoError(t, store.Save(context.Background(), &document{Name: "demo"}))
	value, err := store.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "demo", value.Name)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "{\n  \"name\": \"demo\"\n}\n", string(data))
}

func TestStoreHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := Store[document]{Path: filepath.Join(t.TempDir(), "document.json")}

	_, err := store.Get(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, store.Save(ctx, &document{}), context.Canceled)
}

func TestStoreGetReportsMissingReadAndParseErrors(t *testing.T) {
	wantMissing := errors.New("missing")
	store := Store[document]{
		Path:     filepath.Join(t.TempDir(), "missing.json"),
		NotFound: wantMissing,
		Labels:   Labels{Read: "read document", Parse: "parse document"},
	}

	_, err := store.Get(context.Background())
	require.ErrorIs(t, err, wantMissing)

	dirPath := t.TempDir()
	store.Path = dirPath
	_, err = store.Get(context.Background())
	require.ErrorContains(t, err, "read document")

	store.Path = filepath.Join(t.TempDir(), "invalid.json")
	require.NoError(t, os.WriteFile(store.Path, []byte("{"), 0o644))
	_, err = store.Get(context.Background())
	require.ErrorContains(t, err, "parse document")
}

func TestStoreSaveReportsNilMarshalDirectoryAndWriteErrors(t *testing.T) {
	wantNil := errors.New("nil document")
	store := Store[document]{Path: filepath.Join(t.TempDir(), "document.json"), NilValue: wantNil}
	require.ErrorIs(t, store.Save(context.Background(), nil), wantNil)
	require.ErrorContains(t, (Store[document]{Path: store.Path}).Save(context.Background(), nil), "json value is nil")

	marshalStore := Store[chan int]{Path: filepath.Join(t.TempDir(), "channel.json"), Labels: Labels{Marshal: "marshal document"}}
	value := make(chan int)
	require.ErrorContains(t, marshalStore.Save(context.Background(), &value), "marshal document")

	parentFile := filepath.Join(t.TempDir(), "parent")
	require.NoError(t, os.WriteFile(parentFile, []byte("file"), 0o644))
	directoryStore := Store[document]{Path: filepath.Join(parentFile, "document.json"), Labels: Labels{CreateDir: "create directory"}}
	require.ErrorContains(t, directoryStore.Save(context.Background(), &document{}), "create directory")

	writeStore := Store[document]{Path: t.TempDir(), Labels: Labels{Write: "write document"}}
	require.ErrorContains(t, writeStore.Save(context.Background(), &document{}), "write document")
}

func TestWrapLabel(t *testing.T) {
	wantErr := errors.New("failure")
	require.ErrorIs(t, wrapLabel("", wantErr), wantErr)
	err := wrapLabel("operation", wantErr)
	require.ErrorIs(t, err, wantErr)
	require.ErrorContains(t, err, "operation")
}
