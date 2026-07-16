package cliskills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/stretchr/testify/require"
)

func TestInstallGlobalAtHomeWritesClaudeAndCodexSkillTrees(t *testing.T) {
	homeDir := t.TempDir()

	results, err := InstallGlobalAtHome(homeDir, TargetAll)
	require.NoError(t, err)
	require.Len(t, results, 2)

	claudeRoot := filepath.Join(homeDir, ".claude", "skills", skillName)
	codexRoot := filepath.Join(homeDir, ".codex", "skills", skillName)
	require.FileExists(t, filepath.Join(claudeRoot, "SKILL.md"))
	require.FileExists(t, filepath.Join(codexRoot, "SKILL.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "operation-model.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "init-reset.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "workspace.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "sync.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "learn.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "generate.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "patterns.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "workflow.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "preview.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "check-hook.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "review-profile.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "log-help-version.md"))
	require.FileExists(t, filepath.Join(codexRoot, "references", "cli-skills.md"))

	content, err := os.ReadFile(filepath.Join(codexRoot, "SKILL.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "name: skills-seed-cli")
	require.Contains(t, string(content), "<!-- "+versionKey+": "+metadata.ProgramVersion+" -->")
	require.Contains(t, string(content), "<!-- "+promptHashKey+": ")
	require.Contains(t, string(content), "<!-- "+skillsHashKey+": ")
	require.Contains(t, string(content), "./references/operation-model.md")
}

func TestCommandHasInstallAndUninstallTargetFlags(t *testing.T) {
	cmd := Cmd()

	install, _, err := cmd.Find([]string{"install"})
	require.NoError(t, err)
	require.NotNil(t, install)
	installFlag := install.Flags().Lookup("target")
	require.NotNil(t, installFlag)
	require.Equal(t, "t", installFlag.Shorthand)
	require.Equal(t, TargetAuto, installFlag.DefValue)

	uninstall, _, err := cmd.Find([]string{"uninstall"})
	require.NoError(t, err)
	require.NotNil(t, uninstall)
	uninstallFlag := uninstall.Flags().Lookup("target")
	require.NotNil(t, uninstallFlag)
	require.Equal(t, "t", uninstallFlag.Shorthand)
	require.Equal(t, TargetAuto, uninstallFlag.DefValue)
}

func TestInstallGlobalAtHomeSupportsSingleTarget(t *testing.T) {
	homeDir := t.TempDir()

	results, err := InstallGlobalAtHome(homeDir, TargetClaude)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, filepath.Join(homeDir, ".claude", "skills", skillName), results[0])
	require.FileExists(t, filepath.Join(homeDir, ".claude", "skills", skillName, "SKILL.md"))
	require.NoFileExists(t, filepath.Join(homeDir, ".codex", "skills", skillName, "SKILL.md"))
}

func TestInstallGlobalAtHomeAutoUsesDetectedTargets(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	homeDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755))

	results, err := InstallGlobalAtHome(homeDir, TargetAuto)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, filepath.Join(homeDir, ".claude", "skills", skillName), results[0])
	require.FileExists(t, filepath.Join(homeDir, ".claude", "skills", skillName, "SKILL.md"))
	require.NoFileExists(t, filepath.Join(homeDir, ".codex", "skills", skillName, "SKILL.md"))
}

func TestInstallGlobalAtHomeAutoRequiresDetectedTarget(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	homeDir := t.TempDir()

	_, err := InstallGlobalAtHome(homeDir, TargetAuto)
	require.Error(t, err)

	inspection, err := InspectGlobalAtHome(homeDir, TargetAuto)
	require.NoError(t, err)
	require.Empty(t, inspection)
}

func TestNormalizeTargetSupportsAutoAndRejectsUnsupportedTarget(t *testing.T) {
	got, err := NormalizeTarget("")
	require.NoError(t, err)
	require.Equal(t, TargetAuto, got)

	_, err = NormalizeTarget("cursor")
	require.Error(t, err)
}

