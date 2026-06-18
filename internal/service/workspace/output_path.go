package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/utils"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

func (g *WorkspaceGenerator) workspaceRootOutputPath(projectRoot, workspaceName string) (string, error) {
	return g.targetSkillOutputPath(projectRoot, workspaceSkillName(workspaceName))
}

func workspaceSkillName(workspaceName string) string {
	name := generatedSkillName(workspaceName)
	if strings.HasSuffix(name, "-dev") {
		return strings.TrimSuffix(name, "-dev") + "-workspace"
	}
	return name + "-workspace"
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
		outputPath, err := configuredSkillOutputPath(projectRootPath, childConfig)
		if err != nil {
			return childSkillTarget{}, err
		}
		return childSkillTarget{
			OutputPath:      outputPath,
			UsesChildConfig: true,
			ConfigPath:      configPath,
		}, nil
	}
	outputPath, err := configuredSkillOutputPath(projectRootPath, g.configRepo)
	if err != nil {
		return childSkillTarget{}, err
	}
	return childSkillTarget{
		OutputPath: outputPath,
		ConfigPath: configPath,
	}, nil
}

func configuredSkillOutputPath(projectRoot string, configRepo config.Reader) (string, error) {
	return utils.ConfiguredSkillOutputPath(projectRoot, configRepo)
}

func (g *WorkspaceGenerator) targetSkillOutputPath(projectRoot, skillName string) (string, error) {
	target := ""
	outputPath := ""
	if g.configRepo != nil {
		target = g.configRepo.GetEffectiveSkillsTarget()
		outputPath = g.configRepo.GetEffectiveSkillsPath()
	}
	if strings.TrimSpace(outputPath) == "" {
		outputPath = config.DefaultSkillsPathForTarget(target)
	}
	resolvedOutputPath, err := resolveProjectOutputPath(projectRoot, outputPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(resolvedOutputPath), skillName), nil
}

func resolveProjectOutputPath(projectRoot, outputPath string) (string, error) {
	return utils.ResolveProjectOutputPath(projectRoot, outputPath)
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

func generatedSkillName(projectName string) string {
	var b strings.Builder
	previousHyphen := false
	for _, r := range strings.ToLower(projectName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousHyphen = false
			continue
		}
		if !previousHyphen {
			b.WriteRune('-')
			previousHyphen = true
		}
	}

	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "project"
	}
	if !strings.HasSuffix(name, "-dev") {
		name += "-dev"
	}
	return name
}
