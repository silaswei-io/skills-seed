package codegraph

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	index, err := Scan(context.Background(), root, []string{"main.go"}, "embedded")
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
	data, err := Query(context.Background(), root, "callers", []string{shared}, "embedded")
	require.NoError(t, err)
	require.Contains(t, string(data), "Entry")
	data, err = Query(context.Background(), root, "query", []string{"Poisoned"}, "embedded")
	require.NoError(t, err)
	require.NotContains(t, string(data), `"name":"Poisoned"`)
	again, err := Scan(context.Background(), root, []string{"main.go"}, "embedded")
	require.NoError(t, err)
	require.Equal(t, index, again)
	write("extra.go", "package example\nfunc Extra() {}\n")
	_, err = Scan(context.Background(), root, []string{"main.go", "extra.go"}, "embedded")
	require.NoError(t, err)
	index, err = Scan(context.Background(), root, []string{"main.go"}, "embedded")
	require.NoError(t, err)
	require.Len(t, index.Files, 1)
	data, err = Query(context.Background(), root, "query", []string{"Extra"}, "embedded")
	require.NoError(t, err)
	require.Contains(t, string(data), `"name":"Extra"`)
	require.NoError(t, os.Remove(filepath.Join(root, "main.go")))
	index, err = Scan(context.Background(), root, []string{}, "embedded")
	require.NoError(t, err)
	require.Empty(t, index.Files)
	require.Empty(t, index.Nodes)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Scan(canceled, root, []string{}, "embedded")
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

func TestRuntimeSelection(t *testing.T) {
	t.Setenv("SKILLS_SEED_CODEGRAPH_HOME", "")
	root := t.TempDir()
	node := "node"
	if runtime.GOOS == "windows" {
		node += ".exe"
	}
	home := filepath.Join(root, "bundle")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "lib", "dist", "db"), 0755))
	for _, name := range []string{node, "lib/dist/index.js", "lib/dist/db/migrations.js"} {
		require.NoError(t, os.WriteFile(filepath.Join(home, filepath.FromSlash(name)), []byte("stub"), 0700))
	}
	bin := filepath.Join(home, "bin")
	require.NoError(t, os.MkdirAll(bin, 0755))
	commandName := "codegraph"
	if runtime.GOOS == "windows" {
		commandName += ".exe"
	}
	require.NoError(t, os.WriteFile(filepath.Join(bin, commandName), []byte("stub"), 0700))
	t.Setenv("PATH", bin)
	selected, err := runtimeHome(context.Background(), "auto")
	require.NoError(t, err)
	resolvedHome, err := filepath.EvalSymlinks(home)
	require.NoError(t, err)
	require.Equal(t, resolvedHome, selected)
	selected, err = runtimeHome(context.Background(), "external")
	require.NoError(t, err)
	require.Equal(t, resolvedHome, selected)
	if _, err := runtimeFiles.ReadFile("assets/runtime.zip"); err == nil {
		selected, err = runtimeHome(context.Background(), "embedded")
		require.NoError(t, err)
		require.NotEqual(t, resolvedHome, selected)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		platform = "win32-" + runtime.GOARCH
	}
	if runtime.GOARCH == "amd64" {
		platform = platform[:len(platform)-len("amd64")] + "x64"
	}
	npm := filepath.Join(root, "npm", "@colbymchenry", "codegraph")
	npmHome := filepath.Join(npm, "node_modules", "@colbymchenry", "codegraph-"+platform)
	require.NoError(t, os.MkdirAll(filepath.Join(npmHome, "lib", "dist", "db"), 0755))
	for _, name := range []string{node, "lib/dist/index.js", "lib/dist/db/migrations.js"} {
		require.NoError(t, os.WriteFile(filepath.Join(npmHome, filepath.FromSlash(name)), []byte("stub"), 0700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(npm, commandName), []byte("stub"), 0700))
	t.Setenv("PATH", npm)
	selected, err = runtimeHome(context.Background(), "auto")
	require.NoError(t, err)
	resolvedNPMHome, err := filepath.EvalSymlinks(npmHome)
	require.NoError(t, err)
	require.Equal(t, resolvedNPMHome, selected)
	t.Setenv("PATH", root)
	_, err = runtimeHome(context.Background(), "external")
	require.ErrorContains(t, err, "external CodeGraph unavailable")
}

func TestScanRejectsIncompatibleIndexWithoutMigration(t *testing.T) {
	if _, err := runtimeFiles.ReadFile("assets/runtime.zip"); err != nil && os.Getenv("SKILLS_SEED_CODEGRAPH_HOME") == "" {
		t.Skip("run make bundle for the official SDK integration test")
	}
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package example\nfunc Entry() {}\n"), 0600))
	_, err := Scan(context.Background(), root, []string{"main.go"}, "embedded")
	require.NoError(t, err)
	home, err := runtimeHome(context.Background(), "embedded")
	require.NoError(t, err)
	node := "node"
	if runtime.GOOS == "windows" {
		node += ".exe"
	}
	command := exec.Command(filepath.Join(home, node), "--disable-warning=ExperimentalWarning", "-e", `const {DatabaseSync}=require('node:sqlite');const db=new DatabaseSync(process.argv[1]);db.prepare("INSERT INTO schema_versions(version,applied_at,description) VALUES (999,0,'test')").run();db.close();`, filepath.Join(root, IndexDir, "codegraph.db"))
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	_, err = Scan(context.Background(), root, []string{"main.go"}, "embedded")
	require.ErrorContains(t, err, "index schema 999 differs")
}
