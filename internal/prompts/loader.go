package prompts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/metadata"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

// Loader 加载内置模板，并按 Skills 目标语言约束 AI 自然语言输出。
type Loader struct {
	agentName string
	locale    string
	seedPath  string
	templates map[string]*template.Template
	mu        sync.RWMutex
}

// Renderer 是 Agent 依赖的最小提示词渲染能力，便于测试渲染错误链路。
type Renderer interface {
	Render(name string, data interface{}) (string, error)
	RenderForRuntimeTask(name string, data interface{}, task RuntimeTask) (string, error)
}

type promptPartDebug struct {
	Name       string `json:"name"`
	Included   bool   `json:"included"`
	Length     int    `json:"length"`
	RawLength  int    `json:"raw_length,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`
}

type renderedPromptManifest struct {
	Template    string            `json:"template"`
	Agent       string            `json:"agent"`
	Locale      string            `json:"locale"`
	RuntimeID   string            `json:"runtime_id,omitempty"`
	Slug        string            `json:"slug,omitempty"`
	FinalLength int               `json:"final_length"`
	Parts       []promptPartDebug `json:"parts"`
}

type contextPromptFile struct {
	partName string
	fileName string
}

var contextPromptFiles = []contextPromptFile{
	{partName: "context-background", fileName: "background.md"},
	{partName: "context-terminology", fileName: "terminology.md"},
	{partName: "context-workspace", fileName: "workspace.md"},
}

var knowledgePromptNames = map[string]bool{
	"core-user-pattern":           true,
	"core-workspace-profile":      true,
	"core-workspace-spec":         true,
	"learning-delta-pack-analyze": true,
	"learning-pack-analyze":       true,
	"learning-pack-plan":          true,
	"learning-pattern-normalize":  true,
	"learning-profile-refresh":    true,
	"learning-authority-extract":  true,
	"learning-authority-review":   true,
	"learning-knowledge-review":   true,
}

// RuntimeTask 标识一次 agent 调用共用的 runtime 文件名前缀。
type RuntimeTask struct {
	ID   string
	Slug string
}

// New 创建提示词模板加载器。
func New(agentName, locale, seedPath string) *Loader {
	if locale == "" {
		locale = config.DefaultSkillsLocale
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
	return l.loadWithLocale(name, l.locale)
}

func (l *Loader) loadWithLocale(name, locale string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	cacheKey := templateCacheKey(locale, name)
	if _, ok := l.templates[cacheKey]; ok {
		return nil
	}

	data, err := l.readEmbeddedTemplate(name)
	if err != nil {
		return err
	}

	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap(locale, l.agentName)).Parse(string(data))
	if err != nil {
		return err
	}

	l.templates[cacheKey] = tmpl
	return nil
}

func (l *Loader) readEmbeddedTemplate(name string) ([]byte, error) {
	for _, agentName := range l.templateAgentNames() {
		defaultPath := metadata.PromptTemplatePath(agentName, name, "")
		data, err := embedfs.FS.ReadFile(defaultPath)
		if err == nil {
			return data, nil
		}
	}

	return nil, os.ErrNotExist
}

func (l *Loader) templateAgentNames() []string {
	return metadata.PromptTemplateProviderFallbacks(l.agentName)
}

// Render 渲染指定提示词模板
func (l *Loader) Render(name string, data interface{}) (string, error) {
	return l.RenderForRuntimeTask(name, data, RuntimeTask{})
}

