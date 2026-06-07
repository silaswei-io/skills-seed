package skills

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/silaswei-io/skills-seed/embedfs"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/metadata"
)

// Loader Skills 模板加载器
type Loader struct {
	agentName string
	locale    string
	templates map[string]*template.Template
}

// RenderedFile 表示由模板渲染出的附加文件
type RenderedFile struct {
	Path    string
	Content string
}

// ReferenceItem 描述 SKILL.md 中链接的一个参考文件
type ReferenceItem struct {
	Title       string
	Description string
	Path        string
}

// ReferenceGroup 描述主技能文档中的一组参考文件
type ReferenceGroup struct {
	Title string
	Items []ReferenceItem
}

// NewLoader 创建 Skills 模板加载器
func NewLoader(locale string) *Loader {
	return NewLoaderForAgent(metadata.CommonTemplateProvider, locale)
}

// NewLoaderForAgent 创建指定 Agent 的 Skills 模板加载器
func NewLoaderForAgent(agentName, locale string) *Loader {
	if agentName == "" {
		agentName = metadata.CommonTemplateProvider
	}
	if locale == "" {
		locale = config.DefaultSkillsLocale
	}
	return &Loader{
		agentName: strings.ToLower(agentName),
		locale:    locale,
		templates: make(map[string]*template.Template),
	}
}

// Load 加载指定名称的 Skills 模板
func (l *Loader) Load(name string) (*template.Template, error) {
	entry, ok := TemplateCatalogEntry(name)
	if !ok {
		return nil, fs.ErrNotExist
	}
	return l.loadCatalogTemplate(entry)
}

// LoadReference 加载 references 目录下的模板
// dir 表示目录名（如 "patterns" 或 "examples"）
// category 表示分类名（如 "api", "business"）
func (l *Loader) LoadReference(dir, category string) (*template.Template, error) {
	key := dir + "/" + category
	tmpl, err := l.loadTemplate(key, "project/references/"+dir+"/"+category, metadata.SkillsTemplateExt)
	if err == nil {
		return tmpl, nil
	}
	if dir == "patterns" && category != "default" && errors.Is(err, fs.ErrNotExist) {
		return l.loadTemplate(dir+"/default", "project/references/"+dir+"/default", metadata.SkillsTemplateExt)
	}
	return nil, err
}

// LoadPattern 加载模式模板（便捷方法）
func (l *Loader) LoadPattern(category string) (*template.Template, error) {
	return l.LoadReference("patterns", category)
}

// LoadProjectOverview 加载项目概览模板（特殊处理）
func (l *Loader) LoadProjectOverview() (*template.Template, error) {
	return l.Load("project-reference-overview")
}

// LoadReferenceFile 加载 references/ 下的独立模板。
func (l *Loader) LoadReferenceFile(name string) (*template.Template, error) {
	return l.Load("project-reference-" + name)
}

