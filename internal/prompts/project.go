package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/metadata"
)

var projectPromptNames = []string{
	"init-skills",
	"project-analysis",
	"analyze",
	"batch-learn",
	"generate_skills_summary",
	"generate_fixes",
	"merge-patterns",
}

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
}

func EnsureProjectPrompts(seedPath string, data ProjectPromptData) error {
	if data.ProgramVersion == "" {
		data.ProgramVersion = metadata.ProgramVersion
	}
	if data.PromptTemplatesHash == "" {
		data.PromptTemplatesHash = metadata.HashOrUnavailable(metadata.PromptTemplatesHash(embedfs.FS))
	}

	baseDirs := []string{
		filepath.Join(seedPath, "prompts"),
		filepath.Join(seedPath, "prompts", "project"),
		filepath.Join(seedPath, "prompts", "custom"),
	}

	for _, dir := range baseDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("%s: %w", i18n.GetWithParams("PromptCreateDirFailed", map[string]interface{}{"Path": dir}), err)
		}
	}

	profileContent, err := renderProjectTemplate("project-profile", data.Locale, data)
	if err != nil {
		return err
	}
	profilePath := filepath.Join(seedPath, "prompts", "project", "project-profile.md")
	if err := writeIfNotExists(profilePath, profileContent); err != nil {
		return err
	}

	data.PromptName = "common"
	projectContent, err := renderProjectTemplate("project-prompt", data.Locale, data)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(seedPath, "prompts", "project", "common.md")
	if err := writeIfNotExists(projectPath, projectContent); err != nil {
		return err
	}

	for _, name := range projectPromptNames {
		data.PromptName = name

		customContent, err := renderProjectTemplate("custom-override", data.Locale, data)
		if err != nil {
			return err
		}
		customPath := filepath.Join(seedPath, "prompts", "custom", name+".override.md")
		if err := writeIfNotExists(customPath, customContent); err != nil {
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
		locale = "en-US"
	}

	localizedPath := metadata.ProjectPromptTemplatePath(name, locale)
	data, err := embedfs.FS.ReadFile(localizedPath)
	if err == nil {
		return data, nil
	}

	defaultPath := metadata.ProjectPromptTemplatePath(name, "")
	return embedfs.FS.ReadFile(defaultPath)
}

func writeIfNotExists(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}
