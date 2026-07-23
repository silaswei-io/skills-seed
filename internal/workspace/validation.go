package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

// ValidationOptions 提供无法从工作区候选结果中可信推导的校验上下文。
type ValidationOptions struct {
	RootPath       string
	HasUserContext bool
}

// ValidationIssue 描述一个稳定、可定位的工作区语义错误。
type ValidationIssue struct {
	Field   string
	Message string
}

// ValidationError 聚合一次 AI 输出中的全部语义错误，避免逐项重试。
type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, issue.Field+": "+issue.Message)
	}
	return "invalid workspace analysis: " + strings.Join(parts, "; ")
}

type validationIssues struct {
	items []ValidationIssue
}

func (v *validationIssues) add(field, format string, args ...any) {
	v.items = append(v.items, ValidationIssue{Field: field, Message: fmt.Sprintf(format, args...)})
}

func (v *validationIssues) err() error {
	if len(v.items) == 0 {
		return nil
	}
	return &ValidationError{Issues: v.items}
}

func validateProfile(profile *domain.WorkspaceProfile, projects map[string]domain.WorkspaceProject, issues *validationIssues) {
	roles := projectRoles(projects)
	paths := declaredWorkspacePaths(profile)
	for _, group := range []struct {
		name  string
		items []domain.WorkspacePath
	}{{"shared", profile.Shared}, {"contracts", profile.Contracts}, {"infra", profile.Infra}} {
		for i, item := range group.items {
			field := fmt.Sprintf("%s[%d]", group.name, i)
			validateExistingRelativePath(profile.RootPath, item.Path, field+".path", issues)
			validateProjectIDs(item.Consumers, projects, field+".consumers", issues)
			validateProjectIDs(item.Producers, projects, field+".producers", issues)
			validateProjectIDs(item.AffectedProjects, projects, field+".affected_projects", issues)
		}
	}
	for i, dependency := range profile.Dependencies {
		field := fmt.Sprintf("dependencies[%d]", i)
		if _, ok := projects[dependency.FromProjectID]; !ok {
			issues.add(field+".from_project_id", "unknown project %q", dependency.FromProjectID)
		}
		validateReference(dependency.To, projects, roles, paths, profile.RootPath, false, field+".to", issues)
		if dependency.To.Kind == domain.WorkspaceReferenceRole {
			issues.add(field+".to.kind", "dependency target cannot be a role")
		}
	}
	validateRoutes(profile.ImpactRoutes, projects, profile.RootPath, "impact_routes", issues)
}

func validateSpec(spec *domain.WorkspaceSpec, profile *domain.WorkspaceProfile, opts ValidationOptions, persisted bool, issues *validationIssues) {
	if profile == nil {
		issues.add("profile", "workspace profile is required")
		return
	}
	root := firstNonEmpty(opts.RootPath, profile.RootPath)
	projects := make(map[string]domain.WorkspaceProject, len(profile.Projects))
	for _, project := range profile.Projects {
		projects[project.ID] = project
	}
	roles := projectRoles(projects)
	paths := declaredWorkspacePaths(profile)
	validateRoutes(spec.Routing, projects, root, "routing", issues)
	for i := range spec.Rules {
		rule := &spec.Rules[i]
		field := fmt.Sprintf("rules[%d]", i)
		for j, ref := range rule.AppliesTo {
			validateReference(ref, projects, roles, paths, root, true, fmt.Sprintf("%s.applies_to[%d]", field, j), issues)
		}
		rule.Source = strings.TrimSpace(rule.Source)
		rule.Evidence = cleanStrings(rule.Evidence)
		rule.Authority = ruleAuthority(rule.Source, root, opts.HasUserContext, persisted, field, rule.Evidence, issues)
	}
	for i, step := range spec.ChangeOrder {
		if hasListPrefix(step) {
			issues.add(fmt.Sprintf("change_order[%d]", i), "step text must not contain a list number prefix")
		}
	}
	for i, item := range spec.ParallelAgentGuidance {
		validateReference(item.Scope, projects, roles, paths, root, true, fmt.Sprintf("parallel_agent_guidance[%d].scope", i), issues)
	}
	for i, item := range spec.LoadMultipleSkillsWhen {
		validateProjectIDs(item.ProjectIDs, projects, fmt.Sprintf("load_multiple_skills_when[%d].project_ids", i), issues)
	}
}