// Render 渲染指定名称的模板
func (l *Loader) Render(name string, data interface{}) (string, error) {
	tmpl, err := l.Load(name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return normalizeMarkdown(buf.String()), nil
}

// RenderReference 渲染 references 模板
// dir 表示目录名（如 "patterns" 或 "examples"）
// category 表示分类名（如 "api", "business"）
func (l *Loader) RenderReference(dir, category string, data interface{}) (string, error) {
	tmpl, err := l.LoadReference(dir, category)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return normalizeMarkdown(buf.String()), nil
}

// RenderPattern 渲染模式模板（便捷方法）
func (l *Loader) RenderPattern(category string, data interface{}) (string, error) {
	return l.RenderReference("patterns", category, data)
}

// RenderProjectOverview 渲染项目概览模板（便捷方法）
func (l *Loader) RenderProjectOverview(data interface{}) (string, error) {
	tmpl, err := l.LoadProjectOverview()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return normalizeMarkdown(buf.String()), nil
}

// RenderReferenceFile 渲染独立的 references/{name}.md 模板。
func (l *Loader) RenderReferenceFile(name string, data interface{}) (string, error) {
	tmpl, err := l.LoadReferenceFile(name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return normalizeMarkdown(buf.String()), nil
}

// RenderRelative 渲染 skills 模板目录下的任意相对模板
func (l *Loader) RenderRelative(relativeName string, data interface{}) (string, error) {
	tmpl, err := l.loadTemplate("relative/"+relativeName, relativeName, metadata.SkillsTemplateExt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return normalizeMarkdown(buf.String()), nil
}

// RenderAgentMetadataFiles 渲染 agents/ 下所有可选元数据模板
func (l *Loader) RenderAgentMetadataFiles(data interface{}) ([]RenderedFile, error) {
	entries := l.agentMetadataEntries()
	files := make([]RenderedFile, 0, len(entries))
	for _, entry := range entries {
		templatePath, err := l.catalogTemplatePath(entry)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		templateData, err := embedfs.FS.ReadFile(templatePath)
		if err != nil {
			return nil, err
		}

		tmpl, err := template.New(entry.ID).Option("missingkey=error").Funcs(funcMap()).Parse(string(templateData))
		if err != nil {
			return nil, err
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}

		files = append(files, RenderedFile{
			Path:    entry.OutputPath,
			Content: buf.String(),
		})
	}

	return files, nil
}

func (l *Loader) agentMetadataEntries() []TemplateEntry {
	var entries []TemplateEntry
	for _, entry := range skillTemplateCatalog {
		if strings.HasPrefix(entry.OutputPath, "agents/") && l.entryAllowsProvider(entry, l.agentName) {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (l *Loader) catalogTemplatePath(entry TemplateEntry) (string, error) {
	var lastErr error
	for _, agentName := range l.templateAgentNames() {
		if !l.entryAllowsProvider(entry, agentName) {
			continue
		}
		localizedPath := metadata.SkillsTemplatePath(agentName, entry.RelativeName, l.locale, entry.Ext)
		if _, err := embedfs.FS.ReadFile(localizedPath); err == nil {
			return localizedPath, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			lastErr = err
		}

		defaultPath := metadata.SkillsTemplatePath(agentName, entry.RelativeName, "", entry.Ext)
		if _, err := embedfs.FS.ReadFile(defaultPath); err == nil {
			return defaultPath, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fs.ErrNotExist
}

func (l *Loader) entryAllowsProvider(entry TemplateEntry, provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	for _, allowed := range entry.Providers {
		if provider == strings.ToLower(allowed) {
			return true
		}
	}
	return false
}

// Clear 清除缓存
func (l *Loader) Clear() {
	l.templates = make(map[string]*template.Template)
}

// TemplateExists 检查模板是否存在
func (l *Loader) TemplateExists(name string) bool {
	_, err := l.Load(name)
	return err == nil
}

// GetLocale 获取当前语言设置
func (l *Loader) GetLocale() string {
	return l.locale
}

// GetAgentName 获取当前 Agent 名称
func (l *Loader) GetAgentName() string {
	return l.agentName
}

func (l *Loader) loadTemplate(key, relativeName, ext string) (*template.Template, error) {
	if cached, ok := l.templates[key]; ok {
		return cached, nil
	}

	data, err := l.readTemplateData(relativeName, ext)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(key).Option("missingkey=error").Funcs(funcMap()).Parse(string(data))
	if err != nil {
		return nil, err
	}

	l.templates[key] = tmpl
	return tmpl, nil
}

func (l *Loader) loadCatalogTemplate(entry TemplateEntry) (*template.Template, error) {
	if cached, ok := l.templates[entry.ID]; ok {
		return cached, nil
	}

	templatePath, err := l.catalogTemplatePath(entry)
	if err != nil {
		return nil, err
	}
	data, err := embedfs.FS.ReadFile(templatePath)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(entry.ID).Option("missingkey=error").Funcs(funcMap()).Parse(string(data))
	if err != nil {
		return nil, err
	}
	l.templates[entry.ID] = tmpl
	return tmpl, nil
}

func (l *Loader) readTemplateData(relativeName, ext string) ([]byte, error) {
	var lastErr error
	for _, agentName := range l.templateAgentNames() {
		localizedPath := metadata.SkillsTemplatePath(agentName, relativeName, l.locale, ext)
		data, err := embedfs.FS.ReadFile(localizedPath)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			lastErr = err
		}

		defaultPath := metadata.SkillsTemplatePath(agentName, relativeName, "", ext)
		data, err = embedfs.FS.ReadFile(defaultPath)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			lastErr = err
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fs.ErrNotExist
}

func (l *Loader) templateAgentNames() []string {
	return metadata.TemplateProviderFallbacks(l.agentName)
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"mul": func(a, b interface{}) float64 {
			return toFloat(a) * toFloat(b)
		},
		"upper": func(v interface{}) string {
			return strings.ToUpper(fmt.Sprint(v))
		},
		"add": func(a, b interface{}) int {
			return int(toFloat(a) + toFloat(b))
		},
	}
}

func normalizeMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	normalized := make([]string, 0, len(lines))
	inFence := false
	blankCount := 0

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			blankCount = 0
			normalized = append(normalized, line)
			continue
		}
		if !inFence && strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
			normalized = append(normalized, "")
			continue
		}

		blankCount = 0
		normalized = append(normalized, line)
	}

	return strings.TrimRight(strings.Join(normalized, "\n"), "\n") + "\n"
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	default:
		return 0
	}
}
