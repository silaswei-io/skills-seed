package parser

import (
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

// ParsePlanAnalysisUnitsResult 解析业务分析单元规划结果。
func ParsePlanAnalysisUnitsResult(output string) (*agent.PlanAnalysisUnitsResult, error) {
	var payload aicontract.PlanAnalysisUnitsOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}
	return &agent.PlanAnalysisUnitsResult{Units: analysisUnitsToDomain(payload.Units)}, nil
}

// ParseWorkspaceProfile 解析工作区画像结果。
func ParseWorkspaceProfile(output string) (*domain.WorkspaceProfile, error) {
	var payload aicontract.WorkspaceProfileOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}
	profile := workspaceProfileToDomain(payload)
	if profile.Projects == nil {
		profile.Projects = []domain.WorkspaceProject{}
	}
	if profile.Shared == nil {
		profile.Shared = []domain.WorkspacePath{}
	}
	if profile.Contracts == nil {
		profile.Contracts = []domain.WorkspacePath{}
	}
	if profile.Infra == nil {
		profile.Infra = []domain.WorkspacePath{}
	}
	if profile.Dependencies == nil {
		profile.Dependencies = []domain.WorkspaceDependency{}
	}
	if profile.ImpactRoutes == nil {
		profile.ImpactRoutes = []domain.WorkspaceRoute{}
	}
	return &profile, nil
}

// ParseWorkspaceSpec 解析工作区开发规范结果。
func ParseWorkspaceSpec(output string) (*domain.WorkspaceSpec, error) {
	var payload aicontract.WorkspaceSpecOutput
	if err := parseJSONPayload(output, &payload); err != nil {
		return nil, err
	}
	spec := workspaceSpecToDomain(payload)
	if spec.Routing == nil {
		spec.Routing = []domain.WorkspaceRoute{}
	}
	if spec.Rules == nil {
		spec.Rules = []domain.WorkspaceRule{}
	}
	if spec.ChangeOrder == nil {
		spec.ChangeOrder = []string{}
	}
	if spec.ParallelAgentGuidance == nil {
		spec.ParallelAgentGuidance = []domain.WorkspaceParallelGuidance{}
	}
	if spec.LoadMultipleSkillsWhen == nil {
		spec.LoadMultipleSkillsWhen = []domain.WorkspaceLoadMultipleSkill{}
	}
	return &spec, nil
}

func analysisUnitsToDomain(units []aicontract.AnalysisUnitOutput) []domain.AnalysisUnit {
	out := make([]domain.AnalysisUnit, len(units))
	for i, unit := range units {
		out[i] = domain.AnalysisUnit{
			ID:           unit.ID,
			Name:         unit.Name,
			RouteTerms:   stringsOrEmpty(unit.RouteTerms),
			EntryPaths:   stringsOrEmpty(unit.EntryPaths),
			RelatedPaths: stringsOrEmpty(unit.RelatedPaths),
			ScopeReason:  unit.ScopeReason,
		}
	}
	return out
}

func workspaceProfileToDomain(profile aicontract.WorkspaceProfileOutput) domain.WorkspaceProfile {
	return domain.WorkspaceProfile{
		Summary:      profile.Summary,
		Projects:     workspaceProjectAnalysesToDomain(profile.Projects),
		Shared:       workspacePathsToDomain(profile.Shared),
		Contracts:    workspacePathsToDomain(profile.Contracts),
		Infra:        workspacePathsToDomain(profile.Infra),
		Dependencies: workspaceDependenciesToDomain(profile.Dependencies),
		ImpactRoutes: workspaceRoutesToDomain(profile.ImpactRoutes),
	}
}

func workspaceSpecToDomain(spec aicontract.WorkspaceSpecOutput) domain.WorkspaceSpec {
	return domain.WorkspaceSpec{
		Routing:                workspaceRoutesToDomain(spec.Routing),
		Rules:                  workspaceRulesToDomain(spec.Rules),
		ChangeOrder:            stringsOrEmpty(spec.ChangeOrder),
		ParallelAgentGuidance:  workspaceParallelGuidanceToDomain(spec.ParallelAgentGuidance),
		LoadMultipleSkillsWhen: workspaceLoadMultipleSkillsToDomain(spec.LoadMultipleSkillsWhen),
	}
}

