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
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
)

// Loader loads built-in templates and overlays project/custom prompt fragments
type Loader struct {
	agentName string
	locale    string
	seedPath  string
	templates map[string]*template.Template
}

// NewLoader 创建提示词模板加载器
func NewLoader(agentName, locale, seedPath string) *Loader {
	if locale == "" {
		locale = "en-US"
	}

	return &Loader{
		agentName: agentName,
		locale:    locale,
		seedPath:  seedPath,
		templates: make(map[string]*template.Template),
	}
}

// Load 加载指定提示词模板
func (l *Loader) Load(name string) error {
	data, err := l.readEmbeddedTemplate(name)
	if err != nil {
		return err
	}

	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap()).Parse(string(data))
	if err != nil {
		return err
	}

	l.templates[name] = tmpl
	return nil
}

func (l *Loader) readEmbeddedTemplate(name string) ([]byte, error) {
	for _, agentName := range l.templateAgentNames() {
		localizedPath := metadata.PromptTemplatePath(agentName, name, l.locale)
		data, err := embedfs.FS.ReadFile(localizedPath)
		if err == nil {
			return data, nil
		}

		defaultPath := metadata.PromptTemplatePath(agentName, name, "")
		data, err = embedfs.FS.ReadFile(defaultPath)
		if err == nil {
			return data, nil
		}
	}

	return nil, os.ErrNotExist
}

func (l *Loader) templateAgentNames() []string {
	return metadata.TemplateProviderFallbacks(l.agentName)
}

// Render 渲染指定提示词模板
func (l *Loader) Render(name string, data interface{}) (string, error) {
	if _, ok := l.templates[name]; !ok {
		if err := l.Load(name); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "prompt.load",
				"template", name,
				"agent", l.agentName,
				"locale", l.locale,
				"error", err,
			)
			return "", err
		}
	}

	tmpl := l.templates[name]
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.render",
			"template", name,
			"agent", l.agentName,
			"locale", l.locale,
			"error", err,
		)
		return "", err
	}

	base := buf.String()
	if l.seedPath == "" {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticPromptRendered"),
			"template", name,
			"agent", l.agentName,
			"locale", l.locale,
			"base_length", len(base),
			"final_length", len(strings.TrimSpace(base)),
			"has_seed_path", false,
		)
		return base, nil
	}

	projectProfile := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "project", "project-profile.md"))
	commonProjectPrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "project", "common.md"))
	projectPrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "project", name+".project.md"))
	customPrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "custom", name+".override.md"))

	var parts []string
	if base != "" {
		parts = append(parts, strings.TrimSpace(base))
	}
	if projectProfile != "" {
		parts = append(parts, strings.TrimSpace(projectProfile))
	}
	if commonProjectPrompt != "" {
		parts = append(parts, strings.TrimSpace(commonProjectPrompt))
	}
	if projectPrompt != "" {
		parts = append(parts, strings.TrimSpace(projectPrompt))
	}
	if customPrompt != "" {
		parts = append(parts, strings.TrimSpace(customPrompt))
	}

	rendered := strings.TrimSpace(strings.Join(parts, "\n\n"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticPromptRendered"),
		"template", name,
		"agent", l.agentName,
		"locale", l.locale,
		"base_length", len(base),
		"project_profile_length", len(projectProfile),
		"common_project_prompt_length", len(commonProjectPrompt),
		"project_prompt_length", len(projectPrompt),
		"custom_prompt_length", len(customPrompt),
		"final_length", len(rendered),
		"has_seed_path", true,
	)

	return rendered, nil
}

// Clear 清空已加载的提示词模板缓存
func (l *Loader) Clear() {
	l.templates = make(map[string]*template.Template)
}

// Preload 预加载多个提示词模板
func (l *Loader) Preload(names []string) error {
	for _, name := range names {
		if err := l.Load(name); err != nil {
			return err
		}
	}
	return nil
}

func (l *Loader) readPromptFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"upper": func(v interface{}) string {
			return strings.ToUpper(fmt.Sprint(v))
		},
	}
}
