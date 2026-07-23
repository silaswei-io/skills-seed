package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/skillgen"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

// ChildSkillTarget 是工作区子项目 Skill 的规范输出位置。
type ChildSkillTarget struct {
	OutputPath      string
	UsesChildConfig bool
	ConfigPath      string
}

// ResolveChildSkillTarget 根据子项目配置或工作区根配置解析唯一的 Skill 输出位置。
func ResolveChildSkillTarget(workspaceRoot string, project config.WorkspaceProjectConfig, rootConfig config.Reader) (ChildSkillTarget, error) {
	projectRoot, err := ResolveProjectRoot(workspaceRoot, project)
	if err != nil {
		return ChildSkillTarget{}, err
	}
	childConfig, exists, configPath, err := loadChildConfig(workspaceRoot, project, rootConfig)
	if err != nil {
		return ChildSkillTarget{}, err
	}
	configReader := rootConfig
	projectName := project.ID
	if exists {
		configReader = childConfig
		projectName = firstConfiguredName(childConfig.GetProjectConfig().Name, project.ID)
	}
	outputPath, err := utils.ConfiguredSkillOutputPath(projectRoot, configReader)
	if err != nil {
		return ChildSkillTarget{}, err
	}
	return ChildSkillTarget{
		OutputPath:      normalizeLegacySkillPath(outputPath, projectName),
		UsesChildConfig: exists,
		ConfigPath:      configPath,
	}, nil
}

func loadChildConfig(workspaceRoot string, project config.WorkspaceProjectConfig, rootConfig config.Reader) (config.Reader, bool, string, error) {
	configPath := filepath.Join(workspaceRoot, filepath.FromSlash(project.Path), ".skills-seed", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, configPath, nil
		}
		return nil, false, configPath, err
	}
	locale := config.DefaultToolLocale
	if rootConfig != nil {
		locale = rootConfig.GetToolLocale()
	}
	repo, err := config.NewRepository(filepath.Dir(configPath), locale)
	if err != nil {
		return nil, true, configPath, err
	}
	return repo, true, configPath, nil
}

func normalizeLegacySkillPath(outputPath, projectName string) string {
	if filepath.Base(filepath.Clean(outputPath)) != "skills-seed-skills" {
		return outputPath
	}
	return filepath.Join(filepath.Dir(outputPath), skillgen.GeneratedSkillName(projectName))
}

func firstConfiguredName(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "project"
}
