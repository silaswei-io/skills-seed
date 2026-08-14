package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/repositoryscope"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

// VerifyProjectKnowledge 对可渲染知识执行统一的范围、路径和符号核验。
// 返回值可安全持久化或投影；调用方不得将未核验的输入直接交给模板。
func VerifyProjectKnowledge(ctx context.Context, profile *domain.ProjectProfile, patterns []domain.Pattern, projectRoot string, resolver sourcecode.Resolver, scope repositoryscope.Scope) (*domain.ProjectProfile, []domain.Pattern, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return profile, patterns, nil
	}
	if resolver == nil {
		return nil, nil, fmt.Errorf("symbol resolver is required")
	}
	catalog, err := resolver.Resolve(ctx, projectRoot, knowledgeReferences(profile, patterns))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve project knowledge symbols: %w", err)
	}
	verifier := knowledgeVerifier{root: projectRoot, symbols: sourcecode.NewVerifier(catalog), scope: scope}
	verifiedProfile := verifier.profile(profile)
	verifiedPatterns := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = verifier.pattern(pattern)
		if pattern.AllowsHardConstraint() || len(pattern.EvidenceLocations) > 0 || pattern.BusinessMethod != nil {
			verifiedPatterns = append(verifiedPatterns, pattern)
		}
	}
	return verifier.enrichProfile(verifiedProfile, verifiedPatterns), verifiedPatterns, nil
}

func knowledgeReferences(profile *domain.ProjectProfile, patterns []domain.Pattern) []sourcecode.Reference {
	var refs []sourcecode.Reference
	for _, pattern := range patterns {
		refs = append(refs, sourcecode.EvidenceReferences(pattern.EvidenceLocations)...)
		if pattern.BusinessMethod != nil {
			refs = append(refs, sourcecode.BusinessMethodReferences([]domain.BusinessMethod{*pattern.BusinessMethod})...)
		}
	}
	return nonProcedureReferences(refs)
}

func nonProcedureReferences(refs []sourcecode.Reference) []sourcecode.Reference {
	out := make([]sourcecode.Reference, 0, len(refs))
	for _, ref := range refs {
		if !sourcecode.IsProcedureSource(pathOnly(ref.Path)) {
			out = append(out, ref)
		}
	}
	return out
}

type knowledgeVerifier struct {
	root    string
	symbols *sourcecode.Verifier
	scope   repositoryscope.Scope
}

func (v knowledgeVerifier) profile(profile *domain.ProjectProfile) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}
	out := *profile
	// 旧画像可能保存过能力入口；新链路只从已评审知识投影能力，避免双重事实源。
	out.BusinessMethods = nil
	out.CommonUtils = nil
	out.Layers = make([]domain.ArchitectureLayer, 0, len(profile.Layers))
	for _, layer := range profile.Layers {
		layer.Files = v.paths(layer.Files)
		out.Layers = append(out.Layers, layer)
	}
	out.KeyModules = make([]domain.ModuleInfo, 0, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		if strings.TrimSpace(module.Path) == "" || !v.exists(module.Path) {
			continue
		}
		module.KeyMethods = nil
		out.KeyModules = append(out.KeyModules, module)
	}
	out.KeyModules = verifiedModuleRelations(out.KeyModules)
	return &out
}

func (v knowledgeVerifier) enrichProfile(profile *domain.ProjectProfile, patterns []domain.Pattern) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}
	out := *profile
	out.BusinessMethods = mergeBusinessMethods(profile.BusinessMethods, patterns)
	for i := range out.KeyModules {
		out.KeyModules[i].KeyMethods = moduleMethods(out.KeyModules[i].Path, out.BusinessMethods)
	}
	return &out
}

func (v knowledgeVerifier) pattern(pattern domain.Pattern) domain.Pattern {
	if strings.TrimSpace(pattern.GoodExample) != "" && !v.snippetExists(pattern.GoodExample, patternPaths(pattern)) {
		pattern.GoodExample = ""
	}
	pattern.EvidenceLocations = v.evidenceLocations(pattern.EvidenceLocations)
	if pattern.BusinessMethod != nil {
		methods := v.symbols.VerifyBusinessMethods([]domain.BusinessMethod{*pattern.BusinessMethod})
		if len(methods) == 0 || !v.validKnowledgePath(methods[0].DisplayLocation()) {
			pattern.BusinessMethod = nil
		} else {
			pattern.BusinessMethod = &methods[0]
		}
	}
	if pattern.ScopePath != "" && !v.exists(pattern.ScopePath) {
		pattern.ScopePath = ""
	}
	if !pattern.AllowsHardConstraint() {
		if count := domain.PatternEvidenceFileCount(pattern.EvidenceLocations); count > 0 {
			pattern.Frequency = count
		}
		updatedAt := pattern.UpdatedAt
		pattern.RefreshMetrics()
		pattern.UpdatedAt = updatedAt
	}
	return pattern
}