func TestRenderSkillDocumentsPublicCommands(t *testing.T) {
	files, err := renderSkillTree(TargetCodex, metadataValues{Version: "v1.0.0", PromptHash: "hash-a", SkillsHash: "hash-b"})
	require.NoError(t, err)
	paths := make(map[string]string, len(files))
	for _, file := range files {
		paths[file.Path] = file.Content
	}
	content := paths["SKILL.md"]
	require.NotEmpty(t, content)

	for _, command := range []string{
		"skills-seed --version",
		"skills-seed help [command]",
		"skills-seed cli-skills install",
		"skills-seed cli-skills uninstall",
		"skills-seed init",
		"skills-seed reset",
		"skills-seed workspace add",
		"skills-seed sync",
		"skills-seed learn current",
		"skills-seed learn history",
		"skills-seed generate skills",
		"skills-seed patterns add",
		"skills-seed patterns update",
		"skills-seed patterns delete",
		"skills-seed patterns show",
		"skills-seed patterns stats",
		"skills-seed patterns compact",
		"skills-seed workflow",
		"skills-seed workflow show",
		"skills-seed preview files",
		"skills-seed check",
		"skills-seed review import",
		"skills-seed review stats",
		"skills-seed profile show",
		"skills-seed profile refresh",
		"skills-seed hook install",
		"skills-seed hook uninstall",
		"skills-seed hook run",
		"skills-seed log",
	} {
		require.Contains(t, content, command)
	}
	require.Contains(t, content, "./references/workflow.md")
	require.Contains(t, content, "Use when operating, explaining, debugging, or automating the skills-seed CLI")
}

func TestRenderSkillTreeIncludesPerCommandReferences(t *testing.T) {
	files, err := renderSkillTree(TargetCodex, metadataValues{Version: "v1.0.0", PromptHash: "hash-a", SkillsHash: "hash-b"})
	require.NoError(t, err)

	paths := make(map[string]string)
	for _, file := range files {
		paths[file.Path] = file.Content
	}
	require.Contains(t, paths, "SKILL.md")
	for _, path := range []string{
		"references/operation-model.md",
		"references/init-reset.md",
		"references/workspace.md",
		"references/sync.md",
		"references/learn.md",
		"references/generate.md",
		"references/patterns.md",
		"references/workflow.md",
		"references/preview.md",
		"references/check-hook.md",
		"references/review-profile.md",
		"references/log-help-version.md",
		"references/cli-skills.md",
	} {
		require.Contains(t, paths, path)
	}
	require.Contains(t, paths["references/operation-model.md"], "Command Groups")
	require.Contains(t, paths["references/workflow.md"], "Merge Versus Overwrite")
	require.Contains(t, paths["references/workflow.md"], "Use the detail form when complete workflow content is needed")
}

func TestWriteTreeIfNeededKeepsSameContent(t *testing.T) {
	root := t.TempDir()
	meta := metadataValues{Version: "v1.0.0", PromptHash: "hash-a", SkillsHash: "hash-b"}
	content := skillContentWithMetadata(meta)
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0644))

	err := writeTreeIfNeeded(root, []skillFile{{Path: "SKILL.md", Content: content}})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}

func TestWriteTreeIfNeededUpdatesContentAndAddsReferences(t *testing.T) {
	root := t.TempDir()
	oldMeta := metadataValues{Version: "v1.0.0", PromptHash: "hash-a", SkillsHash: "hash-b"}
	newMeta := metadataValues{Version: "v1.0.1", PromptHash: "hash-a", SkillsHash: "hash-c"}
	newContent := skillContentWithMetadata(newMeta)
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(skillContentWithMetadata(oldMeta)), 0644))

	err := writeTreeIfNeeded(root, []skillFile{
		{Path: "SKILL.md", Content: newContent},
		{Path: "references/operation-model.md", Content: "reference\n"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	require.NoError(t, err)
	require.Equal(t, newContent, string(content))
	require.FileExists(t, filepath.Join(root, "references", "operation-model.md"))
}

func TestWriteTreeIfNeededRemovesRetiredFiles(t *testing.T) {
	root := t.TempDir()
	meta := metadataValues{Version: "v1.0.0", PromptHash: "hash-a", SkillsHash: "hash-b"}
	content := skillContentWithMetadata(meta)
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "references"), 0o755))
	retired := filepath.Join(root, "references", "retired.md")
	require.NoError(t, os.WriteFile(retired, []byte("retired\n"), 0o644))

	err := writeTreeIfNeeded(root, []skillFile{{Path: "SKILL.md", Content: content}})

	require.NoError(t, err)
	require.NoFileExists(t, retired)
}

