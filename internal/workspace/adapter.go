package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type workspacePathKind int

const (
	workspacePathShared workspacePathKind = iota
	workspacePathContract
	workspacePathInfra
)

// workspaceCandidateSanitizer 把非可信 AI 候选事实投影为可进入领域校验的工作区候选。
type workspaceCandidateSanitizer struct {
	root string
	refs workspaceReferenceCatalog
}

func newWorkspaceCandidateSanitizer(root string, projects map[string]domain.WorkspaceProject) workspaceCandidateSanitizer {
	return workspaceCandidateSanitizer{root: root, refs: newWorkspaceReferenceCatalog(projects)}
}

func (s workspaceCandidateSanitizer) sanitizeProfile(profile *domain.WorkspaceProfile) {
	if profile == nil {
		return
	}
	s.resolveProfileProjects(profile)
	s.sanitizeWorkspacePathGroups(profile)
	s.sanitizeDependencies(profile)
	s.sanitizeImpactRoutes(profile)
}

func (s workspaceCandidateSanitizer) sanitizeSpec(spec *domain.WorkspaceSpec) {
	if spec == nil {
		return
	}
	spec.Routing = s.validRoutes(spec.Routing)
	for i := range spec.Rules {
		spec.Rules[i].AppliesTo = s.validReferences(spec.Rules[i].AppliesTo, true)
	}
	spec.ParallelAgentGuidance = s.validParallelGuidance(spec.ParallelAgentGuidance)
	spec.LoadMultipleSkillsWhen = s.validLoadMultipleSkills(spec.LoadMultipleSkillsWhen)
}

func (s workspaceCandidateSanitizer) resolveProfileProjects(profile *domain.WorkspaceProfile) {
	s.resolvePathProjects(profile.Shared)
	s.resolvePathProjects(profile.Contracts)
	s.resolvePathProjects(profile.Infra)
	for i := range profile.Dependencies {
		profile.Dependencies[i].FromProjectID = s.refs.projectIDOrOriginal(profile.Dependencies[i].FromProjectID)
		profile.Dependencies[i].To = s.refs.reference(profile.Dependencies[i].To)
	}
	for i := range profile.ImpactRoutes {
		profile.ImpactRoutes[i].ProjectIDs = s.refs.projectIDs(profile.ImpactRoutes[i].ProjectIDs)
	}
}

func (s workspaceCandidateSanitizer) resolvePathProjects(paths []domain.WorkspacePath) {
	for i := range paths {
		paths[i].Consumers = s.refs.projectIDs(paths[i].Consumers)
		paths[i].Producers = s.refs.projectIDs(paths[i].Producers)
		paths[i].AffectedProjects = s.refs.projectIDs(paths[i].AffectedProjects)
	}
}

func (s workspaceCandidateSanitizer) sanitizeWorkspacePathGroups(profile *domain.WorkspaceProfile) {
	profile.Shared = s.sanitizePathGroup(profile.Shared, &profile.ImpactRoutes, workspacePathShared)
	profile.Contracts = s.sanitizePathGroup(profile.Contracts, &profile.ImpactRoutes, workspacePathContract)
	profile.Infra = s.sanitizePathGroup(profile.Infra, &profile.ImpactRoutes, workspacePathInfra)
	s.absorbImpactRoutes(profile.Shared, &profile.ImpactRoutes, workspacePathShared)
	s.absorbImpactRoutes(profile.Contracts, &profile.ImpactRoutes, workspacePathContract)
	s.absorbImpactRoutes(profile.Infra, &profile.ImpactRoutes, workspacePathInfra)
	profile.ImpactRoutes = mergeRoutes(nil, profile.ImpactRoutes)
}

func (s workspaceCandidateSanitizer) sanitizePathGroup(paths []domain.WorkspacePath, routes *[]domain.WorkspaceRoute, kind workspacePathKind) []domain.WorkspacePath {
	out := make([]domain.WorkspacePath, 0, len(paths))
	for _, item := range paths {
		switch {
		case isWorkspaceExternalLocation(item.Path):
			continue
		case isWorkspacePathPattern(item.Path):
			if s.rootPatternExists(item.Path) {
				s.appendRoute(routes, item, item.Path, workspacePathProjects(item))
			}
		case s.isProjectPath(item.Path):
			continue
		case s.childPathOwner(item.Path) != "":
			item = addWorkspacePathOwner(item, s.childPathOwner(item.Path), kind)
			if len(workspacePathProjects(item)) > 1 && s.rootPathExists(item.Path) {
				out = append(out, item)
			}
		case s.rootPathExists(item.Path):
			out = append(out, item)
		case len(s.childPathProjects(item.Path)) > 0:
			continue
		default:
			continue
		}
	}
	return out
}

