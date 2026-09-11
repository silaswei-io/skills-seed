// Package repositoryscopeconfig 将项目配置编译为仓库知识采集范围。
package repositoryscopeconfig

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/gitignore"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/repositoryscope"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

// KnowledgeScope 将项目配置编译为知识采集范围。
func KnowledgeScope(configRepo config.Reader, projectRoot string) repositoryscope.Scope {
	excludes := KnowledgeExcludes(configRepo, projectRoot)
	if configRepo == nil || !configRepo.GetExcludeConfig().GitIgnore {
		return repositoryscope.New(excludes)
	}
	matcher, err := gitignore.NewMatcher(context.Background(), projectRoot)
	if err != nil {
		return repositoryscope.New(excludes)
	}
	return repositoryscope.NewWithGitIgnore(excludes, matcher)
}

// DefaultKnowledgeScope 返回使用默认配置的独立源码发现范围。
func DefaultKnowledgeScope() repositoryscope.Scope {
	return repositoryscope.New(config.LearnExcludePatterns(config.DefaultExcludePatterns(), nil))
}

// KnowledgeExcludes 合并配置排除规则和已生成 Skills 输出目录。
func KnowledgeExcludes(configRepo config.Reader, projectRoot string) []string {
	var excludes []string
	if configRepo != nil {
		excludes = append(excludes, configRepo.GetExclude()...)
	}
	return config.LearnExcludePatterns(excludes, GeneratedSkillDirs(configRepo, projectRoot))
}

// GeneratedSkillDirs 返回当前项目及子项目配置声明的 Skills 输出目录。
func GeneratedSkillDirs(configRepo config.Reader, projectRoot string) []string {
	dirs := make([]string, 0)
	readers := []config.Reader{}
	if configRepo != nil {
		readers = append(readers, configRepo)
	}
	childSeedPath := filepath.Join(projectRoot, ".skills-seed")
	if _, err := os.Stat(filepath.Join(childSeedPath, "config.yaml")); err == nil {
		if childRepo, err := config.NewRepository(childSeedPath, ""); err == nil {
			readers = append(readers, childRepo)
		}
	}
	for _, reader := range readers {
		for _, outputPath := range reader.GetSkillsConfig().Paths {
			if outputPath == "" {
				continue
			}
			resolved, err := projectpath.ResolveOutput(projectRoot, outputPath)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(projectRoot, resolved)
			if err == nil {
				dirs = append(dirs, filepath.ToSlash(rel))
			}
		}
	}
	sort.Strings(dirs)
	return stringx.UniqueNonEmpty(dirs)
}
