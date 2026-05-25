package skills

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/silaswei-io/skills-seed/embedfs"
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

// ReferenceItem describes one generated reference file linked from SKILL.md.
type ReferenceItem struct {
	Title       string
	Description string
	Path        string
}

// ReferenceGroup groups related pattern reference files for the main skill.
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
		locale = "en-US" // 默认英文
	}
	return &Loader{
		agentName: strings.ToLower(agentName),
		locale:    locale,
		templates: make(map[string]*template.Template),
	}
}

// Load 加载指定名称的 Skills 模板
func (l *Loader) Load(name string) (*template.Template, error) {
	return l.loadTemplate(name, strings.ToUpper(name), metadata.SkillsTemplateExt)
}

// LoadReference 加载 references 目录下的模板
// dir: 目录名（如 "patterns" 或 "examples"）
// category: 分类名（如 "api", "business"）
func (l *Loader) LoadReference(dir, category string) (*template.Template, error) {
	key := dir + "/" + category
	tmpl, err := l.loadTemplate(key, "references/"+dir+"/"+category, metadata.SkillsTemplateExt)
	if err == nil {
		return tmpl, nil
	}
	if dir == "patterns" && category != "default" && errors.Is(err, fs.ErrNotExist) {
		return l.loadTemplate(dir+"/default", "references/"+dir+"/default", metadata.SkillsTemplateExt)
	}
	return nil, err
}

// LoadPattern 加载模式模板（便捷方法）
func (l *Loader) LoadPattern(category string) (*template.Template, error) {
	return l.LoadReference("patterns", category)
}

// LoadProjectOverview 加载项目概览模板（特殊处理）
func (l *Loader) LoadProjectOverview() (*template.Template, error) {
	key := "project-overview"
	return l.loadTemplate(key, "references/project-overview", metadata.SkillsTemplateExt)
}

// LoadReferenceFile loads a standalone template under references/.
func (l *Loader) LoadReferenceFile(name string) (*template.Template, error) {
	key := "reference-file/" + name
	return l.loadTemplate(key, "references/"+name, metadata.SkillsTemplateExt)
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
// dir: 目录名（如 "patterns" 或 "examples"）
// category: 分类名（如 "api", "business"）
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

// RenderReferenceFile renders a standalone references/{name}.md template.
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

// RenderAgentMetadataFiles 渲染 agents/ 下所有可选元数据模板
func (l *Loader) RenderAgentMetadataFiles(data interface{}) ([]RenderedFile, error) {
	templatePaths, err := l.agentMetadataTemplatePaths()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	files := make([]RenderedFile, 0, len(templatePaths))
	outputPaths := make([]string, 0, len(templatePaths))
	for outputPath := range templatePaths {
		outputPaths = append(outputPaths, outputPath)
	}
	sort.Strings(outputPaths)

	for _, outputPath := range outputPaths {
		templatePath := templatePaths[outputPath]
		templateData, err := embedfs.FS.ReadFile(templatePath)
		if err != nil {
			return nil, err
		}

		tmpl, err := template.New(outputPath).Option("missingkey=error").Funcs(funcMap()).Parse(string(templateData))
		if err != nil {
			return nil, err
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}

		files = append(files, RenderedFile{
			Path:    "agents/" + outputPath,
			Content: buf.String(),
		})
	}

	return files, nil
}

func (l *Loader) agentMetadataTemplatePaths() (map[string]string, error) {
	selected := map[string]string{}
	var found bool
	var lastErr error

	for _, agentName := range l.templateAgentNames() {
		baseDir := metadata.SkillsAgentMetadataDir(agentName)
		perAgent := map[string]string{}

		err := fs.WalkDir(embedfs.FS, baseDir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
				return nil
			}

			rel := strings.TrimPrefix(path, baseDir+"/")
			outputPath, localized := metadataOutputPath(rel, l.locale)
			if outputPath == "" {
				return nil
			}
			if _, exists := perAgent[outputPath]; !exists || localized {
				perAgent[outputPath] = path
			}
			return nil
		})
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}

		for outputPath, templatePath := range perAgent {
			found = true
			if _, exists := selected[outputPath]; !exists {
				selected[outputPath] = templatePath
			}
		}
	}

	if found {
		return selected, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fs.ErrNotExist
}

func metadataOutputPath(templatePath, locale string) (string, bool) {
	if !strings.HasSuffix(templatePath, ".tmpl") {
		return "", false
	}
	withoutTemplateSuffix := strings.TrimSuffix(templatePath, ".tmpl")

	baseName := filepath.Base(withoutTemplateSuffix)
	parts := strings.Split(baseName, ".")
	if len(parts) >= 3 {
		localeCandidate := parts[len(parts)-2]
		if strings.Contains(localeCandidate, "-") {
			if localeCandidate != locale {
				return "", false
			}
			parts = append(parts[:len(parts)-2], parts[len(parts)-1])
			dir := filepath.Dir(withoutTemplateSuffix)
			outputName := strings.Join(parts, ".")
			if dir == "." {
				return outputName, true
			}
			return filepath.Join(dir, outputName), true
		}
	}
	return withoutTemplateSuffix, false
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