func (s workspaceCandidateSanitizer) absorbImpactRoutes(paths []domain.WorkspacePath, routes *[]domain.WorkspaceRoute, kind workspacePathKind) {
	if len(paths) == 0 || len(*routes) == 0 {
		return
	}
	remaining := make([]domain.WorkspaceRoute, 0, len(*routes))
	for _, route := range *routes {
		absorbed := false
		for i := range paths {
			if route.PathPattern != routePatternForPath(paths[i].Path) {
				continue
			}
			paths[i] = mergeRouteIntoWorkspacePath(paths[i], route, kind)
			absorbed = true
			break
		}
		if !absorbed {
			remaining = append(remaining, route)
		}
	}
	*routes = remaining
}

func (s workspaceCandidateSanitizer) isProjectPath(path string) bool {
	_, ok := s.refs.projectID(path)
	return ok
}

func (s workspaceCandidateSanitizer) appendRoute(routes *[]domain.WorkspaceRoute, path domain.WorkspacePath, pattern string, projectIDs []string) {
	*routes = append(*routes, domain.WorkspaceRoute{
		PathPattern: pattern,
		ProjectIDs:  cleanStrings(projectIDs),
		Reason:      path.Description,
	})
}

func (s workspaceCandidateSanitizer) validRoutes(items []domain.WorkspaceRoute) []domain.WorkspaceRoute {
	routes := make([]domain.WorkspaceRoute, 0, len(items))
	for _, route := range items {
		if isWorkspaceExternalLocation(route.PathPattern) || !s.routePrefixExists(route.PathPattern) {
			continue
		}
		route.ProjectIDs = s.refs.projectIDs(route.ProjectIDs)
		if len(route.ProjectIDs) == 0 {
			continue
		}
		routes = append(routes, route)
	}
	return routes
}

func (s workspaceCandidateSanitizer) validReferences(refs []domain.WorkspaceReference, allowPattern bool) []domain.WorkspaceReference {
	out := make([]domain.WorkspaceReference, 0, len(refs))
	for _, ref := range refs {
		ref = s.refs.reference(ref)
		if ref.Kind == domain.WorkspaceReferencePath && !s.validPathReference(ref.Value, allowPattern) {
			continue
		}
		out = append(out, ref)
	}
	return out
}

func (s workspaceCandidateSanitizer) validParallelGuidance(items []domain.WorkspaceParallelGuidance) []domain.WorkspaceParallelGuidance {
	out := make([]domain.WorkspaceParallelGuidance, 0, len(items))
	for _, item := range items {
		refs := s.validReferences([]domain.WorkspaceReference{item.Scope}, true)
		if len(refs) == 0 {
			continue
		}
		item.Scope = refs[0]
		out = append(out, item)
	}
	return out
}

func (s workspaceCandidateSanitizer) validLoadMultipleSkills(items []domain.WorkspaceLoadMultipleSkill) []domain.WorkspaceLoadMultipleSkill {
	out := make([]domain.WorkspaceLoadMultipleSkill, 0, len(items))
	for _, item := range items {
		item.ProjectIDs = s.refs.projectIDs(item.ProjectIDs)
		if len(item.ProjectIDs) == 0 {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s workspaceCandidateSanitizer) sanitizeImpactRoutes(profile *domain.WorkspaceProfile) {
	profile.ImpactRoutes = mergeRoutes(nil, s.validRoutes(profile.ImpactRoutes))
}

func (s workspaceCandidateSanitizer) sanitizeDependencies(profile *domain.WorkspaceProfile) {
	paths := declaredWorkspacePaths(profile)
	out := make([]domain.WorkspaceDependency, 0, len(profile.Dependencies))
	for _, item := range profile.Dependencies {
		if _, ok := s.refs.projectID(item.FromProjectID); !ok {
			continue
		}
		item.To = s.refs.reference(item.To)
		switch item.To.Kind {
		case domain.WorkspaceReferenceProject:
			if _, ok := s.refs.projectID(item.To.Value); !ok {
				continue
			}
		case domain.WorkspaceReferencePath:
			if !paths[item.To.Value] {
				continue
			}
		default:
			continue
		}
		out = append(out, item)
	}
	profile.Dependencies = out
}

func (s workspaceCandidateSanitizer) validPathReference(path string, allowPattern bool) bool {
	if isWorkspaceExternalLocation(path) {
		return false
	}
	if isWorkspacePathPattern(path) {
		return allowPattern && s.rootPatternExists(path)
	}
	return s.rootPathExists(path)
}

func (s workspaceCandidateSanitizer) routePrefixExists(pattern string) bool {
	if isWorkspacePathPattern(pattern) {
		return s.rootPatternExists(pattern)
	}
	return s.rootPathExists(pattern)
}

func (s workspaceCandidateSanitizer) rootPathExists(path string) bool {
	if s.root == "" || path == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(path)))
	return err == nil
}

func (s workspaceCandidateSanitizer) rootPatternExists(pattern string) bool {
	prefix := routeStaticPrefix(pattern)
	if prefix == "" {
		return false
	}
	return s.rootPathExists(prefix)
}

func (s workspaceCandidateSanitizer) childPathProjects(path string) []string {
	matches := make([]string, 0, len(s.refs.projects))
	for id, project := range s.refs.projects {
		if project.Path == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(project.Path), filepath.FromSlash(path))); err == nil {
			matches = append(matches, id)
		}
	}
	return cleanStrings(matches)
}

