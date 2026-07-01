package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	textutil "github.com/silaswei-io/skills-seed/internal/utils/text"
)

var projectPromptNames = []string{
	"analysis-plan",
	"pattern-learn-current",
	"pattern-learn-current-batch",
	"project-profile",
	"learn-analyze",
	"learn-batch",
	"fix-generate",
	"pattern-curate",
}

var deprecatedProjectInstructionNames = []string{
	"skill-project-summary",
	"skill-project-init",
	"project-analyze",
}

var workspacePromptNames = []string{
	"skill-workspace-profile",
	"skill-workspace-spec",
}

// ProjectPromptData 渲染项目提示词模板的数据
type ProjectPromptData struct {
	ProgramVersion      string
	PromptTemplatesHash string
	PromptName          string
	ProjectName         string
	Language            string
	ProjectRoot         string
	Structure           string
	MainFiles           []string
	Locale              string
	SkillsLocale        string
}

// WorkspacePromptData 渲染工作区提示词模板的数据
type WorkspacePromptData struct {
	ProgramVersion      string
	PromptTemplatesHash string
	WorkspaceName       string
	WorkspaceRoot       string
	Projects            []WorkspacePromptProject
	Locale              string
	SkillsLocale        string
}

// WorkspacePromptProject 是工作区提示词中展示的子项目摘要
type WorkspacePromptProject struct {
	ID       string
	Path     string
	Type     string
	Language string
}

// EnsureProjectPrompts 初始化项目级提示词文件
func EnsureProjectPrompts(seedPath string, data ProjectPromptData) error {
	data = normalizeProjectPromptData(data)
	if data.ProgramVersion == "" {
		data.ProgramVersion = metadata.ProgramVersion
	}
	if data.PromptTemplatesHash == "" {
		data.PromptTemplatesHash = metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS))
	}
	if data.SkillsLocale == "" {
		data.SkillsLocale = data.Locale
	}

	baseDirs := []string{
		filepath.Join(seedPath, "prompts"),
		filepath.Join(seedPath, "prompts", "project"),
		filepath.Join(seedPath, "prompts", "instructions"),
	}

	for _, dir := range baseDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("%s: %w", i18n.GetWithParams("PromptCreateDirFailed", map[string]interface{}{"Path": dir}), err)
		}
	}

	profileContent, err := renderProjectTemplate("project-profile", data.SkillsLocale, data)
	if err != nil {
		return err
	}
	profilePath := filepath.Join(seedPath, "prompts", "project", "project-profile.md")
	if err := writeIfNotExists(profilePath, profileContent); err != nil {
		return err
	}

	data.PromptName = "common"
	projectContent, err := renderProjectTemplate("project-prompt", data.SkillsLocale, data)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(seedPath, "prompts", "project", "common.md")
	if err := writeIfNotExists(projectPath, projectContent); err != nil {
		return err
	}

	for _, name := range projectPromptNames {
		data.PromptName = name

		instructionsContent, err := renderProjectTemplate("user-instructions", data.SkillsLocale, data)
		if err != nil {
			return err
		}
		instructionsPath := filepath.Join(seedPath, "prompts", "instructions", name+".md")
		if err := writeIfNotExists(instructionsPath, instructionsContent); err != nil {
			return err
		}
	}
	if err := cleanupDeprecatedProjectInstructions(seedPath); err != nil {
		return err
	}

	return nil
}

// EnsureProjectPromptsAt 初始化指定目录下的项目级提示词文件
func EnsureProjectPromptsAt(basePath string, data ProjectPromptData) error {
	data = normalizeProjectPromptData(data)
	if data.ProgramVersion == "" {
		data.ProgramVersion = metadata.ProgramVersion
	}
	if data.PromptTemplatesHash == "" {
		data.PromptTemplatesHash = metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS))
	}
	if data.SkillsLocale == "" {
		data.SkillsLocale = data.Locale
	}

	baseDirs := []string{
		basePath,
	}
	for _, dir := range baseDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("%s: %w", i18n.GetWithParams("PromptCreateDirFailed", map[string]interface{}{"Path": dir}), err)
		}
	}

	profileContent, err := renderProjectTemplate("project-profile", data.SkillsLocale, data)
	if err != nil {
		return err
	}
	if err := writeIfNotExists(filepath.Join(basePath, "project-profile.md"), profileContent); err != nil {
		return err
	}

	data.PromptName = "common"
	projectContent, err := renderProjectTemplate("project-prompt", data.SkillsLocale, data)
	if err != nil {
		return err
	}
	return writeIfNotExists(filepath.Join(basePath, "common.md"), projectContent)
}

// EnsureWorkspacePrompts 初始化工作区级提示词文件
func EnsureWorkspacePrompts(seedPath string, data WorkspacePromptData) error {
	if data.ProgramVersion == "" {
		data.ProgramVersion = metadata.ProgramVersion
	}
	if data.PromptTemplatesHash == "" {
		data.PromptTemplatesHash = metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS))
	}
	if data.SkillsLocale == "" {
		data.SkillsLocale = data.Locale
	}

	workspaceDir := filepath.Join(seedPath, "prompts", "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.GetWithParams("PromptCreateDirFailed", map[string]interface{}{"Path": workspaceDir}), err)
	}

	for _, name := range workspacePromptNames {
		content, err := renderWorkspaceTemplate(name, data.SkillsLocale, data)
		if err != nil {
			return err
		}
		if err := writeIfNotExists(filepath.Join(workspaceDir, name+".md"), content); err != nil {
			return err
		}
	}
	return nil
}

func renderProjectTemplate(name, locale string, data ProjectPromptData) (string, error) {
	templateData, err := readProjectTemplate(name, locale)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(name).Parse(string(templateData))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func readProjectTemplate(name, locale string) ([]byte, error) {
	if locale == "" {
		locale = config.DefaultToolLocale
	}

	localizedPath := metadata.ProjectPromptTemplatePath(name, locale)
	data, err := embedfs.FS.ReadFile(localizedPath)
	if err == nil {
		return data, nil
	}

	defaultPath := metadata.ProjectPromptTemplatePath(name, "")
	return embedfs.FS.ReadFile(defaultPath)
}

func renderWorkspaceTemplate(name, locale string, data WorkspacePromptData) (string, error) {
	templateData, err := readWorkspaceTemplate(name, locale)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(name).Parse(string(templateData))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func readWorkspaceTemplate(name, locale string) ([]byte, error) {
	if locale == "" {
		locale = config.DefaultToolLocale
	}

	localizedPath := metadata.WorkspacePromptTemplatePath(name, locale)
	data, err := embedfs.FS.ReadFile(localizedPath)
	if err == nil {
		return data, nil
	}

	defaultPath := metadata.WorkspacePromptTemplatePath(name, "")
	return embedfs.FS.ReadFile(defaultPath)
}

func normalizeProjectPromptData(data ProjectPromptData) ProjectPromptData {
	data.Structure = textutil.NormalizeStructureSummary(data.Structure)
	return data
}

func writeIfNotExists(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func cleanupDeprecatedProjectInstructions(seedPath string) error {
	for _, name := range deprecatedProjectInstructionNames {
		path := filepath.Join(seedPath, "prompts", "instructions", name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if hasPromptInstructionBody(string(data)) {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func hasPromptInstructionBody(content string) bool {
	var body strings.Builder
	inComment := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<!--") {
			inComment = true
		}
		if !inComment {
			body.WriteString(trimmed)
		}
		if strings.Contains(trimmed, "-->") {
			inComment = false
		}
	}
	return strings.TrimSpace(body.String()) != ""
}
