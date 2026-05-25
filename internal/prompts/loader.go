package prompts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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
	mu        sync.RWMutex
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
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.templates[name]; ok {
		return nil
	}

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
	l.mu.RLock()
	_, loaded := l.templates[name]
	l.mu.RUnlock()
	if !loaded {
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

	l.mu.RLock()
	tmpl := l.templates[name]
	l.mu.RUnlock()
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
	scopedProfile, scopedCommon, scopedPrompt := l.readScopedProjectPrompts(name, data)
	workspacePrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "workspace", name+".md"))
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
	if scopedProfile != "" {
		parts = append(parts, strings.TrimSpace(scopedProfile))
	}
	if scopedCommon != "" {
		parts = append(parts, strings.TrimSpace(scopedCommon))
	}
	if scopedPrompt != "" {
		parts = append(parts, strings.TrimSpace(scopedPrompt))
	}
	if workspacePrompt != "" {
		parts = append(parts, strings.TrimSpace(workspacePrompt))
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
		"scoped_project_profile_length", len(scopedProfile),
		"scoped_common_project_prompt_length", len(scopedCommon),
		"scoped_project_prompt_length", len(scopedPrompt),
		"workspace_prompt_length", len(workspacePrompt),
		"custom_prompt_length", len(customPrompt),
		"final_length", len(rendered),
		"has_seed_path", true,
	)

	return rendered, nil
}

// Clear 清空已加载的提示词模板缓存
func (l *Loader) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
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

func (l *Loader) readScopedProjectPrompts(name string, data interface{}) (string, string, string) {
	projectName := promptProjectName(data)
	if projectName == "" {
		return "", "", ""
	}
	basePath := filepath.Join(l.seedPath, "prompts", "projects", projectName)
	return l.readPromptFile(filepath.Join(basePath, "project-profile.md")),
		l.readPromptFile(filepath.Join(basePath, "common.md")),
		l.readPromptFile(filepath.Join(basePath, name+".project.md"))
}

func promptProjectName(data interface{}) string {
	if data == nil {
		return ""
	}
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Map {
		for _, key := range value.MapKeys() {
			if fmt.Sprint(key.Interface()) == "ProjectName" || fmt.Sprint(key.Interface()) == "PROJECT_NAME" {
				mapValue := value.MapIndex(key)
				if mapValue.IsValid() {
					return fmt.Sprint(mapValue.Interface())
				}
			}
		}
		return ""
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	field := value.FieldByName("ProjectName")
	if field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"upper": func(v interface{}) string {
			return strings.ToUpper(fmt.Sprint(v))
		},
	}
}
