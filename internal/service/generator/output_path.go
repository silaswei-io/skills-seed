package generator

import "github.com/silaswei-io/skills-seed/internal/utils"

func (s *GeneratorService) resolveOutputPath(outputPath string) (string, error) {
	projectRoot := ""
	if s.configRepo != nil {
		projectRoot = s.configRepo.GetProjectConfig().RootPath
	}
	return utils.ResolveProjectOutputPath(projectRoot, outputPath)
}