func (s workspaceCandidateSanitizer) childPathOwner(path string) string {
	path = cleanRelativePath(path)
	owner := ""
	ownerLen := 0
	for id, project := range s.refs.projects {
		projectPath := cleanRelativePath(project.Path)
		if projectPath == "." || projectPath == "" || path == projectPath {
			continue
		}
		if strings.HasPrefix(path, projectPath+"/") && len(projectPath) > ownerLen {
			owner = id
			ownerLen = len(projectPath)
		}
	}
	return owner
}

type workspaceReferenceCatalog struct {
	projects     map[string]domain.WorkspaceProject
	projectPaths map[string]string
	aliases      map[string]string
}

func newWorkspaceReferenceCatalog(projects map[string]domain.WorkspaceProject) workspaceReferenceCatalog {
	paths := make(map[string]string, len(projects))
	aliases := make(map[string]string, len(projects)*4)
	for id, project := range projects {
		addWorkspaceProjectAlias(aliases, id, id)
		addWorkspaceProjectAlias(aliases, projectIDAlias(id), id)
		if project.Path != "" {
			paths[project.Path] = id
			addWorkspaceProjectAlias(aliases, project.Path, id)
			addWorkspaceProjectAlias(aliases, projectIDAlias(project.Path), id)
		}
	}
	return workspaceReferenceCatalog{projects: projects, projectPaths: paths, aliases: aliases}
}

func (r workspaceReferenceCatalog) reference(ref domain.WorkspaceReference) domain.WorkspaceReference {
	if ref.Kind == domain.WorkspaceReferenceProject {
		ref.Value = r.projectIDOrOriginal(ref.Value)
		return ref
	}
	if ref.Kind != domain.WorkspaceReferencePath {
		return ref
	}
	if id, ok := r.projectID(ref.Value); ok {
		ref.Kind = domain.WorkspaceReferenceProject
		ref.Value = id
	}
	return ref
}

func (r workspaceReferenceCatalog) projectIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = r.projectIDOrOriginal(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func (r workspaceReferenceCatalog) projectIDOrOriginal(id string) string {
	if resolved, ok := r.projectID(id); ok {
		return resolved
	}
	return id
}

func (r workspaceReferenceCatalog) projectID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if _, ok := r.projects[value]; ok {
		return value, true
	}
	if id, ok := r.projectPaths[cleanRelativePath(value)]; ok {
		return id, true
	}
	if id, ok := r.aliases[projectIDAlias(value)]; ok {
		return id, true
	}
	return "", false
}

func addWorkspaceProjectAlias(aliases map[string]string, alias, id string) {
	alias = projectIDAlias(alias)
	if alias == "" {
		return
	}
	if existing, ok := aliases[alias]; ok && existing != id {
		delete(aliases, alias)
		return
	}
	aliases[alias] = id
}

func projectIDAlias(value string) string {
	value = strings.ToLower(strings.TrimSpace(cleanRelativePath(value)))
	for _, prefix := range []string{"project:", "path:"} {
		value = strings.TrimPrefix(value, prefix)
	}
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func isWorkspacePathPattern(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func isWorkspaceExternalLocation(path string) bool {
	head, _, ok := strings.Cut(path, "/")
	if !ok {
		head = path
	}
	return strings.Contains(head, ":")
}

func workspacePathProjects(path domain.WorkspacePath) []string {
	return cleanStrings(append(append(append([]string{}, path.Producers...), path.Consumers...), path.AffectedProjects...))
}

func addWorkspacePathOwner(path domain.WorkspacePath, owner string, kind workspacePathKind) domain.WorkspacePath {
	if owner == "" {
		return path
	}
	switch kind {
	case workspacePathInfra:
		path.AffectedProjects = cleanStrings(append([]string{owner}, path.AffectedProjects...))
	default:
		path.Producers = cleanStrings(append([]string{owner}, path.Producers...))
	}
	return path
}

func mergeRouteIntoWorkspacePath(path domain.WorkspacePath, route domain.WorkspaceRoute, kind workspacePathKind) domain.WorkspacePath {
	if path.Description == "" {
		path.Description = route.Reason
	}
	switch kind {
	case workspacePathInfra:
		path.AffectedProjects = cleanStrings(append(path.AffectedProjects, route.ProjectIDs...))
	default:
		path.Consumers = cleanStrings(append(path.Consumers, exceptProjectIDs(route.ProjectIDs, path.Producers)...))
	}
	return path
}

func exceptProjectIDs(ids, excluded []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if slices.Contains(excluded, id) {
			continue
		}
		out = append(out, id)
	}
	return cleanStrings(out)
}

func routePatternForPath(path string) string {
	return filepath.ToSlash(filepath.Join(path, "**"))
}

func workspaceProjectMap(profile *domain.WorkspaceProfile) map[string]domain.WorkspaceProject {
	if profile == nil {
		return nil
	}
	projects := make(map[string]domain.WorkspaceProject, len(profile.Projects))
	for _, project := range profile.Projects {
		projects[project.ID] = project
	}
	return projects
}
