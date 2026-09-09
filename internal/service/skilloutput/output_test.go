package skilloutput

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteManifestRecordsStableOutputHashes(t *testing.T) {
	root := t.TempDir()
	manifestPath := RuntimeManifestPath(filepath.Join(root, ".skills-seed"), "codex")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "references"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("skill\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "references", "example.md"), []byte("reference\n"), 0o644))

	require.NoError(t, WriteManifest(root, manifestPath, Manifest{
		KnowledgeSnapshotHash: "knowledge-hash",
		ProgramVersion:        "v1.2.3",
		TargetAgent:           "codex",
		OutputPath:            ".agents/skills/demo-dev",
		TemplatesHash:         "templates-hash",
	}))

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest Manifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, 2, manifest.SchemaVersion)
	require.Equal(t, "knowledge-hash", manifest.KnowledgeSnapshotHash)
	require.Equal(t, "codex", manifest.TargetAgent)
	require.Equal(t, ".agents/skills/demo-dev", manifest.OutputPath)
	require.NoFileExists(t, filepath.Join(root, manifestFileName))
	require.Equal(t, []ManifestFile{
		{Path: "SKILL.md", SHA256: "3088e5b60779a95389e4ed08d2ecee6eaac2311c590dab3f2e4beb3090a54f00"},
		{Path: "references/example.md", SHA256: "4151674fad2310eaff3e54db63b6ee84a6c96a68dd46c0f8df5df620d57f899a"},
	}, manifest.OutputFiles)
}

func TestRuntimeManifestPathUsesSafeTargetDirectory(t *testing.T) {
	path := RuntimeManifestPath("/project/.skills-seed", "Custom Target/Agent")

	require.Equal(t, "/project/.skills-seed/runtime/generated-skills/custom-target-agent/manifest.json", filepath.ToSlash(path))
}

func TestWriteManifestRejectsMissingOutput(t *testing.T) {
	root := t.TempDir()

	err := WriteManifest(filepath.Join(root, "missing"), filepath.Join(root, "manifest.json"), Manifest{})

	require.Error(t, err)
}

func TestOutputFilesSkipsNonRegularEntries(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("skill\n"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(root, "SKILL.md"), filepath.Join(root, "skill-link")))

	files, err := outputFiles(root)

	require.NoError(t, err)
	require.Equal(t, []ManifestFile{{Path: "SKILL.md", SHA256: "3088e5b60779a95389e4ed08d2ecee6eaac2311c590dab3f2e4beb3090a54f00"}}, files)
}

func TestReplaceReplacesExistingDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skill")
	require.NoError(t, os.MkdirAll(root, 0o755))
	oldPath := filepath.Join(root, "old.md")
	require.NoError(t, os.WriteFile(oldPath, []byte("old\n"), 0o644))

	require.NoError(t, Replace(root, func(staging string) error {
		return os.WriteFile(filepath.Join(staging, "SKILL.md"), []byte("new\n"), 0o644)
	}))
	require.NoFileExists(t, oldPath)
	require.FileExists(t, filepath.Join(root, "SKILL.md"))
}

func TestReplaceReturnsBuildError(t *testing.T) {
	wantErr := errors.New("build failed")

	err := Replace(filepath.Join(t.TempDir(), "skill"), func(string) error { return wantErr })

	require.ErrorIs(t, err, wantErr)
}

func TestReplaceWithinRootPublishesOutput(t *testing.T) {
	projectRoot := t.TempDir()
	outputPath := filepath.Join(projectRoot, ".agents", "skills", "demo-dev")

	err := ReplaceWithinRoot(projectRoot, outputPath, func(staging string) error {
		return os.WriteFile(filepath.Join(staging, "SKILL.md"), []byte("new\n"), 0o644)
	})

	require.NoError(t, err)
	require.FileExists(t, filepath.Join(outputPath, "SKILL.md"))
}

func TestReplaceWithinRootSupportsUnscopedOutput(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "skill")

	err := ReplaceWithinRoot("", outputPath, func(staging string) error {
		return os.WriteFile(filepath.Join(staging, "SKILL.md"), []byte("new\n"), 0o644)
	})

	require.NoError(t, err)
	require.FileExists(t, filepath.Join(outputPath, "SKILL.md"))
}

func TestReplaceWithinRootRejectsOutputOutsideProject(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	require.NoError(t, os.Mkdir(projectRoot, 0o755))

	err := ReplaceWithinRoot(projectRoot, filepath.Join(root, "outside"), func(string) error { return nil })

	require.Error(t, err)
}

func TestRemoveDeletesConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("existing\n"), 0o644))

	require.NoError(t, Remove(root))
	require.NoDirExists(t, root)
	require.NoError(t, Remove(root))
}
