package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
)

// Loader 加载内置模板，并叠加项目/自定义提示词片段。
type Loader struct {
	agentName    string
	locale       string
	skillsLocale string
	seedPath     string
	templates    map[string]*template.Template
	mu           sync.RWMutex
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
	Label       string            `json:"label,omitempty"`
	FinalLength int               `json:"final_length"`
	Parts       []promptPartDebug `json:"parts"`
}

type contextPromptFile struct {
	partName string
	fileName string
}

var contextPromptFiles = []contextPromptFile{
	{partName: "context-background", fileName: "background.md"},
	{partName: "context-constraints", fileName: "constraints.md"},
	{partName: "context-terminology", fileName: "terminology.md"},
	{partName: "context-workspace", fileName: "workspace.md"},
}

// RuntimeTask 标识一次 agent 调用共用的 runtime 文件名前缀。
type RuntimeTask struct {
	ID   string
	Slug string
}

// New 创建提示词模板加载器。
func New(agentName, locale, seedPath string) *Loader {
	return NewWithLocales(agentName, locale, "", seedPath)
}

// NewWithLocales 创建可按提示词用途选择语言的提示词模板加载器。
func NewWithLocales(agentName, locale, skillsLocale, seedPath string) *Loader {
	if locale == "" {
		locale = config.DefaultToolLocale
	}
	if skillsLocale == "" {
		skillsLocale = config.DefaultSkillsLocale
	}

	return &Loader{
		agentName:    agentName,
		locale:       locale,
		skillsLocale: skillsLocale,
		seedPath:     seedPath,
		templates:    make(map[string]*template.Template),
	}
}

// Load 加载指定提示词模板
func (l *Loader) Load(name string) error {
	return l.loadWithLocale(name, l.localeForPrompt(name))
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

	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap(locale)).Parse(string(data))
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
	locale := l.localeForPrompt(name)
	cacheKey := templateCacheKey(locale, name)
	l.mu.RLock()
	_, loaded := l.templates[cacheKey]
	l.mu.RUnlock()
	if !loaded {
		if err := l.loadWithLocale(name, locale); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.render",
			"template", name,
			"agent", l.agentName,
			"locale", locale,
			"error", err,
		)
		return "", err
	}

	base := buf.String()
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
		"context_constraints_length", contextLengths["context-constraints"],
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
		Label:       promptRuntimeLabel(data),
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
	tmpl, err := template.New("output-contract-guard").Option("missingkey=error").Funcs(funcMap(locale)).Parse(string(data))
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
	base = strings.TrimSpace(base)
	if contractGuard == "" {
		return base
	}
	if base == "" {
		return contractGuard
	}
	return strings.TrimSpace(base + "\n\n" + contractGuard)
}

func (l *Loader) localeForPrompt(name string) string {
	if strings.TrimSpace(name) != "" {
		return l.skillsLocale
	}
	return l.skillsLocale
}

func templateCacheKey(locale, name string) string {
	return locale + "/" + name
}

var htmlCommentBlockPattern = regexp.MustCompile(`(?s)<!--.*?-->\s*`)
var generatedWorkspaceProjectLinePattern = regexp.MustCompile("^[-*] `[^`]+` \\(`")

func prepareContextPromptFragment(content string) string {
	content = stripPromptMetadata(content)
	content = removeDefaultContextScaffold(content)
	return strings.TrimSpace(content)
}

func stripPromptMetadata(content string) string {
	return htmlCommentBlockPattern.ReplaceAllString(content, "")
}

