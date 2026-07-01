package generator

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

func sanitizeGenerationInputs(profile *domain.ProjectProfile, patterns []domain.Pattern, projectRoot string) (*domain.ProjectProfile, []domain.Pattern) {
	if strings.TrimSpace(projectRoot) == "" {
		return profile, patterns
	}
	sanitizer := projectPathSanitizer{root: projectRoot}
	sanitizedProfile := sanitizer.profile(profile)
	sanitizedPatterns := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		sanitizedPatterns = append(sanitizedPatterns, sanitizer.pattern(pattern))
	}
	return sanitizedProfile, sanitizedPatterns
}

type projectPathSanitizer struct {
	root string
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
		if module.Path != "" && !s.exists(module.Path) {
			module.Path = ""
		}
		out.KeyModules = append(out.KeyModules, module)
	}
	out.BusinessMethods = make([]domain.BusinessMethod, 0, len(profile.BusinessMethods))
	for _, method := range profile.BusinessMethods {
		out.BusinessMethods = append(out.BusinessMethods, s.businessMethod(method))
	}
	out.CommonUtils = make([]domain.UtilityFunction, 0, len(profile.CommonUtils))
	for _, utility := range profile.CommonUtils {
		if utility.File != "" && !s.exists(utility.File) {
			utility.File = ""
		}
		out.CommonUtils = append(out.CommonUtils, utility)
	}
	return &out
}

func (s projectPathSanitizer) pattern(pattern domain.Pattern) domain.Pattern {
	pattern.EvidenceLocations = s.evidenceLocations(pattern.EvidenceLocations)
	if pattern.BusinessMethod != nil {
		method := s.businessMethod(*pattern.BusinessMethod)
		pattern.BusinessMethod = &method
	}
	if pattern.ScopePath != "" && !s.exists(pattern.ScopePath) {
		pattern.ScopePath = ""
	}
	return pattern
}

func (s projectPathSanitizer) businessMethod(method domain.BusinessMethod) domain.BusinessMethod {
	if method.CodeLocation.CurrentLocation != "" && !s.exists(method.CodeLocation.CurrentLocation) {
		method.CodeLocation.CurrentLocation = ""
	}
	if method.CodeLocation.HistoricalLocation != "" && !s.exists(method.CodeLocation.HistoricalLocation) {
		method.CodeLocation.HistoricalLocation = ""
	}
	if method.CodeLocation.CurrentLocation == "" && method.CodeLocation.HistoricalLocation == "" {
		method.CodeLocation.Status = domain.CodeLocationStatusMissing
	}
	return method
}

func (s projectPathSanitizer) evidenceLocations(locations []domain.PatternEvidenceLocation) []domain.PatternEvidenceLocation {
	out := make([]domain.PatternEvidenceLocation, 0, len(locations))
	for _, location := range locations {
		if s.exists(location.DisplayLocation()) {
			out = append(out, location)
		}
	}
	return out
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
