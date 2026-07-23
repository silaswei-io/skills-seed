package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
)

const (
	workspaceRuleSourceSystem  = "system"
	workspaceRuleSourceProfile = "workspace_profile"
	workspaceRuleSourceUser    = "user_context"
)

// AssembleProfile 使用配置画像作为身份真源，只合并 AI 拥有的事实和关系字段。
func AssembleProfile(base, analyzed *domain.WorkspaceProfile) (*domain.WorkspaceProfile, error) {
	return assembleProfile(base, analyzed, false)
}

// ReconcileProfile 校验已持久化画像仍与当前配置身份完全一致。
func ReconcileProfile(base, stored *domain.WorkspaceProfile) (*domain.WorkspaceProfile, error) {
	return assembleProfile(base, stored, true)
}

func assembleProfile(base, analyzed *domain.WorkspaceProfile, strictIdentity bool) (*domain.WorkspaceProfile, error) {
	if base == nil {
		return nil, fmt.Errorf("workspace base profile is required")
	}
	if analyzed == nil {
		analyzed = &domain.WorkspaceProfile{}
	}

	issues := &validationIssues{}
	projects, projectSet := assembleProjects(base.Projects, analyzed.Projects, strictIdentity, issues)
	if strictIdentity {
		if analyzed.Name != base.Name {
			issues.add("name", "stored value %q does not match configured workspace %q", analyzed.Name, base.Name)
		}
		if filepath.Clean(analyzed.RootPath) != filepath.Clean(base.RootPath) {
			issues.add("root_path", "stored value %q does not match configured root %q", analyzed.RootPath, base.RootPath)
		}
	}

	profile := &domain.WorkspaceProfile{
		Name:         base.Name,
		RootPath:     base.RootPath,
		Summary:      strings.TrimSpace(analyzed.Summary),
		Projects:     projects,
		Shared:       cleanWorkspacePaths(analyzed.Shared),
		Contracts:    cleanWorkspacePaths(analyzed.Contracts),
		Infra:        cleanWorkspacePaths(analyzed.Infra),
		Dependencies: cleanWorkspaceDependencies(analyzed.Dependencies),
		ImpactRoutes: cleanWorkspaceRoutes(analyzed.ImpactRoutes),
		GeneratedAt:  analyzed.GeneratedAt,
	}
	validateProfile(profile, projectSet, issues)
	if err := issues.err(); err != nil {
		return nil, err
	}
	return profile, nil
}

func assembleProjects(base, analyzed []domain.WorkspaceProject, strict bool, issues *validationIssues) ([]domain.WorkspaceProject, map[string]domain.WorkspaceProject) {
	known := make(map[string]domain.WorkspaceProject, len(base))
	result := make([]domain.WorkspaceProject, 0, len(base))
	for i, project := range base {
		project.ID = strings.TrimSpace(project.ID)
		project.Path = cleanRelativePath(project.Path)
		project.Type = strings.TrimSpace(project.Type)
		project.Language = strings.TrimSpace(project.Language)
		if project.ID == "" {
			issues.add(fmt.Sprintf("projects[%d].id", i), "configured project id is empty")
			continue
		}
		if _, exists := known[project.ID]; exists {
			issues.add(fmt.Sprintf("projects[%d].id", i), "duplicate configured project %q", project.ID)
			continue
		}
		known[project.ID] = project
		result = append(result, project)
	}

	if strict && len(analyzed) != len(result) {
		issues.add("projects", "stored project count %d does not match configured count %d", len(analyzed), len(result))
	}
	seen := make(map[string]bool, len(analyzed))
	for i, candidate := range analyzed {
		id := strings.TrimSpace(candidate.ID)
		configured, ok := known[id]
		if !ok {
			issues.add(fmt.Sprintf("projects[%d].id", i), "unknown project %q", id)
			continue
		}
		if seen[id] {
			issues.add(fmt.Sprintf("projects[%d].id", i), "duplicate project analysis %q", id)
			continue
		}
		seen[id] = true
		if strict {
			if i >= len(result) || result[i].ID != id {
				issues.add(fmt.Sprintf("projects[%d].id", i), "stored project order does not match configuration")
			}
			if cleanRelativePath(candidate.Path) != configured.Path || strings.TrimSpace(candidate.Type) != configured.Type || strings.TrimSpace(candidate.Language) != configured.Language {
				issues.add(fmt.Sprintf("projects[%d]", i), "stored identity for %q does not match configuration", id)
			}
		}
		for j := range result {
			if result[j].ID != id {
				continue
			}
			result[j].Responsibility = strings.TrimSpace(candidate.Responsibility)
			result[j].Frameworks = cleanStrings(candidate.Frameworks)
			known[id] = result[j]
			break
		}
	}
	return result, known
}

