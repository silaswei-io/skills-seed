package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
)

const (
	// ProgramVersion 是 `skills-seed --version` 展示的 CLI 版本
	ProgramVersion = "v0.8.1"

	UnavailableHash = "unavailable"

	PromptTemplatesRoot = "templates/prompts"
	SkillsTemplatesRoot = "templates/skills"

	CommonTemplateProvider    = "common"
	ProjectTemplateProvider   = "project"
	WorkspaceTemplateProvider = "workspace"

	PromptTemplateExt        = ".txt.tmpl"
	ProjectPromptTemplateExt = ".md.tmpl"
	SkillsTemplateExt        = ".md.tmpl"
	GenericTemplateExt       = ".tmpl"
)

// TemplateProviderFallbacks 返回 provider 模板查找顺序
func TemplateProviderFallbacks(provider string) []string {
	var providers []string
	seen := make(map[string]bool)
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		providers = append(providers, name)
	}

	add(provider)
	add(CommonTemplateProvider)
	return providers
}

// PromptTemplatePath 返回提示词模板路径
func PromptTemplatePath(provider, name, locale string) string {
	fileName := name + PromptTemplateExt
	if templateLocaleSuffix(locale) != "" {
		fileName = name + "." + templateLocaleSuffix(locale) + PromptTemplateExt
	}
	return filepath.ToSlash(filepath.Join(PromptTemplatesRoot, provider, fileName))
}

// ProjectPromptTemplatePath 返回项目提示词模板路径
func ProjectPromptTemplatePath(name, locale string) string {
	fileName := name + ProjectPromptTemplateExt
	if templateLocaleSuffix(locale) != "" {
		fileName = name + "." + templateLocaleSuffix(locale) + ProjectPromptTemplateExt
	}
	return filepath.ToSlash(filepath.Join(PromptTemplatesRoot, ProjectTemplateProvider, fileName))
}

// WorkspacePromptTemplatePath 返回工作区提示词模板路径
func WorkspacePromptTemplatePath(name, locale string) string {
	fileName := name + ProjectPromptTemplateExt
	if templateLocaleSuffix(locale) != "" {
		fileName = name + "." + templateLocaleSuffix(locale) + ProjectPromptTemplateExt
	}
	return filepath.ToSlash(filepath.Join(PromptTemplatesRoot, WorkspaceTemplateProvider, fileName))
}

// SkillsTemplatePath 返回 Skills 模板路径
func SkillsTemplatePath(provider, relativeName, locale, ext string) string {
	if ext == "" {
		ext = SkillsTemplateExt
	}
	fileName := relativeName + ext
	if templateLocaleSuffix(locale) != "" {
		fileName = relativeName + "." + templateLocaleSuffix(locale) + ext
	}
	return filepath.ToSlash(filepath.Join(SkillsTemplatesRoot, provider, fileName))
}

func templateLocaleSuffix(locale string) string {
	return config.TemplateLocaleSuffix(locale)
}

// SkillsAgentMetadataDir 返回 Agent 元数据模板目录
func SkillsAgentMetadataDir(provider string) string {
	return filepath.ToSlash(filepath.Join(SkillsTemplatesRoot, provider, "agents"))
}

// PromptTemplatesHash 计算提示词模板树哈希
func PromptTemplatesHash(fsys fs.FS) (string, error) {
	return EmbeddedTreeHash(fsys, PromptTemplatesRoot)
}

// SkillsTemplatesHash 计算 Skills 模板树哈希
func SkillsTemplatesHash(fsys fs.FS) (string, error) {
	return EmbeddedTreeHash(fsys, SkillsTemplatesRoot)
}

// EmbeddedTreeHash 计算嵌入文件树哈希
func EmbeddedTreeHash(fsys fs.FS, root string) (string, error) {
	var paths []string
	if err := fs.WalkDir(fsys, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		paths = append(paths, filepath.ToSlash(path))
		return nil
	}); err != nil {
		return "", err
	}

	sort.Strings(paths)

	hash := sha256.New()
	for _, path := range paths {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return "", err
		}
		hash.Write([]byte(path))
		hash.Write([]byte{0})
		hash.Write(data)
		hash.Write([]byte{0})
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HashOrUnavailable 返回哈希或不可用占位值
func HashOrUnavailable(hash string, err error) string {
	if err != nil || hash == "" {
		return UnavailableHash
	}
	return hash
}