func ruleAuthority(source, root string, hasUserContext, persisted bool, field string, evidence []string, issues *validationIssues) domain.WorkspaceRuleAuthority {
	switch source {
	case workspaceRuleSourceSystem:
		return domain.WorkspaceRuleAuthoritySystem
	case workspaceRuleSourceProfile:
		return domain.WorkspaceRuleAuthorityInferred
	case workspaceRuleSourceUser:
		if !hasUserContext && !persisted {
			issues.add(field+".source", "user_context was not provided for this learn run")
		}
		return domain.WorkspaceRuleAuthorityUser
	case "":
		issues.add(field+".source", "rule source is required")
		return ""
	default:
		validateExistingRelativePath(root, source, field+".source", issues)
		if len(evidence) == 0 {
			issues.add(field+".evidence", "repository rule requires evidence paths")
		}
		for i, path := range evidence {
			validateExistingRelativePath(root, path, fmt.Sprintf("%s.evidence[%d]", field, i), issues)
		}
		return domain.WorkspaceRuleAuthorityRepository
	}
}

func validateRoutes(routes []domain.WorkspaceRoute, projects map[string]domain.WorkspaceProject, root, field string, issues *validationIssues) {
	for i, route := range routes {
		prefix := fmt.Sprintf("%s[%d]", field, i)
		validateProjectIDs(route.ProjectIDs, projects, prefix+".project_ids", issues)
		validateExistingPathPattern(root, route.PathPattern, prefix+".path_pattern", issues)
	}
}

func validateProjectIDs(ids []string, projects map[string]domain.WorkspaceProject, field string, issues *validationIssues) {
	for i, id := range ids {
		if _, ok := projects[id]; !ok {
			issues.add(fmt.Sprintf("%s[%d]", field, i), "unknown project %q", id)
		}
	}
}

func validateReference(ref domain.WorkspaceReference, projects map[string]domain.WorkspaceProject, roles, paths map[string]bool, root string, allowPattern bool, field string, issues *validationIssues) {
	switch ref.Kind {
	case domain.WorkspaceReferenceProject:
		if _, ok := projects[ref.Value]; !ok {
			issues.add(field+".value", "unknown project %q", ref.Value)
		}
	case domain.WorkspaceReferenceRole:
		if !roles[ref.Value] {
			issues.add(field+".value", "unknown role %q", ref.Value)
		}
	case domain.WorkspaceReferencePath:
		if allowPattern {
			validateExistingPathPattern(root, ref.Value, field+".value", issues)
		} else if !paths[ref.Value] {
			issues.add(field+".value", "unknown declared workspace path %q", ref.Value)
		}
	default:
		issues.add(field+".kind", "unsupported reference kind %q", ref.Kind)
	}
}

func validateExistingRelativePath(root, path, field string, issues *validationIssues) {
	clean, ok := safeRelativePath(path)
	if !ok || strings.ContainsAny(clean, "*?[") {
		issues.add(field, "invalid workspace-relative path %q", path)
		return
	}
	resolved, err := utils.CanonicalPathWithinRoot(root, filepath.Join(root, filepath.FromSlash(clean)))
	if err != nil {
		issues.add(field, "path %q is outside the workspace root", clean)
		return
	}
	if _, err := os.Stat(resolved); err != nil {
		issues.add(field, "path %q does not exist", clean)
	}
}

func validateExistingPathPattern(root, pattern, field string, issues *validationIssues) {
	clean, ok := safeRelativePath(pattern)
	if !ok {
		issues.add(field, "invalid workspace-relative path pattern %q", pattern)
		return
	}
	prefix := routeStaticPrefix(clean)
	if prefix == "" {
		return
	}
	resolved, err := utils.CanonicalPathWithinRoot(root, filepath.Join(root, filepath.FromSlash(prefix)))
	if err != nil {
		issues.add(field, "static path prefix %q is outside the workspace root", prefix)
		return
	}
	if _, err := os.Stat(resolved); err != nil {
		issues.add(field, "static path prefix %q does not exist", prefix)
	}
}

func safeRelativePath(path string) (string, bool) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" || filepath.IsAbs(filepath.FromSlash(path)) {
		return "", false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func routeStaticPrefix(pattern string) string {
	cut := len(pattern)
	for _, marker := range []string{"*", "?", "["} {
		if idx := strings.Index(pattern, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return strings.Trim(pattern[:cut], "/")
}

func hasListPrefix(value string) bool {
	value = strings.TrimSpace(value)
	i := 0
	for i < len(value) && unicode.IsDigit(rune(value[i])) {
		i++
	}
	return i > 0 && i < len(value) && (value[i] == '.' || value[i] == ')')
}

func declaredWorkspacePaths(profile *domain.WorkspaceProfile) map[string]bool {
	paths := make(map[string]bool, len(profile.Shared)+len(profile.Contracts)+len(profile.Infra))
	for _, item := range append(append(append([]domain.WorkspacePath{}, profile.Shared...), profile.Contracts...), profile.Infra...) {
		paths[item.Path] = true
	}
	return paths
}

func projectRoles(projects map[string]domain.WorkspaceProject) map[string]bool {
	roles := make(map[string]bool)
	for _, project := range projects {
		if project.Type != "" {
			roles[project.Type] = true
		}
	}
	return roles
}
