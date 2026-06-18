package generator

import (
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
	return utils.ResolveProjectOutputPath(projectRoot, outputPath)
}

func configuredSkillOutputPath(projectRoot string, configRepo config.Reader) (string, error) {
	return utils.ConfiguredSkillOutputPath(projectRoot, configRepo)
}
