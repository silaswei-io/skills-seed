package generator

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
)

func sanitizeGenerationInputs(profile *domain.ProjectProfile, patterns []domain.Pattern, projectRoot string) (*domain.ProjectProfile, []domain.Pattern) {
	if strings.TrimSpace(projectRoot) == "" {
		return profile, patterns
	}
	sanitizer := projectPathSanitizer{
		root:     projectRoot,
		verifier: sourcecode.NewVerifier(projectRoot),
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
	return sanitizedProfile, sanitizedPatterns
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
		if s.exists(path) {
			out = append(out, path)
		}
	}
	return out
}

func (s projectPathSanitizer) exists(location string) bool {
	path := referencePathOnly(location)
	if path == "" {
		return false
	}
	fullPath := filepath.Join(s.root, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (s projectPathSanitizer) snippetExists(snippet string, paths []string) bool {
	snippet = strings.TrimSpace(snippet)
	if snippet == "" {
		return true
	}
	for _, path := range paths {
		path = referencePathOnly(path)
		if path == "" || !looksProjectRelativeReference(path) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.root, path))
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
	location = strings.Trim(strings.TrimSpace(location), "`")
	if location == "" {
		return ""
	}
	if idx := strings.Index(location, ":"); idx > 0 {
		next := location[idx+1:]
		if next == "" || isLineSuffix(next) {
			location = location[:idx]
		}
	}
	return filepath.Clean(filepath.ToSlash(location))
}

func looksProjectRelativeReference(location string) bool {
	path := referencePathOnly(location)
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "://") {
		return false
	}
	parts := strings.Split(path, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		return false
	}
	return true
}

func isLineSuffix(value string) bool {
	for _, part := range strings.Split(value, ":") {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}
