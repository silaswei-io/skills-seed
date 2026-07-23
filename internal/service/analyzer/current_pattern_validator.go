package analyzer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type currentPatternSource struct {
	content   string
	lineCount int
	symbols   []sourcecode.Symbol
}

type currentPatternValidator struct {
	projectRoot string
	catalog     sourcecode.Catalog
	files       map[string]currentPatternSource
}

func validateCurrentPatterns(ctx context.Context, projectRoot string, patterns []domain.Pattern, resolver sourcecode.Resolver) ([]domain.Pattern, error) {
	validator, err := newCurrentPatternValidator(ctx, projectRoot, patterns, resolver)
	if err != nil {
		return nil, err
	}
	return validator.validatePatterns(patterns), nil
}

func newCurrentPatternValidator(ctx context.Context, projectRoot string, patterns []domain.Pattern, resolver sourcecode.Resolver) (*currentPatternValidator, error) {
	if resolver == nil {
		return nil, fmt.Errorf("symbol resolver is required")
	}
	refs := make([]sourcecode.Reference, 0)
	for _, pattern := range patterns {
		refs = append(refs, sourcecode.EvidenceReferences(pattern.EvidenceLocations)...)
	}
	catalog, err := resolver.Resolve(ctx, projectRoot, refs)
	if err != nil {
		return nil, fmt.Errorf("resolve pattern evidence: %w", err)
	}
	return &currentPatternValidator{
		projectRoot: projectRoot,
		catalog:     catalog,
		files:       make(map[string]currentPatternSource),
	}, nil
}

func (v *currentPatternValidator) validatePatterns(patterns []domain.Pattern) []domain.Pattern {
	validated := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if candidate, ok := v.validate(pattern); ok {
			validated = append(validated, candidate)
		}
	}
	return validated
}

func (v *currentPatternValidator) validate(pattern domain.Pattern) (domain.Pattern, bool) {
	if strings.TrimSpace(pattern.ID) == "" || strings.TrimSpace(pattern.Name) == "" || strings.TrimSpace(pattern.Rule) == "" {
		return domain.Pattern{}, false
	}

	locations := make([]domain.PatternEvidenceLocation, 0, len(pattern.EvidenceLocations))
	seen := make(map[string]struct{}, len(pattern.EvidenceLocations))
	for _, location := range pattern.EvidenceLocations {
		verified, ok := v.validateLocation(location)
		if !ok {
			continue
		}
		key := evidenceKey(verified)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		locations = append(locations, verified)
	}
	if len(locations) == 0 {
		return domain.Pattern{}, false
	}

	pattern.EvidenceLocations = locations
	pattern.Source = domain.SourceLearnedCurrent
	pattern.Status = domain.PatternStatusActive
	pattern.Frequency = domain.PatternEvidenceFileCount(locations)
	if strings.TrimSpace(pattern.GoodExample) != "" && !v.exampleExists(pattern.GoodExample, locations) {
		pattern.GoodExample = ""
	}
	if pattern.ScopePath == "" {
		pattern.ScopePath = singleEvidencePath(locations)
	}
	pattern.RefreshMetrics()
	return pattern, true
}

func (v *currentPatternValidator) validateLocation(location domain.PatternEvidenceLocation) (domain.PatternEvidenceLocation, bool) {
	path, source, ok := v.source(location.Path)
	if !ok {
		return domain.PatternEvidenceLocation{}, false
	}
	location.Path = path
	if strings.TrimSpace(location.Symbol) == "" {
		if location.Line < 0 || location.Line > source.lineCount {
			return domain.PatternEvidenceLocation{}, false
		}
		if location.Kind == "" {
			location.Kind = "file"
		}
		return location, true
	}

	symbol, ok := sourcecode.FindSymbol(source.symbols, location.Symbol, location.Kind, location.Line)
	if !ok {
		return domain.PatternEvidenceLocation{}, false
	}
	location.Symbol = symbol.Name
	location.Kind = symbol.Kind
	location.Line = symbol.Line
	return location, true
}

func (v *currentPatternValidator) source(path string) (string, currentPatternSource, bool) {
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" || filepath.IsAbs(path) {
		return "", currentPatternSource{}, false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", currentPatternSource{}, false
	}
	fullPath := filepath.Join(v.projectRoot, clean)
	resolvedPath, err := utils.CanonicalPathWithinRoot(v.projectRoot, fullPath)
	if err != nil {
		return "", currentPatternSource{}, false
	}
	relative, err := filepath.Rel(v.projectRoot, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", currentPatternSource{}, false
	}
	path = filepath.ToSlash(relative)
	if source, ok := v.files[path]; ok {
		return path, source, true
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", currentPatternSource{}, false
	}
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	source := currentPatternSource{
		content:   normalized,
		lineCount: len(strings.Split(normalized, "\n")),
		symbols:   v.catalog[path],
	}
	v.files[path] = source
	return path, source, true
}

func (v *currentPatternValidator) exampleExists(example string, locations []domain.PatternEvidenceLocation) bool {
	example = strings.TrimSpace(strings.ReplaceAll(example, "\r\n", "\n"))
	for _, location := range locations {
		_, source, ok := v.source(location.Path)
		if ok && strings.Contains(source.content, example) {
			return true
		}
	}
	return false
}

func singleEvidencePath(locations []domain.PatternEvidenceLocation) string {
	path := ""
	for _, location := range locations {
		if path == "" {
			path = location.Path
			continue
		}
		if path != location.Path {
			return ""
		}
	}
	return path
}

func evidenceKey(location domain.PatternEvidenceLocation) string {
	return location.Path + "|" + strconv.Itoa(location.Line) + "|" + location.Symbol + "|" + location.Kind
}
