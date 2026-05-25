package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// ProgramVersion is the CLI version shown by `skills-seed --version`.
	ProgramVersion = "v0.0.1"

	UnavailableHash = "unavailable"

	PromptTemplatesRoot = "templates/prompts"
	SkillsTemplatesRoot = "templates/skills"

	CommonTemplateProvider  = "common"
	ProjectTemplateProvider = "project"

	PromptTemplateExt        = ".txt.tmpl"
	ProjectPromptTemplateExt = ".md.tmpl"
	SkillsTemplateExt        = ".md.tmpl"
	GenericTemplateExt       = ".tmpl"
)

// TemplateProviderFallbacks returns provider-specific lookup order.
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

func PromptTemplatePath(provider, name, locale string) string {
	fileName := name + PromptTemplateExt
	if locale != "" {
		fileName = name + "." + locale + PromptTemplateExt
	}
	return filepath.ToSlash(filepath.Join(PromptTemplatesRoot, provider, fileName))
}

func ProjectPromptTemplatePath(name, locale string) string {
	fileName := name + ProjectPromptTemplateExt
	if locale != "" {
		fileName = name + "." + locale + ProjectPromptTemplateExt
	}
	return filepath.ToSlash(filepath.Join(PromptTemplatesRoot, ProjectTemplateProvider, fileName))
}

func SkillsTemplatePath(provider, relativeName, locale, ext string) string {
	if ext == "" {
		ext = SkillsTemplateExt
	}
	fileName := relativeName + ext
	if locale != "" {
		fileName = relativeName + "." + locale + ext
	}
	return filepath.ToSlash(filepath.Join(SkillsTemplatesRoot, provider, fileName))
}

func SkillsAgentMetadataDir(provider string) string {
	return filepath.ToSlash(filepath.Join(SkillsTemplatesRoot, provider, "agents"))
}

func PromptTemplatesHash(fsys fs.FS) (string, error) {
	return EmbeddedTreeHash(fsys, PromptTemplatesRoot)
}

func SkillsTemplatesHash(fsys fs.FS) (string, error) {
	return EmbeddedTreeHash(fsys, SkillsTemplatesRoot)
}

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

func HashOrUnavailable(hash string, err error) string {
	if err != nil || hash == "" {
		return UnavailableHash
	}
	return hash
}
