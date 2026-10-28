package generator

import (
	"context"

	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/utils/stringx"
)

func (s *GeneratorService) projectRoot(ctx context.Context) string {
	projectRoot := ""
	if s.configRepo != nil {
		projectRoot = s.configRepo.GetProjectConfig().RootPath
	}
	return stringx.FirstNonBlank(projectRoot, runtimecontext.ProjectRoot(ctx))
}

func (s *GeneratorService) resolveOutputPath(ctx context.Context, outputPath string) (string, error) {
	return projectpath.ResolveOutput(s.projectRoot(ctx), outputPath)
}
