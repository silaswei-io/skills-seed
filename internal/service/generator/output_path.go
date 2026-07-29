package generator

import "github.com/silaswei-io/skills-seed/internal/projectpath"

func (s *GeneratorService) resolveOutputPath(outputPath string) (string, error) {
	projectRoot := ""
	if s.configRepo != nil {
		projectRoot = s.configRepo.GetProjectConfig().RootPath
	}
	return projectpath.ResolveOutput(projectRoot, outputPath)
}
