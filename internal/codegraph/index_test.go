package codegraph

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSDKIncrementalIndexAndQueries(t *testing.T) {
	if _, err := runtimeFiles.ReadFile("assets/runtime.zip"); err != nil && os.Getenv("SKILLS_SEED_CODEGRAPH_HOME") == "" {
		t.Skip("run make bundle for the official SDK integration test")
	}
	root := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0600))
	}
	write("main.go", "package example\ntype Contract interface { Run() }\ntype Service struct{}\nfunc (Service) Handle() {}\nfunc (Service) helper() {}\nfunc Shared() {}\nfunc Entry() { Shared() }\n")
	write(".skills-seed/cache/ignored/fake.go", "package fake\nfunc Poisoned() {}\n")
	index, err := Scan(context.Background(), root, []string{"main.go"})
	require.NoError(t, err)
	require.Len(t, index.Files, 1)
	require.Empty(t, index.Unindexed)
	var shared, entry string
	for _, node := range index.Nodes {
		require.Equal(t, "main.go", node.Path)
		require.GreaterOrEqual(t, node.EndLine, node.Line)
		if node.Name == "Shared" {
			shared = node.ID
		} else if node.Name == "Entry" {
			entry = node.ID
		} else if node.Name == "Run" {
			require.True(t, node.DeclarationOnly)
		} else if node.Name == "Handle" {
			require.True(t, node.Exported)
		} else if node.Name == "helper" {
			require.False(t, node.Exported)
		}
	}
	require.NotEmpty(t, shared)
	require.Contains(t, index.Calls, Call{Source: entry, Target: shared})
	data, err := Query(context.Background(), root, "callers", []string{shared})
	require.NoError(t, err)
	require.Contains(t, string(data), "Entry")
	data, err = Query(context.Background(), root, "query", []string{"Poisoned"})
	require.NoError(t, err)
	require.NotContains(t, string(data), `"name":"Poisoned"`)
	again, err := Scan(context.Background(), root, []string{"main.go"})
	require.NoError(t, err)
	require.Equal(t, index, again)
	write("extra.go", "package example\nfunc Extra() {}\n")
	_, err = Scan(context.Background(), root, []string{"main.go", "extra.go"})
	require.NoError(t, err)
	index, err = Scan(context.Background(), root, []string{"main.go"})
	require.NoError(t, err)
	require.Len(t, index.Files, 1)
	data, err = Query(context.Background(), root, "query", []string{"Extra"})
	require.NoError(t, err)
	require.Contains(t, string(data), `"name":"Extra"`)
	require.NoError(t, os.Remove(filepath.Join(root, "main.go")))
	index, err = Scan(context.Background(), root, []string{})
	require.NoError(t, err)
	require.Empty(t, index.Files)
	require.Empty(t, index.Nodes)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Scan(canceled, root, []string{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestRuntimeRejectsTraversal(t *testing.T) {
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	file, err := writer.Create("../outside")
	require.NoError(t, err)
	_, err = file.Write([]byte("bad"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.ErrorContains(t, extractRuntime(context.Background(), payload.Bytes(), t.TempDir()), "unsafe")
}
