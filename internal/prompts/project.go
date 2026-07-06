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

var contextFileNames = []string{
	"README",
	"project",
	"rules",
	"glossary",
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

	contextDir := filepath.Join(seedPath, "context")
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.GetWithParams("PromptCreateDirFailed", map[string]interface{}{"Path": contextDir}), err)
	}
	for _, name := range contextFileNames {
		content, err := renderContextTemplate(name, data.SkillsLocale, data)
		if err != nil {
			return err
		}
		if err := writeIfNotExists(filepath.Join(contextDir, name+".md"), content); err != nil {
			return err
		}
	}
	if err := cleanupDeprecatedProjectInstructions(seedPath); err != nil {
		return err
	}
	if err := cleanupGeneratedPromptScaffold(seedPath); err != nil {
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

	profileContent, err := renderContextTemplate("project", data.SkillsLocale, data)
	if err != nil {
		return err
	}
	if err := writeIfNotExists(filepath.Join(basePath, "project.md"), profileContent); err != nil {
		return err
	}

	rulesContent, err := renderContextTemplate("rules", data.SkillsLocale, data)
	if err != nil {
		return err
	}
	return writeIfNotExists(filepath.Join(basePath, "rules.md"), rulesContent)
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

	workspaceDir := filepath.Join(seedPath, "context")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.GetWithParams("PromptCreateDirFailed", map[string]interface{}{"Path": workspaceDir}), err)
	}

	content, err := renderWorkspaceTemplate("skill-workspace-profile", data.SkillsLocale, data)
	if err != nil {
		return err
	}
	if err := writeIfNotExists(filepath.Join(workspaceDir, "workspace.md"), content); err != nil {
		return err
	}
	return nil
}

func renderContextTemplate(name, locale string, data ProjectPromptData) (string, error) {
	templateData, err := readContextTemplate(name, locale)
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

func readContextTemplate(name, locale string) ([]byte, error) {
	if locale == "" {
		locale = config.DefaultToolLocale
	}
	fileName := name + ".md.tmpl"
	if suffix := config.TemplateLocaleSuffix(locale); suffix != "" {
		localizedPath := filepath.ToSlash(filepath.Join("templates", "prompts", "context", name+"."+suffix+".md.tmpl"))
		if data, err := embedfs.FS.ReadFile(localizedPath); err == nil {
			return data, nil
		}
	}
	defaultPath := filepath.ToSlash(filepath.Join("templates", "prompts", "context", fileName))
	return embedfs.FS.ReadFile(defaultPath)
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

func cleanupGeneratedPromptScaffold(seedPath string) error {
	paths := []string{
		filepath.Join(seedPath, "prompts", "project", "project-profile.md"),
		filepath.Join(seedPath, "prompts", "project", "common.md"),
	}
	for _, name := range projectPromptNames {
		paths = append(paths, filepath.Join(seedPath, "prompts", "instructions", name+".md"))
	}
	for _, path := range paths {
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
	_ = os.Remove(filepath.Join(seedPath, "prompts", "project"))
	_ = os.Remove(filepath.Join(seedPath, "prompts", "instructions"))
	_ = os.Remove(filepath.Join(seedPath, "prompts"))
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