func removeDefaultContextScaffold(content string) string {
	defaultFragments := []string{
		"# 背景与外部事实",
		"## 业务背景",
		"说明这个项目服务的业务、用户、核心资源或关键流程。",
		"## 外部系统和依赖",
		"记录代码里不容易看出的外部平台、接口、账号体系、人工流程或上下游系统。",
		"## 线上事实和历史约束",
		"记录灰度策略、兼容对象、迁移状态、历史包袱、发布窗口或运维事实。",
		"# 约束与边界",
		"## 必须遵守",
		"记录未来所有学习、检查和生成都必须遵守的长期团队约束。",
		"## 禁止变更",
		"记录不能破坏的兼容性、安全、数据、发布或运维边界。",
		"## 验证偏好",
		"记录必须优先使用或避免使用的验证方式、环境限制、人工确认要求。",
		"# 术语与映射",
		"## 业务词到代码词",
		"记录需求、产品、运营常用词与代码里的包名、类型名、字段名、状态名之间的对应关系。",
		"## 别名和历史名称",
		"记录同一概念的旧名称、缩写、别名或容易混淆的叫法。",
		"## 状态和枚举",
		"记录代码中状态值、枚举值、错误码、事件名对应的业务含义。",
		"# Background and External Facts",
		"## Business Background",
		"Describe the business, users, core resources, or key flows this project supports.",
		"## External Systems and Dependencies",
		"Record external platforms, APIs, account systems, manual processes, or upstream/downstream systems that are hard to infer from code.",
		"## Production Facts and Historical Constraints",
		"Record rollout strategy, compatibility targets, migration status, historical constraints, release windows, or operations facts.",
		"# Constraints and Boundaries",
		"## Must Follow",
		"Record long-lived team constraints that future learning, checks, and generated skills must follow.",
		"## Forbidden Changes",
		"Record compatibility, security, data, release, or operations boundaries that must not be broken.",
		"## Validation Preferences",
		"Record preferred or forbidden validation methods, environment limits, or required manual confirmations.",
		"# Terminology and Mappings",
		"## Business Terms to Code Terms",
		"Record mappings from product, business, or operations terms to package names, type names, field names, state names, or code identifiers.",
		"## Aliases and Historical Names",
		"Record old names, abbreviations, aliases, or confusing names for the same concept.",
		"## States and Enumerations",
		"Record business meanings for state values, enum values, error codes, or event names used in code.",
		"# 工作区背景",
		"## 子项目职责",
		"记录每个子项目负责的产品能力、对外接口、依赖方向和交付边界。",
		"- 暂未识别子项目。",
		"## 跨项目约束",
		"记录多个子项目共同遵守的契约、共享库、配置、环境变量、数据 schema 或发布边界。",
		"## 路由和影响范围",
		"记录哪些路径或任务需要同时查看多个子项目 skill，例如契约、共享代码、基础设施、根目录脚本或部署配置。",
		"# Workspace Background",
		"## Child Project Responsibilities",
		"Record each child project's product capability, public interfaces, dependency direction, and delivery boundary.",
		"- No child projects detected yet.",
		"## Cross-Project Constraints",
		"Record contracts, shared libraries, config, environment variables, data schemas, or release boundaries shared by multiple child projects.",
		"## Routing and Impact Radius",
		"Record paths or tasks that require multiple child project skills to be read together, such as contracts, shared code, infrastructure, root scripts, or deployment config.",
	}
	for _, fragment := range defaultFragments {
		content = strings.ReplaceAll(content, fragment, "")
	}
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- 项目名称:") ||
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

func (l *Loader) saveRenderedPrompt(name, content string, manifest renderedPromptManifest) {
	if strings.TrimSpace(l.seedPath) == "" {
		return
	}

	dir := layout.New(l.seedPath).Runtime("rendered-prompts")
	if config.DefaultAutoDeleteRenderedPrompts {
		if err := os.RemoveAll(dir); err != nil {
			logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.rendered.mkdir",
			"template", name,
			"path", dir,
			"error", err,
		)
		return
	}

	slug := strings.TrimSpace(manifest.Slug)
	if slug == "" {
		parts := []string{name}
		if strings.TrimSpace(manifest.Label) != "" {
			parts = append(parts, manifest.Label)
		}
		slug = strings.Join(parts, "-")
	}
	filename := runtimefiles.NameWithID(manifest.RuntimeID, slug) + ".md"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content+"\n"), 0600); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
			"operation", "prompt.rendered.manifest.marshal",
			"template", name,
			"path", manifestPath,
			"error", err,
		)
		return
	}
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0600); err != nil {
		logger.Diagnostic(i18n.Get("LoggerDiagnosticOperationFailed"),
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
		"label", manifest.Label,
		"content_length", len(content),
	)
}

func promptRuntimeLabel(data interface{}) string {
	return promptStringField(data, "RuntimeLabel")
}

func promptStringField(data interface{}, fieldName string) string {
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
			keyText := fmt.Sprint(key.Interface())
			if keyText == fieldName || keyText == strings.ToUpper(fieldName) {
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
	field := value.FieldByName(fieldName)
	if field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}

func funcMap(locale string) template.FuncMap {
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
			return "Preserve framework names, library names, commands, file paths, function signatures, config keys, environment variables, and code identifiers exactly when needed."
		},
		"jsonContract": func(name string) (string, error) {
			return aicontract.JSONSchema(name)
		},
	}
}

type outputLanguage struct {
	instruction            string
	translationInstruction string
}

func outputLanguageSpec(locale string) outputLanguage {
	switch config.NormalizeSkillsLocale(locale) {
	case i18n.LocaleChinese:
		return outputLanguage{
			instruction:            "All user-facing natural-language fields must be written in Simplified Chinese (zh-CN). Technical identifiers, framework names, library names, commands, file paths, function signatures, config keys, environment variables, enum values, and code identifiers must remain unchanged when needed.",
			translationInstruction: "If earlier context, existing Skills files, learned patterns, README text, comments, or user-provided prompt fragments contain English or another language, translate or rewrite prose into Simplified Chinese while preserving technical identifiers.",
		}
	default:
		return outputLanguage{
			instruction:            "All user-facing natural-language fields must be written in English (en-US). Technical identifiers, framework names, library names, commands, file paths, function signatures, config keys, environment variables, enum values, and code identifiers must remain unchanged when needed.",
			translationInstruction: "If earlier context, existing Skills files, learned patterns, README text, comments, or user-provided prompt fragments contain Chinese or another language, translate or rewrite prose into English while preserving technical identifiers.",
		}
	}
}
