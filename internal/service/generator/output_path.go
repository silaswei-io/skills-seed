package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

func (s *GeneratorService) resolveOutputPath(outputPath string) (string, error) {
	projectRoot := ""
	if s.configRepo != nil {
		projectRoot = s.configRepo.GetProjectConfig().RootPath
	}
	return resolveProjectOutputPath(projectRoot, outputPath)
}

func resolveProjectOutputPath(projectRoot, outputPath string) (string, error) {
	resolvedPath, err := utils.ResolvePath(projectRoot, outputPath)
	if err != nil {
		return "", err
	}
	if projectRoot == "" {
		return filepath.Clean(resolvedPath), nil
	}

	rootAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", err
	}
	rootAbs = filepath.Clean(rootAbs)
	pathAbs = filepath.Clean(pathAbs)
	relPath, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%s", i18n.GetWithParams("GenerateOutputPathOutsideProjectRoot", map[string]interface{}{
			"OutputPath":  outputPath,
			"ProjectRoot": projectRoot,
		}))
	}
	return pathAbs, nil
}

func configuredSkillOutputPath(projectRoot string, configRepo config.Reader) (string, error) {
	target := ""
	outputPath := ""
	if configRepo != nil {
		target = configRepo.GetEffectiveSkillsTarget()
		outputPath = configRepo.GetEffectiveSkillsPath()
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = config.DefaultSkillsPathForTarget(target)
	}
	return resolveProjectOutputPath(projectRoot, outputPath)
}