func workspaceProjectAnalysesToDomain(projects []aicontract.WorkspaceProjectAnalysisOutput) []domain.WorkspaceProject {
	out := make([]domain.WorkspaceProject, len(projects))
	for i, project := range projects {
		out[i] = domain.WorkspaceProject{
			ID:             project.ProjectID,
			Responsibility: project.Responsibility,
			Frameworks:     stringsOrEmpty(project.Frameworks),
		}
	}
	return out
}

func workspacePathsToDomain(paths []aicontract.WorkspacePathOutput) []domain.WorkspacePath {
	out := make([]domain.WorkspacePath, len(paths))
	for i, path := range paths {
		out[i] = domain.WorkspacePath{
			Path:             path.Path,
			Description:      path.Description,
			Consumers:        stringsOrEmpty(path.Consumers),
			Producers:        stringsOrEmpty(path.Producers),
			AffectedProjects: stringsOrEmpty(path.AffectedProjects),
		}
	}
	return out
}

func workspaceDependenciesToDomain(dependencies []aicontract.WorkspaceDependencyOutput) []domain.WorkspaceDependency {
	out := make([]domain.WorkspaceDependency, len(dependencies))
	for i, dependency := range dependencies {
		out[i] = domain.WorkspaceDependency{
			FromProjectID: dependency.FromProjectID,
			To:            workspaceReferenceToDomain(dependency.To),
			Reason:        dependency.Reason,
		}
	}
	return out
}

func workspaceRoutesToDomain(routes []aicontract.WorkspaceRouteOutput) []domain.WorkspaceRoute {
	out := make([]domain.WorkspaceRoute, len(routes))
	for i, route := range routes {
		out[i] = domain.WorkspaceRoute{
			PathPattern: route.PathPattern,
			ProjectIDs:  stringsOrEmpty(route.ProjectIDs),
			Reason:      route.Reason,
		}
	}
	return out
}

func workspaceRulesToDomain(rules []aicontract.WorkspaceRuleOutput) []domain.WorkspaceRule {
	out := make([]domain.WorkspaceRule, len(rules))
	for i, rule := range rules {
		out[i] = domain.WorkspaceRule{
			Title:       rule.Title,
			Description: rule.Description,
			AppliesTo:   workspaceReferencesToDomain(rule.AppliesTo),
			Source:      rule.Source,
			Evidence:    stringsOrEmpty(rule.Evidence),
		}
	}
	return out
}

func workspaceParallelGuidanceToDomain(items []aicontract.WorkspaceParallelGuidanceOutput) []domain.WorkspaceParallelGuidance {
	out := make([]domain.WorkspaceParallelGuidance, len(items))
	for i, item := range items {
		out[i] = domain.WorkspaceParallelGuidance{
			Scope:     workspaceReferenceToDomain(item.Scope),
			Allowed:   item.Allowed,
			Condition: item.Condition,
		}
	}
	return out
}

func workspaceReferencesToDomain(refs []aicontract.WorkspaceReferenceOutput) []domain.WorkspaceReference {
	out := make([]domain.WorkspaceReference, len(refs))
	for i, ref := range refs {
		out[i] = workspaceReferenceToDomain(ref)
	}
	return out
}

func workspaceReferenceToDomain(ref aicontract.WorkspaceReferenceOutput) domain.WorkspaceReference {
	return domain.WorkspaceReference{
		Kind:  domain.WorkspaceReferenceKind(ref.Kind),
		Value: ref.Value,
	}
}

func workspaceLoadMultipleSkillsToDomain(items []aicontract.WorkspaceLoadMultipleSkillOutput) []domain.WorkspaceLoadMultipleSkill {
	out := make([]domain.WorkspaceLoadMultipleSkill, len(items))
	for i, item := range items {
		out[i] = domain.WorkspaceLoadMultipleSkill{
			Condition:  item.Condition,
			ProjectIDs: stringsOrEmpty(item.ProjectIDs),
			Reason:     item.Reason,
		}
	}
	return out
}