// RenderForRuntimeTask 渲染提示词并使用指定 runtime 任务名保存调试文件。
func (l *Loader) RenderForRuntimeTask(name string, data interface{}, task RuntimeTask) (string, error) {
	locale := l.locale
	cacheKey := templateCacheKey(locale, name)
	l.mu.RLock()
	_, loaded := l.templates[cacheKey]
	l.mu.RUnlock()
	if !loaded {
		if err := l.loadWithLocale(name, locale); err != nil {
			logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "prompt.load",
				"template", name,
				"agent", l.agentName,
				"locale", locale,
				"error", err,
			)
			return "", err
		}
	}

	l.mu.RLock()
	tmpl := l.templates[cacheKey]
	l.mu.RUnlock()
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.render",
			"template", name,
			"agent", l.agentName,
			"locale", locale,
			"error", err,
		)
		return "", err
	}

	base := l.prependKnowledgeGoal(locale, name, buf.String())
	contractGuard := l.outputContractGuard(locale, name)
	if l.seedPath == "" {
		rendered := l.appendOutputContractGuard(base, contractGuard)
		logger.Diagnostic(i18n.Get("LoggerDiagnosticPromptRendered"),
			"template", name,
			"agent", l.agentName,
			"locale", locale,
			"base_length", len(base),
			"output_contract_guard_length", len(contractGuard),
			"final_length", len(rendered),
			"has_seed_path", false,
		)
		return rendered, nil
	}

	var parts []string
	debugParts := []promptPartDebug{}
	addPart := func(partName, raw, cleaned string) {
		if strings.TrimSpace(cleaned) == "" {
			if strings.TrimSpace(raw) != "" {
				debugParts = append(debugParts, promptPartDebug{
					Name:       partName,
					Included:   false,
					RawLength:  len(raw),
					SkipReason: "empty-after-filter",
				})
			}
			return
		}
		trimmed := strings.TrimSpace(cleaned)
		parts = append(parts, trimmed)
		debugParts = append(debugParts, promptPartDebug{
			Name:      partName,
			Included:  true,
			Length:    len(trimmed),
			RawLength: len(raw),
		})
	}
	if base != "" {
		addPart("base", base, base)
	}
	contextLengths := make(map[string]int, len(contextPromptFiles))
	for _, file := range contextPromptFiles {
		raw := l.readPromptFile(filepath.Join(l.seedPath, "context", file.fileName))
		cleaned := prepareContextPromptFragment(raw)
		addPart(file.partName, raw, cleaned)
		contextLengths[file.partName] = len(cleaned)
	}
	if contractGuard != "" {
		addPart("output-contract-guard", contractGuard, contractGuard)
	}

	rendered := strings.TrimSpace(strings.Join(parts, "\n\n"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticPromptRendered"),
		"template", name,
		"agent", l.agentName,
		"locale", locale,
		"base_length", len(base),
		"context_background_length", contextLengths["context-background"],
		"context_terminology_length", contextLengths["context-terminology"],
		"context_workspace_length", contextLengths["context-workspace"],
		"output_contract_guard_length", len(contractGuard),
		"final_length", len(rendered),
		"has_seed_path", true,
	)
	l.saveRenderedPrompt(name, rendered, renderedPromptManifest{
		Template:    name,
		Agent:       l.agentName,
		Locale:      locale,
		RuntimeID:   task.ID,
		Slug:        task.Slug,
		FinalLength: len(rendered),
		Parts:       debugParts,
	})

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

func (l *Loader) outputContractGuard(locale, promptName string) string {
	if promptName == "output-contract-guard" {
		return ""
	}
	data, err := readAppendTemplate("output-contract-guard")
	if err != nil {
		return ""
	}
	tmpl, err := template.New("output-contract-guard").Option("missingkey=error").Funcs(funcMap(locale, l.agentName)).Parse(string(data))
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{}); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func (l *Loader) prependKnowledgeGoal(locale, promptName, base string) string {
	if !knowledgePromptNames[promptName] {
		return base
	}
	return prependPromptSection(base, l.renderAppendTemplate(locale, "knowledge-goal-contract"))
}

func (l *Loader) renderAppendTemplate(locale, name string) string {
	data, err := readAppendTemplate(name)
	if err != nil {
		return ""
	}
	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap(locale, l.agentName)).Parse(string(data))
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{}); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func readAppendTemplate(name string) ([]byte, error) {
	defaultPath := metadata.PromptAppendTemplatePath(name, "")
	data, err := embedfs.FS.ReadFile(defaultPath)
	if err == nil {
		return data, nil
	}

	return nil, os.ErrNotExist
}

func (l *Loader) appendOutputContractGuard(base, contractGuard string) string {
	return appendPromptSection(base, contractGuard)
}

func appendPromptSection(base, section string) string {
	base = strings.TrimSpace(base)
	section = strings.TrimSpace(section)
	if section == "" {
		return base
	}
	if base == "" {
		return section
	}
	return strings.TrimSpace(base + "\n\n" + section)
}

func prependPromptSection(base, section string) string {
	return appendPromptSection(section, base)
}

func templateCacheKey(locale, name string) string {
	return locale + "/" + name
}

var htmlCommentBlockPattern = regexp.MustCompile(`(?s)<!--.*?-->\s*`)
var generatedWorkspaceProjectLinePattern = regexp.MustCompile("^[-*] `[^`]+` \\(`")

var (
	defaultContextScaffoldOnce  sync.Once
	defaultContextScaffoldLines map[string]struct{}
)

func prepareContextPromptFragment(content string) string {
	content = stripPromptMetadata(content)
	content = removeDefaultContextScaffold(content)
	return strings.TrimSpace(content)
}

func stripPromptMetadata(content string) string {
	return htmlCommentBlockPattern.ReplaceAllString(content, "")
}

func removeDefaultContextScaffold(content string) string {
	scaffoldLines := loadDefaultContextScaffoldLines()
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		_, isDefaultScaffold := scaffoldLines[trimmed]
		if isDefaultScaffold ||
			strings.HasPrefix(trimmed, "- 项目名称:") ||
			strings.HasPrefix(trimmed, "- 主要语言:") ||
			strings.HasPrefix(trimmed, "- 项目根目录:") ||
			strings.HasPrefix(trimmed, "- Project name:") ||
			strings.HasPrefix(trimmed, "- Primary language:") ||
			strings.HasPrefix(trimmed, "- Project root:") ||
			strings.HasPrefix(trimmed, "- 工作区名称:") ||
			strings.HasPrefix(trimmed, "- 工作区根目录:") ||
			strings.HasPrefix(trimmed, "- Workspace name:") ||
			strings.HasPrefix(trimmed, "- Workspace root:") ||
			generatedWorkspaceProjectLinePattern.MatchString(trimmed) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func loadDefaultContextScaffoldLines() map[string]struct{} {
	defaultContextScaffoldOnce.Do(func() {
		defaultContextScaffoldLines = make(map[string]struct{})
		for _, file := range contextPromptFiles {
			for _, locale := range []string{"", i18n.LocaleEnglish} {
				data, err := embedfs.FS.ReadFile(metadata.SeedContextTemplatePath(strings.TrimSuffix(file.fileName, filepath.Ext(file.fileName)), locale))
				if err != nil {
					continue
				}
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line == "" || strings.Contains(line, "{{") || strings.HasPrefix(line, "<!--") {
						continue
					}
					defaultContextScaffoldLines[line] = struct{}{}
				}
			}
		}
	})
	return defaultContextScaffoldLines
}

