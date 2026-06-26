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
		skillsLocale = locale
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

	data, err := l.readEmbeddedTemplateWithLocale(name, locale)
	if err != nil {
		return err
	}

	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap()).Parse(string(data))
	if err != nil {
		return err
	}

	l.templates[cacheKey] = tmpl
	return nil
}

func (l *Loader) readEmbeddedTemplate(name string) ([]byte, error) {
	return l.readEmbeddedTemplateWithLocale(name, l.localeForPrompt(name))
}

func (l *Loader) readEmbeddedTemplateWithLocale(name, locale string) ([]byte, error) {
	for _, agentName := range l.templateAgentNames() {
		localizedPath := metadata.PromptTemplatePath(agentName, name, locale)
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

	rawProjectProfile := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "project", "project-profile.md"))
	rawCommonProjectPrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "project", "common.md"))
	rawProjectPrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "project", name+".md"))
	projectProfile := prepareProjectProfilePrompt(rawProjectProfile)
	commonProjectPrompt := prepareUserPromptFragment(rawCommonProjectPrompt)
	projectPrompt := prepareUserPromptFragment(rawProjectPrompt)
	scopedProfile, scopedCommon, scopedPrompt := l.readScopedProjectPrompts(name, data)
	rawScopedProfile, rawScopedCommon, rawScopedPrompt := scopedProfile, scopedCommon, scopedPrompt
	scopedProfile = prepareProjectProfilePrompt(scopedProfile)
	scopedCommon = prepareUserPromptFragment(scopedCommon)
	scopedPrompt = prepareUserPromptFragment(scopedPrompt)
	rawWorkspacePrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "workspace", name+".md"))
	rawInstructionsPrompt := l.readPromptFile(filepath.Join(l.seedPath, "prompts", "instructions", name+".md"))
	workspacePrompt := prepareUserPromptFragment(rawWorkspacePrompt)
	instructionsPrompt := prepareUserPromptFragment(rawInstructionsPrompt)

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
	addPart("project-profile", rawProjectProfile, projectProfile)
	addPart("project-common", rawCommonProjectPrompt, commonProjectPrompt)
	addPart("project-prompt", rawProjectPrompt, projectPrompt)
	addPart("scoped-project-profile", rawScopedProfile, scopedProfile)
	addPart("scoped-project-common", rawScopedCommon, scopedCommon)
	addPart("scoped-project-prompt", rawScopedPrompt, scopedPrompt)
	addPart("workspace-prompt", rawWorkspacePrompt, workspacePrompt)
	addPart("instructions-prompt", rawInstructionsPrompt, instructionsPrompt)
	if contractGuard != "" {
		addPart("output-contract-guard", contractGuard, contractGuard)
	}

	rendered := strings.TrimSpace(strings.Join(parts, "\n\n"))
	logger.Diagnostic(i18n.Get("LoggerDiagnosticPromptRendered"),
		"template", name,
		"agent", l.agentName,
		"locale", locale,
		"base_length", len(base),
		"project_profile_length", len(projectProfile),
		"common_project_prompt_length", len(commonProjectPrompt),
		"project_prompt_length", len(projectPrompt),
		"scoped_project_profile_length", len(scopedProfile),
		"scoped_common_project_prompt_length", len(scopedCommon),
		"scoped_project_prompt_length", len(scopedPrompt),
		"workspace_prompt_length", len(workspacePrompt),
		"instructions_prompt_length", len(instructionsPrompt),
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
	data, err := readAppendTemplateWithLocale("output-contract-guard", locale)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readAppendTemplateWithLocale(name, locale string) ([]byte, error) {
	localizedPath := metadata.PromptAppendTemplatePath(name, locale)
	data, err := embedfs.FS.ReadFile(localizedPath)
	if err == nil {
		return data, nil
	}

	defaultPath := metadata.PromptAppendTemplatePath(name, "")
	data, err = embedfs.FS.ReadFile(defaultPath)
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

func prepareUserPromptFragment(content string) string {
	content = stripPromptMetadata(content)
	content = removeLegacyDefaultUserInstructionScaffold(content)
	content = removeLegacyDefaultProjectPromptScaffold(content)
	content = strings.TrimSpace(content)
	if content == "# 用户补充指令" || content == "# User Instructions" || content == "# 项目专属约束" || content == "# Project-Specific Constraints" {
		return ""
	}
	return content
}

func prepareProjectProfilePrompt(content string) string {
	content = stripPromptMetadata(content)
	content = removeUnrecordedProfileSections(content)
	content = removeStructureSummarySection(content)
	return strings.TrimSpace(content)
}

func stripPromptMetadata(content string) string {
	return htmlCommentBlockPattern.ReplaceAllString(content, "")
}

func removeLegacyDefaultUserInstructionScaffold(content string) string {
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "# 用户补充指令" || trimmed == "# User Instructions":
			continue
		case strings.Contains(trimmed, "这些内容会追加到内置") || strings.Contains(trimmed, "This content is appended after the built-in"):
			continue
		case strings.Contains(trimmed, "在此补充团队约束") || strings.Contains(trimmed, "Add team constraints, coding preferences"):
			continue
		default:
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func removeLegacyDefaultProjectPromptScaffold(content string) string {
	defaultFragments := []string{
		"# 项目专属约束",
		"处理这个项目时，优先遵循当前仓库的真实结构、命名风格和已有模式",
		"## 项目画像来源",
		"请结合 `project-profile.md` 中记录的项目背景理解代码，不要输出适用于任意项目的泛化建议",
		"## 额外要求",
		"- 先遵循本项目现有结构",
		"- 优先复用现有模式",
		"- 仅在必要时引入新抽象",
		"- 输出必须具体到当前项目",
		"# Project-Specific Constraints",
		"When working on this project, prioritize the real structure, naming style, and established patterns in this repository",
		"## Project Context Source",
		"Use `project-profile.md` as the primary background for this project. Avoid generic advice that would fit any project",
		"## Extra Requirements",
		"- Follow the current project structure first",
		"- Reuse existing patterns whenever possible",
		"- Introduce new abstractions only when necessary",
		"- Keep outputs specific to this project",
	}
	for _, fragment := range defaultFragments {
		content = strings.ReplaceAll(content, fragment, "")
	}
	return content
}

func removeUnrecordedProfileSections(content string) string {
	sectionTitles := []string{
		"## 架构摘要",
		"## 关键模块",
		"## 团队编码风格",
		"## Architecture Summary",
		"## Key Modules",
		"## Team Coding Style",
	}
	for _, title := range sectionTitles {
		content = removeSectionWithOnlyValues(content, title, []string{"未记录", "Not recorded"})
	}
	return content
}

func removeStructureSummarySection(content string) string {
	for _, title := range []string{"## 目录结构摘要", "## Structure Summary"} {
		content = removeMarkdownSection(content, title)
	}
	return content
}

func removeSectionWithOnlyValues(content, title string, values []string) string {
	section := extractMarkdownSection(content, title)
	if section == "" {
		return content
	}
	body := strings.TrimSpace(strings.TrimPrefix(section, title))
	for _, value := range values {
		if body == value {
			return strings.Replace(content, section, "", 1)
		}
	}
	return content
}

func removeMarkdownSection(content, title string) string {
	section := extractMarkdownSection(content, title)
	if section == "" {
		return content
	}
	return strings.Replace(content, section, "", 1)
}

func extractMarkdownSection(content, title string) string {
	start := strings.Index(content, title)
	if start < 0 {
		return ""
	}
	rest := content[start+len(title):]
	nextRel := -1
	for _, marker := range []string{"\n## ", "\n# "} {
		if idx := strings.Index(rest, marker); idx >= 0 && (nextRel < 0 || idx < nextRel) {
			nextRel = idx
		}
	}
	if nextRel < 0 {
		return content[start:]
	}
	return content[start : start+len(title)+nextRel]
}

func (l *Loader) readScopedProjectPrompts(name string, data interface{}) (string, string, string) {
	projectName := promptProjectName(data)
	if projectName == "" {
		return "", "", ""
	}
	basePath := filepath.Join(l.seedPath, "prompts", "projects", projectName)
	return l.readPromptFile(filepath.Join(basePath, "project-profile.md")),
		l.readPromptFile(filepath.Join(basePath, "common.md")),
		l.readPromptFile(filepath.Join(basePath, name+".md"))
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

func promptProjectName(data interface{}) string {
	return promptStringField(data, "ProjectName")
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

func funcMap() template.FuncMap {
	return template.FuncMap{
		"upper": func(v interface{}) string {
			return strings.ToUpper(fmt.Sprint(v))
		},
	}
}
