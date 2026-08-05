package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/projectpath"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/utils/pathx"
)

func sanitizeGenerationInputs(ctx context.Context, profile *domain.ProjectProfile, patterns []domain.Pattern, projectRoot string, resolver sourcecode.Resolver) (*domain.ProjectProfile, []domain.Pattern, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return profile, patterns, nil
	}
	if resolver == nil {
		return nil, nil, fmt.Errorf("symbol resolver is required")
	}
	refs := generationReferences(profile, patterns)
	catalog, err := resolver.Resolve(ctx, projectRoot, refs)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve generation symbols: %w", err)
	}
	sanitizer := projectPathSanitizer{
		root:     projectRoot,
		verifier: sourcecode.NewVerifier(catalog),
	}
	sanitizedProfile := sanitizer.profile(profile)
	sanitizedPatterns := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = sanitizer.pattern(pattern)
		if pattern.AllowsHardConstraint() || len(pattern.EvidenceLocations) > 0 || pattern.BusinessMethod != nil || strings.TrimSpace(pattern.ScopePath) != "" {
			sanitizedPatterns = append(sanitizedPatterns, pattern)
		}
	}
	sanitizedProfile = enrichVerifiedProfileEntries(sanitizedProfile, sanitizedPatterns)
	return sanitizedProfile, sanitizedPatterns, nil
}

func generationReferences(profile *domain.ProjectProfile, patterns []domain.Pattern) []sourcecode.Reference {
	var refs []sourcecode.Reference
	if profile != nil {
		refs = append(refs, sourcecode.UtilityReferences(profile.CommonUtils)...)
		refs = append(refs, sourcecode.BusinessMethodReferences(profile.BusinessMethods)...)
	}
	for _, pattern := range patterns {
		refs = append(refs, sourcecode.EvidenceReferences(pattern.EvidenceLocations)...)
		if pattern.BusinessMethod != nil {
			refs = append(refs, sourcecode.BusinessMethodReferences([]domain.BusinessMethod{*pattern.BusinessMethod})...)
		}
	}
	return refs
}

type projectPathSanitizer struct {
	root     string
	verifier *sourcecode.Verifier
}

func (s projectPathSanitizer) profile(profile *domain.ProjectProfile) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}
	out := *profile
	out.Layers = make([]domain.ArchitectureLayer, 0, len(profile.Layers))
	for _, layer := range profile.Layers {
		layer.Files = s.validPathList(layer.Files)
		out.Layers = append(out.Layers, layer)
	}
	out.KeyModules = make([]domain.ModuleInfo, 0, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		if strings.TrimSpace(module.Path) == "" || !s.exists(module.Path) {
			continue
		}
		module.KeyMethods = nil
		out.KeyModules = append(out.KeyModules, module)
	}
	out.BusinessMethods = s.verifier.VerifyBusinessMethods(profile.BusinessMethods)
	out.CommonUtils = s.verifier.VerifyUtilities(profile.CommonUtils)
	return &out
}

func enrichVerifiedProfileEntries(profile *domain.ProjectProfile, patterns []domain.Pattern) *domain.ProjectProfile {
	if profile == nil {
		return nil
	}
	out := *profile
	out.BusinessMethods = mergeVerifiedBusinessMethods(profile.BusinessMethods, patterns)
	for i := range out.KeyModules {
		out.KeyModules[i].KeyMethods = moduleBusinessMethodNames(out.KeyModules[i].Path, out.BusinessMethods)
	}
	return &out
}

func mergeVerifiedBusinessMethods(methods []domain.BusinessMethod, patterns []domain.Pattern) []domain.BusinessMethod {
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

func moduleBusinessMethodNames(modulePath string, methods []domain.BusinessMethod) []string {
	modulePath = strings.Trim(referencePathOnly(modulePath), "/")
	if modulePath == "" {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, method := range methods {
		path := strings.Trim(referencePathOnly(method.DisplayLocation()), "/")
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

func (s projectPathSanitizer) pattern(pattern domain.Pattern) domain.Pattern {
	if strings.TrimSpace(pattern.GoodExample) != "" && !s.snippetExists(pattern.GoodExample, patternSnippetPaths(pattern)) {
		pattern.GoodExample = ""
	}
	pattern.EvidenceLocations = s.evidenceLocations(pattern.EvidenceLocations)
	if pattern.BusinessMethod != nil {
		methods := s.verifier.VerifyBusinessMethods([]domain.BusinessMethod{*pattern.BusinessMethod})
		if len(methods) == 0 {
			pattern.BusinessMethod = nil
		} else {
			pattern.BusinessMethod = &methods[0]
		}
	}
	if pattern.ScopePath != "" && !s.exists(pattern.ScopePath) {
		pattern.ScopePath = ""
	}
	if !pattern.AllowsHardConstraint() {
		if evidenceCount := domain.PatternEvidenceFileCount(pattern.EvidenceLocations); evidenceCount > 0 {
			pattern.Frequency = evidenceCount
		}
		pattern.RefreshMetrics()
	}
	return pattern
}

func (s projectPathSanitizer) evidenceLocations(locations []domain.PatternEvidenceLocation) []domain.PatternEvidenceLocation {
	verified := s.verifier.VerifyEvidenceLocations(locations)
	files := make(map[string]bool, len(verified))
	for _, location := range verified {
		files[referencePathOnly(location.Path)] = true
	}
	for _, location := range locations {
		path := referencePathOnly(location.Path)
		if path == "" || strings.TrimSpace(location.Symbol) != "" || files[path] || !s.exists(path) {
			continue
		}
		files[path] = true
		verified = append(verified, domain.PatternEvidenceLocation{Path: path, Kind: "file"})
	}
	return verified
}

func (s projectPathSanitizer) validPathList(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if clean, _, ok := s.dirOrFilePath(path); ok {
			out = append(out, clean)
		}
	}
	return out
}

func (s projectPathSanitizer) exists(location string) bool {
	_, _, ok := s.dirOrFilePath(location)
	return ok
}

func (s projectPathSanitizer) resolve(location string) (string, string, bool) {
	path := pathx.CleanEvidenceLocationPath(location)
	if path == "" || strings.TrimSpace(s.root) == "" {
		return "", "", false
	}
	fullPath, err := projectpath.CanonicalWithinRoot(s.root, filepath.Join(s.root, filepath.FromSlash(path)))
	if err != nil {
		return "", "", false
	}
	return path, fullPath, true
}

func (s projectPathSanitizer) filePath(location string) (string, string, bool) {
	path, fullPath, ok := s.resolve(location)
	if !ok {
		return "", "", false
	}
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return "", "", false
	}
	return path, fullPath, true
}

func (s projectPathSanitizer) dirOrFilePath(location string) (string, string, bool) {
	path, fullPath, ok := s.resolve(location)
	if !ok {
		return "", "", false
	}
	if _, err := os.Stat(fullPath); err != nil {
		return "", "", false
	}
	return path, fullPath, true
}

func (s projectPathSanitizer) snippetExists(snippet string, paths []string) bool {
	snippet = strings.TrimSpace(snippet)
	if snippet == "" {
		return true
	}
	for _, path := range paths {
		_, fullPath, ok := s.filePath(path)
		if !ok {
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), snippet) {
			return true
		}
	}
	return false
}

func patternSnippetPaths(pattern domain.Pattern) []string {
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

func referencePathOnly(location string) string {
	return pathx.CleanEvidenceLocationPath(location)
}