// AssembleSpec 合并默认规范与 AI 候选规范，并根据可信来源确定规则权威级别。
func AssembleSpec(base, analyzed *domain.WorkspaceSpec, profile *domain.WorkspaceProfile, opts ValidationOptions) (*domain.WorkspaceSpec, error) {
	return assembleSpec(base, analyzed, profile, opts, false)
}

func assembleSpec(base, analyzed *domain.WorkspaceSpec, profile *domain.WorkspaceProfile, opts ValidationOptions, persisted bool) (*domain.WorkspaceSpec, error) {
	if base == nil {
		base = &domain.WorkspaceSpec{}
	}
	if analyzed == nil {
		analyzed = &domain.WorkspaceSpec{}
	}
	spec := &domain.WorkspaceSpec{
		Routing:                mergeRoutes(base.Routing, analyzed.Routing),
		Rules:                  mergeRules(base.Rules, analyzed.Rules),
		ChangeOrder:            cleanStrings(analyzed.ChangeOrder),
		ParallelAgentGuidance:  cleanParallelGuidance(analyzed.ParallelAgentGuidance),
		LoadMultipleSkillsWhen: cleanLoadMultiple(analyzed.LoadMultipleSkillsWhen),
		GeneratedAt:            analyzed.GeneratedAt,
	}
	if spec.GeneratedAt == "" {
		spec.GeneratedAt = base.GeneratedAt
	}

	issues := &validationIssues{}
	validateSpec(spec, profile, opts, persisted, issues)
	if err := issues.err(); err != nil {
		return nil, err
	}
	return spec, nil
}

// ReconcileSpec 重新校验持久化规范，不信任其中保存的 authority 字段。
func ReconcileSpec(base, stored *domain.WorkspaceSpec, profile *domain.WorkspaceProfile, opts ValidationOptions) (*domain.WorkspaceSpec, error) {
	return assembleSpec(base, stored, profile, opts, true)
}

// SpecFromProfile 根据工作区画像生成指定语言的保守路由规则和跨项目规则。
func SpecFromProfile(profile *domain.WorkspaceProfile, locale string) *domain.WorkspaceSpec {
	if profile == nil {
		return &domain.WorkspaceSpec{}
	}
	routing := make([]domain.WorkspaceRoute, 0, len(profile.Projects)+len(profile.Shared)+len(profile.Contracts)+len(profile.Infra)+len(profile.ImpactRoutes))
	for _, project := range profile.Projects {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(project.Path, "**")),
			ProjectIDs:  []string{project.ID},
			Reason:      i18n.GetForLocale(locale, "WorkspaceRouteProjectReason"),
		})
	}
	routing = append(routing, profile.ImpactRoutes...)
	projectIDs := ProjectIDs(profile.Projects)
	for _, path := range profile.Contracts {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(append(append([]string{}, path.Producers...), path.Consumers...), projectIDs),
			Reason:      firstNonEmpty(path.Description, i18n.GetForLocale(locale, "WorkspaceRouteContractReason")),
		})
	}
	for _, path := range profile.Shared {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(path.Consumers, projectIDs),
			Reason:      firstNonEmpty(path.Description, i18n.GetForLocale(locale, "WorkspaceRouteSharedReason")),
		})
	}
	for _, path := range profile.Infra {
		routing = append(routing, domain.WorkspaceRoute{
			PathPattern: filepath.ToSlash(filepath.Join(path.Path, "**")),
			ProjectIDs:  nonEmptyStrings(path.AffectedProjects, projectIDs),
			Reason:      firstNonEmpty(path.Description, i18n.GetForLocale(locale, "WorkspaceRouteInfraReason")),
		})
	}
	refs := make([]domain.WorkspaceReference, 0, len(projectIDs))
	for _, id := range projectIDs {
		refs = append(refs, domain.WorkspaceReference{Kind: domain.WorkspaceReferenceProject, Value: id})
	}
	return &domain.WorkspaceSpec{
		Routing: routing,
		Rules: []domain.WorkspaceRule{{
			Title:       i18n.GetForLocale(locale, "WorkspaceRuleBoundaryTitle"),
			Description: i18n.GetForLocale(locale, "WorkspaceRuleBoundaryDescription"),
			AppliesTo:   refs,
			Source:      workspaceRuleSourceSystem,
			Authority:   domain.WorkspaceRuleAuthoritySystem,
		}},
	}
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

func nonEmptyStrings(values, fallback []string) []string {
	result := cleanStrings(values)
	if len(result) > 0 {
		return result
	}
	return fallback
}
