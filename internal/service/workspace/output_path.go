package workspace

import (
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
)

func (g *WorkspaceGenerator) workspaceRootOutputPath(projectRoot, workspaceName string) (string, error) {
	name := skillgen.ConfiguredSkillName(config.ProjectConfig{Name: workspaceName, Mode: domain.ModeWorkspace}, g.configRepo.GetSkillsConfig())
	return g.targetSkillOutputPath(projectRoot, name)
}

func (g *WorkspaceGenerator) targetSkillOutputPath(projectRoot, skillName string) (string, error) {
	resolvedOutputPath, err := projectpath.ConfiguredSkillOutput(projectRoot, g.configRepo)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(resolvedOutputPath), skillName), nil
}

func relativeWorkspacePath(workspaceRoot, path string) string {
	if workspaceRoot == "" {
		return path
	}
	rel, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return path
	}
	return rel
}
