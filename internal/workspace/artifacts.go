package workspace

import (
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

// MergeProfile 将 AI 分析出的工作区事实覆盖到由配置推导出的基础画像上。
func MergeProfile(base, analyzed *domain.WorkspaceProfile) *domain.WorkspaceProfile {
	if base == nil {
		base = &domain.WorkspaceProfile{}
	}
	if analyzed == nil {
		return base
	}
	if analyzed.Name != "" {
		base.Name = analyzed.Name
	}
	if analyzed.RootPath != "" {
		base.RootPath = analyzed.RootPath
	}
	base.Summary = analyzed.Summary
	base.Projects = mergeWorkspaceProjects(base.Projects, analyzed.Projects)
	base.Shared = ChoosePaths(base.Shared, analyzed.Shared)
	base.Contracts = ChoosePaths(base.Contracts, analyzed.Contracts)
	base.Infra = ChoosePaths(base.Infra, analyzed.Infra)
	base.Dependencies = analyzed.Dependencies
	base.ImpactRoutes = analyzed.ImpactRoutes
	base.GeneratedAt = analyzed.GeneratedAt
	return base
}

// MergeSpec 将 AI 分析出的工作区规则覆盖到由画像推导出的默认规范上。
func MergeSpec(base, analyzed *domain.WorkspaceSpec) *domain.WorkspaceSpec {
	if base == nil {
		base = &domain.WorkspaceSpec{}
	}
	if analyzed == nil {
		return base
	}
	if analyzed.Name != "" {
		base.Name = analyzed.Name
	}
	if analyzed.RootPath != "" {
		base.RootPath = analyzed.RootPath
	}
	if len(analyzed.Projects) > 0 {
		base.Projects = analyzed.Projects
	}
	if len(analyzed.Routing) > 0 {
		base.Routing = analyzed.Routing
	}
	if len(analyzed.Rules) > 0 {
		base.Rules = analyzed.Rules
	}
	base.ChangeOrder = analyzed.ChangeOrder
	base.ParallelAgentGuidance = analyzed.ParallelAgentGuidance
	base.LoadMultipleSkillsWhen = analyzed.LoadMultipleSkillsWhen
	if analyzed.GeneratedAt != "" {
		base.GeneratedAt = analyzed.GeneratedAt
	}
	return base
}

// SpecFromProfile 根据工作区画像生成保守的路由规则和跨项目规则。
func SpecFromProfile(profile *domain.WorkspaceProfile) *domain.WorkspaceSpec {
	if profile == nil {
		return &domain.WorkspaceSpec{}
	}
	routing := make([]domain.WorkspaceRoute, 0, len(profile.Projects)+len(profile.Shared)+len(profile.Contracts)+len(profile.Infra)+len(profile.ImpactRoutes))
	for _, project := range profile.Projects {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(project.Path, "**")),
			ProjectIDs:  []string{project.ID},
			Reason:      "子项目路径只路由到该子项目的独立 skill",
		})
	}
	routing = append(routing, profile.ImpactRoutes...)
	projectIDs := ProjectIDs(profile.Projects)
	for _, path := range profile.Contracts {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(append(append([]string{}, path.Producers...), path.Consumers...), projectIDs),
			Reason:      firstNonEmpty(path.Description, "契约路径变更需要检查生产者、消费者和生成物"),
		})
	}
	for _, path := range profile.Shared {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(path.Consumers, projectIDs),
			Reason:      firstNonEmpty(path.Description, "共享代码变更需要检查所有导入方或复用方"),
		})
	}
	for _, path := range profile.Infra {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(path.AffectedProjects, projectIDs),
			Reason:      firstNonEmpty(path.Description, "基础设施变更需要检查受部署或运行时配置影响的子项目"),
		})
	}
	return &domain.WorkspaceSpec{
		Name:     profile.Name,
		RootPath: profile.RootPath,
		Projects: profile.Projects,
		Routing:  routing,
		Rules: []domain.WorkspaceRule{
			{
				Title:       "跨项目改动先定边界",
				Description: "修改契约、共享代码或基础设施前，先确认受影响子项目并读取对应 skill。",
				AppliesTo:   projectIDs,
			},
		},
	}
}

// ChoosePaths 优先使用分析结果中的路径；分析结果为空时保留基础路径。
func ChoosePaths(base, analyzed []domain.WorkspacePath) []domain.WorkspacePath {
	if len(analyzed) > 0 {
		return analyzed
	}
	return base
}

// ProjectIDs 按原顺序返回非空的工作区子项目 ID。
func ProjectIDs(projects []domain.WorkspaceProject) []string {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project.ID != "" {
			ids = append(ids, project.ID)
		}
	}
	return ids
}

func mergeWorkspaceProjects(base, analyzed []domain.WorkspaceProject) []domain.WorkspaceProject {
	if len(base) == 0 {
		return analyzed
	}
	byID := make(map[string]domain.WorkspaceProject, len(analyzed))
	for _, project := range analyzed {
		byID[project.ID] = project
	}
	result := make([]domain.WorkspaceProject, 0, len(base))
	for _, project := range base {
		if analyzedProject, ok := byID[project.ID]; ok {
			if analyzedProject.Path != "" {
				project.Path = analyzedProject.Path
			}
			if analyzedProject.Type != "" {
				project.Type = analyzedProject.Type
			}
			if analyzedProject.Language != "" {
				project.Language = analyzedProject.Language
			}
			project.Responsibility = analyzedProject.Responsibility
			project.Frameworks = analyzedProject.Frameworks
		}
		result = append(result, project)
	}
	return result
}

func nonEmptyStrings(values, fallback []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	if len(result) > 0 {
		return result
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
