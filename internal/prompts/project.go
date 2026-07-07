package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	textutil "github.com/silaswei-io/skills-seed/internal/utils/text"
)

var contextFileNames = []string{
	"README",
	"background",
	"constraints",
	"terminology",
}

// ProjectContextData 渲染项目 context 模板的数据。
type ProjectContextData struct {
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

// WorkspaceContextData 渲染工作区 context 模板的数据。
type WorkspaceContextData struct {
	ProgramVersion      string
	PromptTemplatesHash string
	WorkspaceName       string
	WorkspaceRoot       string
	Projects            []WorkspaceContextProject
	Locale              string
	SkillsLocale        string
}

// WorkspaceContextProject 是工作区 context 中展示的子项目摘要。
type WorkspaceContextProject struct {
	ID       string
	Path     string
	Type     string
	Language string
}

// EnsureProjectContext 初始化项目级 context 文件。
func EnsureProjectContext(seedPath string, data ProjectContextData) error {
	data = normalizeProjectContextData(data)
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
	if err := removeDeprecatedContextFiles(contextDir); err != nil {
		return err
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
	return removeDeprecatedPromptsDir(seedPath)
}

// EnsureWorkspaceContext 初始化工作区级 context 文件。
func EnsureWorkspaceContext(seedPath string, data WorkspaceContextData) error {
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

	content, err := renderWorkspaceContextTemplate(data.SkillsLocale, data)
	if err != nil {
		return err
	}
	if err := writeIfNotExists(filepath.Join(workspaceDir, "workspace.md"), content); err != nil {
		return err
	}
	return nil
}

func renderContextTemplate(name, locale string, data ProjectContextData) (string, error) {
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

func renderWorkspaceContextTemplate(locale string, data WorkspaceContextData) (string, error) {
	templateData, err := readContextTemplate("workspace", locale)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("workspace").Parse(string(templateData))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func normalizeProjectContextData(data ProjectContextData) ProjectContextData {
	data.Structure = textutil.NormalizeStructureSummary(data.Structure)
	return data
}

func writeIfNotExists(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func removeDeprecatedPromptsDir(seedPath string) error {
	return os.RemoveAll(filepath.Join(seedPath, "prompts"))
}

func removeDeprecatedContextFiles(contextDir string) error {
	for _, name := range []string{"project.md", "rules.md", "glossary.md"} {
		if err := os.Remove(filepath.Join(contextDir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