func (l *Loader) saveRenderedPrompt(name, content string, manifest renderedPromptManifest) {
	if strings.TrimSpace(l.seedPath) == "" {
		return
	}

	dir := layout.New(l.seedPath).Runtime("rendered-prompts")
	if config.DefaultAutoDeleteRenderedPrompts {
		if err := os.RemoveAll(dir); err != nil {
			logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
				"operation", "prompt.rendered.cleanup",
				"template", name,
				"path", dir,
				"error", err,
			)
		}
	}
	if !config.DefaultSaveRenderedPrompts {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.rendered.mkdir",
			"template", name,
			"path", dir,
			"error", err,
		)
		return
	}

	slug := strings.TrimSpace(manifest.Slug)
	if slug == "" {
		slug = name
	}
	filename := runtimefiles.NameWithID(manifest.RuntimeID, slug) + ".md"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content+"\n"), 0600); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.rendered.write",
			"template", name,
			"path", path,
			"error", err,
		)
		return
	}
	manifestPath := strings.TrimSuffix(path, ".md") + ".manifest.json"
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.rendered.manifest.marshal",
			"template", name,
			"path", manifestPath,
			"error", err,
		)
		return
	}
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0600); err != nil {
		logger.DiagnosticError(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.rendered.manifest.write",
			"template", name,
			"path", manifestPath,
			"error", err,
		)
		return
	}
	logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationComplete"),
		"operation", "prompt.rendered.write",
		"template", name,
		"path", path,
		"manifest_path", manifestPath,
		"content_length", len(content),
	)
}

func funcMap(locale, agentName string) template.FuncMap {
	outputLanguage := outputLanguageSpec(locale)
	return template.FuncMap{
		"upper": func(v interface{}) string {
			return strings.ToUpper(fmt.Sprint(v))
		},
		"outputLanguageInstruction": func() string {
			return outputLanguage.instruction
		},
		"translateToOutputLanguageInstruction": func() string {
			return outputLanguage.translationInstruction
		},
		"preserveTechnicalTermsInstruction": func() string {
			return outputLanguage.preserveTechnicalTermsInstruction
		},
		"jsonContract": func(name string) (string, error) {
			return renderJSONContract(agentName, name, aicontract.StructuredOutputOptions{})
		},
		"jsonContractWithProjectIDs": func(name string, projectIDs []string) (string, error) {
			return renderJSONContract(agentName, name, aicontract.StructuredOutputOptions{ProjectIDs: projectIDs})
		},
		"jsonContractWithFocusIDs": func(name string, focusIDs []string) (string, error) {
			return renderJSONContract(agentName, name, aicontract.StructuredOutputOptions{FocusIDs: focusIDs})
		},
	}
}

func renderJSONContract(agentName, name string, opts aicontract.StructuredOutputOptions) (string, error) {
	if strings.EqualFold(strings.TrimSpace(agentName), "codex") {
		return aicontract.StrictStructuredOutputSchemaWithOptions(name, opts)
	}
	return aicontract.StructuredOutputSchemaWithOptions(name, opts)
}

type outputLanguage struct {
	instruction                       string
	translationInstruction            string
	preserveTechnicalTermsInstruction string
}

func outputLanguageSpec(locale string) outputLanguage {
	targetLanguage := "English (en-US)"
	translationTarget := "English"
	switch config.NormalizeSkillsLocale(locale) {
	case i18n.LocaleChinese:
		targetLanguage = "Simplified Chinese (zh-CN)"
		translationTarget = "Simplified Chinese"
	default:
	}
	return outputLanguage{
		instruction:                       "All user-facing natural-language fields must be written in " + targetLanguage + ". Technical identifiers, framework names, library names, commands, file paths, function signatures, config keys, environment variables, enum values, and code identifiers must remain unchanged when needed.",
		translationInstruction:            "If earlier context, existing Skills files, learned patterns, README text, comments, or user-provided prompt fragments contain English or another language, translate or rewrite prose into " + translationTarget + " while preserving technical identifiers.",
		preserveTechnicalTermsInstruction: "Preserve framework names, library names, commands, file paths, function signatures, config keys, environment variables, and code identifiers exactly when needed.",
	}
}
