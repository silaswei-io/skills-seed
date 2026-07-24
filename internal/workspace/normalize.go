package workspace

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func mergeRoutes(base, analyzed []domain.WorkspaceRoute) []domain.WorkspaceRoute {
	routes := cleanWorkspaceRoutes(base)
	indexByPath := make(map[string]int, len(routes))
	for i, route := range routes {
		indexByPath[route.PathPattern] = i
	}
	for _, route := range cleanWorkspaceRoutes(analyzed) {
		if index, ok := indexByPath[route.PathPattern]; ok {
			routes[index].ProjectIDs = cleanStrings(append(routes[index].ProjectIDs, route.ProjectIDs...))
			if route.Reason != "" {
				routes[index].Reason = route.Reason
			}
			continue
		}
		indexByPath[route.PathPattern] = len(routes)
		routes = append(routes, route)
	}
	return routes
}

func mergeRules(base, analyzed []domain.WorkspaceRule) []domain.WorkspaceRule {
	rules := cleanWorkspaceRules(base)
	seen := make(map[string]bool, len(rules)+len(analyzed))
	for _, rule := range rules {
		seen[workspaceRuleKey(rule)] = true
	}
	for _, rule := range cleanWorkspaceRules(analyzed) {
		key := workspaceRuleKey(rule)
		if seen[key] {
			continue
		}
		seen[key] = true
		rules = append(rules, rule)
	}
	return rules
}

func workspaceRuleKey(rule domain.WorkspaceRule) string {
	return strings.ToLower(rule.Source) + "\x00" + strings.ToLower(rule.Title)
}

func cleanWorkspacePaths(items []domain.WorkspacePath) []domain.WorkspacePath {
	out := make([]domain.WorkspacePath, 0, len(items))
	indexByPath := make(map[string]int, len(items))
	for _, item := range items {
		item.Path = cleanRelativePath(item.Path)
		item.Description = strings.TrimSpace(item.Description)
		item.Consumers = cleanStrings(item.Consumers)
		item.Producers = cleanStrings(item.Producers)
		item.AffectedProjects = cleanStrings(item.AffectedProjects)
		if item.Path == "." {
			continue
		}
		if index, ok := indexByPath[item.Path]; ok {
			out[index].Description = firstNonEmpty(out[index].Description, item.Description)
			out[index].Consumers = cleanStrings(append(out[index].Consumers, item.Consumers...))
			out[index].Producers = cleanStrings(append(out[index].Producers, item.Producers...))
			out[index].AffectedProjects = cleanStrings(append(out[index].AffectedProjects, item.AffectedProjects...))
			continue
		}
		indexByPath[item.Path] = len(out)
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func cleanWorkspaceDependencies(items []domain.WorkspaceDependency) []domain.WorkspaceDependency {
	out := make([]domain.WorkspaceDependency, len(items))
	for i, item := range items {
		item.FromProjectID = strings.TrimSpace(item.FromProjectID)
		item.To = cleanReference(item.To)
		item.Reason = strings.TrimSpace(item.Reason)
		out[i] = item
	}
	return out
}

func cleanWorkspaceRoutes(items []domain.WorkspaceRoute) []domain.WorkspaceRoute {
	out := make([]domain.WorkspaceRoute, len(items))
	for i, item := range items {
		item.PathPattern = cleanRelativePath(item.PathPattern)
		item.ProjectIDs = cleanStrings(item.ProjectIDs)
		item.Reason = strings.TrimSpace(item.Reason)
		out[i] = item
	}
	return out
}

func cleanWorkspaceRules(items []domain.WorkspaceRule) []domain.WorkspaceRule {
	out := make([]domain.WorkspaceRule, len(items))
	for i, item := range items {
		item.Title = strings.TrimSpace(item.Title)
		item.Description = strings.TrimSpace(item.Description)
		item.Source = strings.TrimSpace(item.Source)
		item.Evidence = cleanStrings(item.Evidence)
		item.AppliesTo = cleanReferences(item.AppliesTo)
		out[i] = item
	}
	return out
}

func cleanParallelGuidance(items []domain.WorkspaceParallelGuidance) []domain.WorkspaceParallelGuidance {
	out := make([]domain.WorkspaceParallelGuidance, len(items))
	for i, item := range items {
		item.Scope = cleanReference(item.Scope)
		item.Condition = strings.TrimSpace(item.Condition)
		out[i] = item
	}
	return out
}

func cleanLoadMultiple(items []domain.WorkspaceLoadMultipleSkill) []domain.WorkspaceLoadMultipleSkill {
	out := make([]domain.WorkspaceLoadMultipleSkill, len(items))
	for i, item := range items {
		item.Condition = strings.TrimSpace(item.Condition)
		item.ProjectIDs = cleanStrings(item.ProjectIDs)
		item.Reason = strings.TrimSpace(item.Reason)
		out[i] = item
	}
	return out
}

func cleanReferences(refs []domain.WorkspaceReference) []domain.WorkspaceReference {
	out := make([]domain.WorkspaceReference, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		ref = cleanReference(ref)
		key := string(ref.Kind) + "\x00" + ref.Value
		if ref.Value == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ref)
	}
	return out
}

func cleanReference(ref domain.WorkspaceReference) domain.WorkspaceReference {
	ref.Kind = domain.WorkspaceReferenceKind(strings.TrimSpace(string(ref.Kind)))
	ref.Value = strings.TrimSpace(ref.Value)
	if ref.Kind == domain.WorkspaceReferencePath {
		ref.Value = cleanRelativePath(ref.Value)
	}
	return ref
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func cleanRelativePath(path string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(path))))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