func (v knowledgeVerifier) evidenceLocations(locations []domain.PatternEvidenceLocation) []domain.PatternEvidenceLocation {
	resolved := v.symbols.VerifyEvidenceLocations(locations)
	verified := make([]domain.PatternEvidenceLocation, 0, len(resolved))
	files := make(map[string]bool, len(resolved))
	for _, location := range resolved {
		path := pathOnly(location.Path)
		if !v.scope.AllowsKnowledge(path) || sourcecode.IsProcedureSource(path) {
			continue
		}
		location.Path = path
		files[path] = true
		verified = append(verified, location)
	}
	for _, location := range locations {
		path := pathOnly(location.Path)
		if path == "" || !v.scope.AllowsKnowledge(path) || sourcecode.IsProcedureSource(path) || strings.TrimSpace(location.Symbol) != "" || files[path] || !v.exists(path) {
			continue
		}
		files[path] = true
		verified = append(verified, domain.PatternEvidenceLocation{Path: path, Kind: "file"})
	}
	return verified
}

func (v knowledgeVerifier) paths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if clean, _, ok := v.path(path); ok {
			out = append(out, clean)
		}
	}
	return out
}

func (v knowledgeVerifier) exists(location string) bool {
	_, _, ok := v.path(location)
	return ok
}

func (v knowledgeVerifier) path(location string) (string, string, bool) {
	path := pathOnly(location)
	if path == "" || strings.TrimSpace(v.root) == "" || !v.validKnowledgePath(path) {
		return "", "", false
	}
	fullPath, err := projectpath.CanonicalWithinRoot(v.root, filepath.Join(v.root, filepath.FromSlash(path)))
	if err != nil {
		return "", "", false
	}
	if _, err := os.Stat(fullPath); err != nil {
		return "", "", false
	}
	return path, fullPath, true
}

func (v knowledgeVerifier) validKnowledgePath(location string) bool {
	path := pathOnly(location)
	return path != "" && v.scope.AllowsKnowledge(path) && !sourcecode.IsProcedureSource(path)
}

func (v knowledgeVerifier) snippetExists(snippet string, paths []string) bool {
	snippet = strings.TrimSpace(snippet)
	if snippet == "" {
		return true
	}
	for _, path := range paths {
		_, fullPath, ok := v.path(path)
		if !ok {
			continue
		}
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err == nil && strings.Contains(string(content), snippet) {
			return true
		}
	}
	return false
}

func mergeBusinessMethods(methods []domain.BusinessMethod, patterns []domain.Pattern) []domain.BusinessMethod {
	out := make([]domain.BusinessMethod, 0, len(methods)+len(patterns))
	seen := make(map[string]bool, len(methods)+len(patterns))
	add := func(method domain.BusinessMethod) {
		location := strings.TrimSpace(method.DisplayLocation())
		signature := strings.TrimSpace(method.Function)
		if location == "" || signature == "" {
			return
		}
		key := strings.ToLower(location + "\x00" + signature)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, method)
	}
	for _, method := range methods {
		add(method)
	}
	for _, pattern := range patterns {
		if pattern.BusinessMethod != nil {
			add(*pattern.BusinessMethod)
		}
	}
	return out
}

func verifiedModuleRelations(modules []domain.ModuleInfo) []domain.ModuleInfo {
	aliases := make(map[string]string, len(modules)*3)
	for _, module := range modules {
		canonical := strings.TrimSpace(module.Path)
		for _, value := range []string{module.Name, module.DisplayName, module.Path} {
			if key := moduleRelationKey(value); key != "" {
				aliases[key] = canonical
			}
		}
	}
	for i := range modules {
		self := strings.TrimSpace(modules[i].Path)
		modules[i].Dependencies = verifiedModuleRelationList(modules[i].Dependencies, self, aliases)
		modules[i].Dependents = verifiedModuleRelationList(modules[i].Dependents, self, aliases)
	}
	return modules
}

func verifiedModuleRelationList(values []string, self string, aliases map[string]string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		canonical := aliases[moduleRelationKey(value)]
		if canonical == "" || canonical == self || seen[canonical] {
			continue
		}
		seen[canonical] = true
		out = append(out, canonical)
	}
	return out
}

func moduleRelationKey(value string) string {
	value = pathx.CleanEvidenceLocationPath(value)
	return strings.ToLower(strings.Trim(value, "/"))
}

func moduleMethods(modulePath string, methods []domain.BusinessMethod) []string {
	modulePath = strings.Trim(pathOnly(modulePath), "/")
	if modulePath == "" {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, method := range methods {
		path := strings.Trim(pathOnly(method.DisplayLocation()), "/")
		if path != modulePath && !strings.HasPrefix(path, modulePath+"/") {
			continue
		}
		name := strings.TrimSpace(method.Name)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, name)
	}
	return names
}

func patternPaths(pattern domain.Pattern) []string {
	paths := make([]string, 0, len(pattern.EvidenceLocations)+2)
	for _, location := range pattern.EvidenceLocations {
		if location.Path != "" {
			paths = append(paths, location.Path)
		}
	}
	if pattern.ScopePath != "" {
		paths = append(paths, pattern.ScopePath)
	}
	if pattern.BusinessMethod != nil {
		paths = append(paths, pattern.BusinessMethod.DisplayLocation())
	}
	return paths
}

func pathOnly(location string) string {
	return pathx.CleanEvidenceLocationPath(location)
}
