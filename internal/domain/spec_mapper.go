package domain

import "time"

// WorkspaceProjectOverride 描述工作区子项目覆盖字段，
// 用于 NewProjectSpecFromProfile 中避免 domain → config 依赖。
type WorkspaceProjectOverride struct {
	ID       string
	Path     string
	Type     string
	Language string
}

// NewProjectSpecFromProfile 根据画像构建权威规则投影。
// project 参数为可选的工作区子项目覆盖。
func NewProjectSpecFromProfile(profile *ProjectProfile, project WorkspaceProjectOverride) *ProjectSpec {
	if profile == nil {
		return nil
	}
	profile = CleanProjectProfile(profile)

	spec := &ProjectSpec{
		ProjectName:       profile.ProjectName,
		Language:          profile.Language,
		EngineeringRules:  append([]EngineeringRule(nil), profile.EngineeringRules...),
		AuthorityCoverage: append([]AuthorityCoverage(nil), profile.AuthorityCoverage...),
		GeneratedAt:       time.Now().Format("2006-01-02 15:04:05"),
	}
	if project.ID != "" {
		spec.ProjectID = project.ID
		spec.ProjectName = project.ID
		spec.ScopePath = project.Path
		spec.WorkspaceRole = project.Type
		if project.Language != "" {
			spec.Language = project.Language
		}
	}
	if spec.ProjectName == "" {
		spec.ProjectName = "project"
	}
	if spec.Language == "" {
		spec.Language = "unknown"
	}

	return spec
}
