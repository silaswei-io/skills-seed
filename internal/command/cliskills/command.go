package cliskills

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/service/skilloutput"
	skilltemplates "github.com/silaswei-io/skills-seed/internal/templates/skills"
	"github.com/spf13/cobra"
)

var errSkillTreeMismatch = errors.New("skill tree does not match rendered files")

const (
	TargetAuto   = "auto"
	TargetAll    = "all"
	TargetClaude = "claude"
	TargetCodex  = "codex"
)

const (
	// skillName 是全局 skills-seed CLI Skill 名称。
	skillName = "skills-seed-cli"
	// versionKey 记录生成该 CLI Skill 的 skills-seed 版本。
	versionKey = "skills-seed-version"
	// promptHashKey 记录生成该 CLI Skill 时的提示词模板树指纹。
	promptHashKey = "prompt-templates-sha256"
	// skillsHashKey 记录生成该 CLI Skill 时的 Skills 模板树指纹。
	skillsHashKey = "skills-templates-sha256"
)

type metadataValues struct {
	Version    string
	PromptHash string
	SkillsHash string
}

type targetPath struct {
	Provider string
	Path     string
}

type skillFile struct {
	Path    string
	Content string
}

type InstallState string

const (
	InstallCurrent  InstallState = "current"
	InstallMissing  InstallState = "missing"
	InstallOutdated InstallState = "outdated"
)

// Cmd 返回全局 CLI Skills 管理命令。
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cli-skills",
		Short:   i18n.Get("CLISkillsShort"),
		Long:    i18n.Get("CLISkillsLongDesc"),
		Example: i18n.Get("CLISkillsExample"),
	}
	cmd.AddCommand(installCmd())
	cmd.AddCommand(uninstallCmd())
	return cmd
}

func installCmd() *cobra.Command {
	target := TargetAuto
	cmd := &cobra.Command{
		Use:     "install",
		Short:   i18n.Get("CLISkillsInstallShort"),
		Long:    i18n.Get("CLISkillsInstallLongDesc"),
		Example: i18n.Get("CLISkillsInstallExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := InstallGlobal(target)
			if err != nil {
				return err
			}
			return PrintInstallResults(cmd.OutOrStdout(), results)
		},
	}
	cmd.Flags().StringVarP(&target, "target", "t", TargetAuto, i18n.Get("CLISkillsInstallFlagTarget"))
	return cmd
}

func uninstallCmd() *cobra.Command {
	target := TargetAuto
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   i18n.Get("CLISkillsUninstallShort"),
		Long:    i18n.Get("CLISkillsUninstallLongDesc"),
		Example: i18n.Get("CLISkillsUninstallExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := UninstallGlobal(target)
			if err != nil {
				return err
			}
			return PrintUninstallResults(cmd.OutOrStdout(), results)
		},
	}
	cmd.Flags().StringVarP(&target, "target", "t", TargetAuto, i18n.Get("CLISkillsUninstallFlagTarget"))
	return cmd
}

func InstallGlobal(target string) ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return InstallGlobalAtHome(homeDir, target)
}

func InstallGlobalAtHome(homeDir string, target string) ([]string, error) {
	targets, err := resolveGlobalTargets(homeDir, target)
	if err != nil {
		return nil, err
	}
	meta := currentMetadata()
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		files, err := renderSkillTree(target.Provider, meta)
		if err != nil {
			return nil, err
		}
		if err := writeTreeIfNeeded(target.Path, files); err != nil {
			return nil, err
		}
		paths = append(paths, target.Path)
	}
	return paths, nil
}

func UninstallGlobal(target string) ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return UninstallGlobalAtHome(homeDir, target)
}

func UninstallGlobalAtHome(homeDir string, target string) ([]string, error) {
	targets, err := resolveGlobalTargets(homeDir, target)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		if err := skilloutput.Remove(target.Path); err != nil {
			return nil, err
		}
		paths = append(paths, target.Path)
	}
	return paths, nil
}

func InspectGlobal(target string) ([]InstallState, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return InspectGlobalAtHome(homeDir, target)
}

