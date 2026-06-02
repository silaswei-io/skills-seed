package commandutil

import "github.com/silaswei-io/skills-seed/internal/infra/config"

func WorkspaceProjectProgressNames(projects []config.WorkspaceProjectConfig) []string {
	names := make([]string, 0, len(projects))
	for _, project := range projects {
		names = append(names, WorkspaceProjectProgressName(project))
	}
	return names
}

func WorkspaceProjectProgressName(project config.WorkspaceProjectConfig) string {
	if project.ID != "" {
		return project.ID
	}
	return project.Path
}
