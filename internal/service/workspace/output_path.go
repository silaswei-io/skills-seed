package workspace

import (
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

func (g *WorkspaceGenerator) workspaceRootOutputPath(projectRoot, workspaceName string) (string, error) {
	return g.targetSkillOutputPath(projectRoot, skillgen.GeneratedWorkspaceSkillName(workspaceName))
}

func legacyWorkspaceSkillName(workspaceName string) string {
	name := skillgen.GeneratedSkillName(workspaceName)
	return strings.TrimSuffix(name, "-dev") + "-workspace"
}

func (g *WorkspaceGenerator) targetSkillOutputPath(projectRoot, skillName string) (string, error) {
	resolvedOutputPath, err := utils.ConfiguredSkillOutputPath(projectRoot, g.configRepo)
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
