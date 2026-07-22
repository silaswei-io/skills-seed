package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/utils"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

func (g *WorkspaceGenerator) workspaceRootOutputPath(projectRoot, workspaceName string) (string, error) {
	return g.targetSkillOutputPath(projectRoot, skillgen.GeneratedWorkspaceSkillName(workspaceName))
}

func legacyWorkspaceSkillName(workspaceName string) string {
	name := skillgen.GeneratedSkillName(workspaceName)
	return strings.TrimSuffix(name, "-dev") + "-workspace"
}

func (g *WorkspaceGenerator) childSkillTarget(projectRoot string, project config.WorkspaceProjectConfig) (childSkillTarget, error) {
	projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
	if err != nil {
		return childSkillTarget{}, err
	}
	childConfig, exists, configPath, err := g.workspaceChildConfig(projectRoot, project)
	if err != nil {
		return childSkillTarget{}, err
	}
	if exists {
		outputPath, err := utils.ConfiguredSkillOutputPath(projectRootPath, childConfig)
		if err != nil {
			return childSkillTarget{}, err
		}
		outputPath = normalizeLegacyChildSkillPath(outputPath, firstNonEmpty(childConfig.GetProjectConfig().Name, project.ID))
		return childSkillTarget{
			OutputPath:      outputPath,
			UsesChildConfig: true,
			ConfigPath:      configPath,
		}, nil
	}
	outputPath, err := utils.ConfiguredSkillOutputPath(projectRootPath, g.configRepo)
	if err != nil {
		return childSkillTarget{}, err
	}
	outputPath = normalizeLegacyChildSkillPath(outputPath, project.ID)
	return childSkillTarget{
		OutputPath: outputPath,
		ConfigPath: configPath,
	}, nil
}

// normalizeLegacyChildSkillPath 将旧版通用目录迁移为当前项目的稳定生成名；显式自定义目录保持不变。
func normalizeLegacyChildSkillPath(outputPath, projectName string) string {
	if filepath.Base(filepath.Clean(outputPath)) != "skills-seed-skills" {
		return outputPath
	}
	return filepath.Join(filepath.Dir(outputPath), skillgen.GeneratedSkillName(projectName))
}

func (g *WorkspaceGenerator) targetSkillOutputPath(projectRoot, skillName string) (string, error) {
	resolvedOutputPath, err := utils.ConfiguredSkillOutputPath(projectRoot, g.configRepo)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(resolvedOutputPath), skillName), nil
}

func (g *WorkspaceGenerator) workspaceChildConfig(projectRoot string, project config.WorkspaceProjectConfig) (config.Reader, bool, string, error) {
	configPath := filepath.Join(projectRoot, filepath.FromSlash(project.Path), ".skills-seed", "config.yaml")
	_, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, configPath, nil
		}
		return nil, false, configPath, err
	}
	locale := config.DefaultToolLocale
	if g.configRepo != nil {
		locale = g.configRepo.GetToolLocale()
	}
	repo, err := config.NewRepository(filepath.Dir(configPath), locale)
	if err != nil {
		return nil, true, configPath, err
	}
	return repo, true, configPath, nil
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