func InspectGlobalAtHome(homeDir string, target string) ([]InstallState, error) {
	normalizedTarget, err := NormalizeTarget(target)
	if err != nil {
		return nil, err
	}
	meta := currentMetadata()
	targets := globalTargets(homeDir, normalizedTarget)
	states := make([]InstallState, 0, len(targets))
	for _, target := range targets {
		state, err := inspectInstallState(target.Path, meta)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func NeedsInstallOrUpdate(states []InstallState) bool {
	for _, state := range states {
		if state == InstallMissing || state == InstallOutdated {
			return true
		}
	}
	return false
}

func PrintInstallResults(out io.Writer, paths []string) error {
	return printPaths(out, "CLISkillsInstallReady", paths)
}

func PrintUninstallResults(out io.Writer, paths []string) error {
	return printPaths(out, "CLISkillsUninstallReady", paths)
}

func printPaths(out io.Writer, messageKey string, paths []string) error {
	_, err := fmt.Fprintln(out, i18n.GetWithParams(messageKey, map[string]interface{}{"Paths": strings.Join(paths, ", ")}))
	return err
}

func NormalizeTarget(target string) (string, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = TargetAuto
	}
	switch target {
	case TargetAuto, TargetAll, TargetClaude, TargetCodex:
		return target, nil
	default:
		return "", fmt.Errorf("%s", i18n.GetWithParams("CLISkillsUnsupportedTarget", map[string]interface{}{"Target": target}))
	}
}

func resolveGlobalTargets(homeDir string, target string) ([]targetPath, error) {
	normalizedTarget, err := NormalizeTarget(target)
	if err != nil {
		return nil, err
	}
	targets := globalTargets(homeDir, normalizedTarget)
	if normalizedTarget != TargetAuto || len(targets) > 0 {
		return targets, nil
	}
	return nil, fmt.Errorf("%s", i18n.Get("CLISkillsNoAvailableTarget"))
}

func globalTargets(homeDir string, target string) []targetPath {
	targets := []targetPath{
		{
			Provider: TargetClaude,
			Path:     filepath.Join(homeDir, ".claude", "skills", skillName),
		},
		{
			Provider: TargetCodex,
			Path:     filepath.Join(homeDir, ".codex", "skills", skillName),
		},
	}
	if target == TargetAll {
		return targets
	}
	if target == TargetAuto {
		filtered := make([]targetPath, 0, len(targets))
		for _, candidate := range targets {
			if isTargetAvailable(homeDir, candidate) {
				filtered = append(filtered, candidate)
			}
		}
		return filtered
	}
	filtered := targets[:0]
	for _, candidate := range targets {
		if candidate.Provider == target {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func isTargetAvailable(homeDir string, target targetPath) bool {
	switch target.Provider {
	case TargetClaude:
		return pathExists(filepath.Join(homeDir, ".claude")) || commandExists("claude")
	case TargetCodex:
		return pathExists(filepath.Join(homeDir, ".codex")) || commandExists("codex")
	default:
		return false
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func currentMetadata() metadataValues {
	return metadataValues{
		Version:    metadata.ProgramVersion,
		PromptHash: metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS)),
		SkillsHash: metadata.HashOrUnavailable(metadata.SkillsTemplatesHash(embedfs.FS)),
	}
}

func writeTreeIfNeeded(rootPath string, files []skillFile) error {
	if len(files) == 0 {
		return nil
	}

	matches, err := skillTreeMatches(rootPath, files)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}
	return skilloutput.Replace(rootPath, func(staging string) error {
		for _, file := range files {
			if err := writeFile(filepath.Join(staging, file.Path), file.Content); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func skillTreeMatches(rootPath string, files []skillFile) (bool, error) {
	expected := make(map[string]string, len(files))
	for _, file := range files {
		expected[filepath.Clean(file.Path)] = file.Content
	}
	seen := make(map[string]struct{}, len(files))
	err := filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		want, ok := expected[filepath.Clean(relative)]
		if !ok {
			return errSkillTreeMismatch
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(content) != want {
			return errSkillTreeMismatch
		}
		seen[filepath.Clean(relative)] = struct{}{}
		return nil
	})
	if os.IsNotExist(err) || err == errSkillTreeMismatch {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(seen) == len(expected), nil
}

func inspectInstallState(rootPath string, expected metadataValues) (InstallState, error) {
	content, err := os.ReadFile(filepath.Join(rootPath, "SKILL.md"))
	if os.IsNotExist(err) {
		return InstallMissing, nil
	}
	if err != nil {
		return "", err
	}
	current := parseMetadata(string(content))
	if current.Version == expected.Version && current.PromptHash == expected.PromptHash && current.SkillsHash == expected.SkillsHash {
		return InstallCurrent, nil
	}
	return InstallOutdated, nil
}

func parseMetadata(content string) metadataValues {
	var meta metadataValues
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := parseHTMLMetadataComment(line)
		if !ok {
			continue
		}
		switch key {
		case versionKey:
			meta.Version = value
		case promptHashKey:
			meta.PromptHash = value
		case skillsHashKey:
			meta.SkillsHash = value
		}
	}
	return meta
}

func parseHTMLMetadataComment(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "<!--") || !strings.HasSuffix(line, "-->") {
		return "", "", false
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "<!--"), "-->"))
	key, value, ok := strings.Cut(body, ":")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), true
}

func renderSkillTree(provider string, meta metadataValues) ([]skillFile, error) {
	loader := skilltemplates.NewLoaderForAgent(provider, config.DefaultSkillsLocale)
	relativeTemplates := []struct {
		Template string
		Output   string
	}{
		{Template: "cli/skill", Output: "SKILL.md"},
		{Template: "cli/references/operation-model", Output: "references/operation-model.md"},
		{Template: "cli/references/init-reset", Output: "references/init-reset.md"},
		{Template: "cli/references/workspace", Output: "references/workspace.md"},
		{Template: "cli/references/sync", Output: "references/sync.md"},
		{Template: "cli/references/learn", Output: "references/learn.md"},
		{Template: "cli/references/generate", Output: "references/generate.md"},
		{Template: "cli/references/patterns", Output: "references/patterns.md"},
		{Template: "cli/references/workflow", Output: "references/workflow.md"},
		{Template: "cli/references/preview", Output: "references/preview.md"},
		{Template: "cli/references/check-hook", Output: "references/check-hook.md"},
		{Template: "cli/references/review-profile", Output: "references/review-profile.md"},
		{Template: "cli/references/log-help-version", Output: "references/log-help-version.md"},
		{Template: "cli/references/cli-skills", Output: "references/cli-skills.md"},
	}
	files := make([]skillFile, 0, len(relativeTemplates))
	for _, item := range relativeTemplates {
		content, err := loader.RenderRelative(item.Template, meta)
		if err != nil {
			return nil, err
		}
		files = append(files, skillFile{
			Path:    item.Output,
			Content: content,
		})
	}
	return files, nil
}
