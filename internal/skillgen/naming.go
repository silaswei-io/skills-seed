package skillgen

import "strings"

// GeneratedSkillName 把项目名规整为 skills-seed 生成 skill 使用的稳定目录名。
func GeneratedSkillName(projectName string) string {
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

// GeneratedWorkspaceSkillName 把工作区名规整为根 workspace skill 使用的稳定目录名。
func GeneratedWorkspaceSkillName(workspaceName string) string {
	name := GeneratedSkillName(workspaceName)
	if strings.HasSuffix(name, "-workspace-dev") {
		return name
	}
	if strings.HasSuffix(name, "-dev") {
		base := strings.TrimSuffix(name, "-dev")
		if strings.HasSuffix(base, "-workspace") {
			return base + "-dev"
		}
		return base + "-workspace-dev"
	}
	return name + "-workspace-dev"
}