func TestWriteTreeIfNeededOverwritesExistingTree(t *testing.T) {
	root := t.TempDir()
	existingContent := "---\nname: skills-seed-cli\n---\nexisting\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(existingContent), 0644))

	err := writeTreeIfNeeded(root, []skillFile{
		{Path: "SKILL.md", Content: skillContentWithMetadata(metadataValues{Version: "v1.0.0", PromptHash: "hash-a", SkillsHash: "hash-b"})},
		{Path: "references/operation-model.md", Content: "reference\n"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	require.NoError(t, err)
	require.NotEqual(t, existingContent, string(content))
	require.FileExists(t, filepath.Join(root, "references", "operation-model.md"))
}

func TestWriteTreeIfNeededCreatesNewTree(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills-seed-cli")
	meta := metadataValues{Version: "v1.0.0", PromptHash: "hash-a", SkillsHash: "hash-b"}
	content := skillContentWithMetadata(meta)

	err := writeTreeIfNeeded(root, []skillFile{
		{Path: "SKILL.md", Content: content},
		{Path: "references/operation-model.md", Content: "reference\n"},
	})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	require.NoError(t, err)
	require.Equal(t, content, string(got))
	require.FileExists(t, filepath.Join(root, "references", "operation-model.md"))
}

func TestUninstallGlobalAtHomeRemovesConfiguredTargets(t *testing.T) {
	homeDir := t.TempDir()
	claudeRoot := filepath.Join(homeDir, ".claude", "skills", skillName)
	codexRoot := filepath.Join(homeDir, ".codex", "skills", skillName)
	for _, root := range []string{claudeRoot, codexRoot} {
		require.NoError(t, os.MkdirAll(root, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("existing\n"), 0644))
	}

	paths, err := UninstallGlobalAtHome(homeDir, TargetAll)
	require.NoError(t, err)
	require.Equal(t, []string{claudeRoot, codexRoot}, paths)
	require.NoDirExists(t, claudeRoot)
	require.NoDirExists(t, codexRoot)
}

func TestInspectGlobalAtHomeDetectsCurrentMissingAndOutdated(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	homeDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".claude", "skills", skillName), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".codex", "skills", skillName), 0755))

	current := currentMetadata()
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".claude", "skills", skillName, "SKILL.md"), []byte(skillContentWithMetadata(current)), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".codex", "skills", skillName, "SKILL.md"), []byte(skillContentWithMetadata(metadataValues{Version: "old", PromptHash: current.PromptHash, SkillsHash: current.SkillsHash})), 0644))

	results, err := InspectGlobalAtHome(homeDir, TargetAll)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, InstallCurrent, results[0])
	require.Equal(t, InstallOutdated, results[1])
	require.True(t, NeedsInstallOrUpdate(results))

	require.NoError(t, os.Remove(filepath.Join(homeDir, ".codex", "skills", skillName, "SKILL.md")))
	results, err = InspectGlobalAtHome(homeDir, TargetAll)
	require.NoError(t, err)
	require.Equal(t, InstallMissing, results[1])

	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".codex", "skills", skillName, "SKILL.md"), []byte("unmarked\n"), 0644))
	results, err = InspectGlobalAtHome(homeDir, TargetAll)
	require.NoError(t, err)
	require.Equal(t, InstallOutdated, results[1])
}

func TestInspectGlobalAtHomeReturnsSkillReadError(t *testing.T) {
	homeDir := t.TempDir()
	skillPath := filepath.Join(homeDir, ".codex", "skills", skillName, "SKILL.md")
	require.NoError(t, os.MkdirAll(skillPath, 0o755))

	_, err := InspectGlobalAtHome(homeDir, TargetCodex)
	require.Error(t, err)
}

func skillContentWithMetadata(meta metadataValues) string {
	return "<!-- generated-by: skills-seed -->\n" +
		"<!-- " + versionKey + ": " + meta.Version + " -->\n" +
		"<!-- " + promptHashKey + ": " + meta.PromptHash + " -->\n" +
		"<!-- " + skillsHashKey + ": " + meta.SkillsHash + " -->\n"
}
