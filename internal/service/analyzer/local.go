package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type localAnalysis struct {
	Patterns   []domain.Pattern
	Unresolved []string
}

type localEvidenceGroup struct {
	ID          string
	Name        string
	Category    domain.Category
	Description string
	Rule        string
	Locations   []domain.PatternEvidenceLocation
}

func planLocalAnalysisUnits(paths []string, scope config.LearningScope) []domain.AnalysisUnit {
	groups := make(map[string][]string)
	for _, path := range normalizeLocalPaths(paths) {
		parts := strings.Split(path, "/")
		depth := 1
		switch config.NormalizeLearningScope(string(scope)) {
		case config.LearningScopeFlow:
			depth = 2
		case config.LearningScopeModule:
			depth = max(1, len(parts)-1)
		}
		if depth > len(parts)-1 {
			depth = len(parts) - 1
		}
		key := "root"
		if depth > 0 {
			key = strings.Join(parts[:depth], "/")
		}
		groups[key] = append(groups[key], path)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	units := make([]domain.AnalysisUnit, 0, len(keys))
	for _, key := range keys {
		paths := groups[key]
		sort.Strings(paths)
		units = append(units, domain.AnalysisUnit{
			ID:          localID("unit", key),
			Name:        strings.ReplaceAll(key, "/", " / "),
			RouteTerms:  strings.FieldsFunc(key, func(r rune) bool { return r == '/' || r == '-' || r == '_' }),
			EntryPaths:  paths,
			ScopeReason: "按稳定目录和模块边界生成的本地分析单元",
		})
	}
	return units
}

func (s *AnalyzerService) analyzeLocalUnit(ctx context.Context, root string, unit domain.AnalysisUnit, paths []string) localAnalysis {
	groups := map[string]*localEvidenceGroup{}
	resolved := map[string]bool{}
	paths = normalizeLocalPaths(paths)
	if s.symbolCollector == nil {
		return localAnalysis{Unresolved: paths}
	}
	catalog, err := s.symbolCollector.Collect(ctx, root, paths)
	if err != nil {
		return localAnalysis{Unresolved: paths}
	}
	for _, path := range paths {
		if ctx.Err() != nil {
			break
		}
		symbols := catalog[path]
		if len(symbols) == 0 {
			continue
		}
		for _, symbol := range symbols {
			kind := localSymbolPattern(path, symbol)
			if kind == nil {
				continue
			}
			location := domain.PatternEvidenceLocation{Path: path, Line: symbol.Line, Symbol: symbol.Name, Kind: symbol.Kind, Description: kind.Description, Confidence: 0.95}
			group := groups[kind.ID]
			if group == nil {
				copy := *kind
				group = &copy
				groups[kind.ID] = group
			}
			group.Locations = append(group.Locations, location)
		}
	}
	patterns := make([]domain.Pattern, 0, len(groups))
	for _, group := range groups {
		if localEvidenceFileCount(group.Locations) < 2 {
			continue
		}
		pattern := domain.NewPattern(localID(group.ID, unit.ID), group.Name, group.Category)
		pattern.Description = group.Description
		pattern.Rule = group.Rule
		pattern.Confidence = 0.82
		pattern.Frequency = len(group.Locations)
		pattern.Source = domain.SourceLearnedCurrent
		pattern.AnalysisUnitID = unit.ID
		pattern.AnalysisUnitName = unit.Name
		pattern.EvidenceLocations = group.Locations
		for _, location := range group.Locations {
			resolved[location.Path] = true
		}
		pattern.RefreshMetrics()
		patterns = append(patterns, *pattern)
	}
	sort.Slice(patterns, func(i, j int) bool { return patterns[i].ID < patterns[j].ID })
	var unresolved []string
	for _, path := range normalizeLocalPaths(paths) {
		if !resolved[path] {
			unresolved = append(unresolved, path)
		}
	}
	return localAnalysis{Patterns: patterns, Unresolved: unresolved}
}

func localEvidenceFileCount(locations []domain.PatternEvidenceLocation) int {
	paths := map[string]bool{}
	for _, location := range locations {
		if location.Path != "" {
			paths[location.Path] = true
		}
	}
	return len(paths)
}

func localSymbolPattern(path string, symbol sourcecode.Symbol) *localEvidenceGroup {
	name := strings.ToLower(symbol.Name)
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	switch {
	case isTestSourcePath(lowerPath):
		return &localEvidenceGroup{ID: "test-symbol-convention", Name: "Test symbol convention", Category: domain.CategoryTesting, Description: "Tests use repeated, source-backed test entry point conventions.", Rule: "Place new tests with the demonstrated test files and preserve their symbol naming convention."}
	case strings.HasPrefix(name, "new") || strings.HasPrefix(name, "create") || strings.HasPrefix(name, "build"):
		return &localEvidenceGroup{ID: "constructor-convention", Name: "Constructor convention", Category: domain.CategoryStructure, Description: "The project consistently exposes explicit construction functions at the listed source symbols.", Rule: "When adding the same kind of component, follow the demonstrated constructor naming and dependency assembly shape."}
	case strings.Contains(name, "validate") || strings.HasPrefix(name, "check") || strings.Contains(name, "verify"):
		return &localEvidenceGroup{ID: "validation-entrypoints", Name: "Validation entry points", Category: domain.CategoryStructure, Description: "Validation responsibilities are exposed through dedicated functions or methods at the listed symbols.", Rule: "Keep validation at the demonstrated entry points and reuse those checks before duplicating validation logic."}
	case strings.Contains(name, "handler") || strings.Contains(name, "controller") || strings.Contains(name, "endpoint"):
		return &localEvidenceGroup{ID: "request-handler-convention", Name: "Request handler convention", Category: domain.CategoryAPI, Description: "Request entry points follow a repeated handler or controller naming convention.", Rule: "Add request entry points beside the listed handlers and preserve their established naming boundary."}
	case strings.Contains(name, "middleware") || strings.Contains(lowerPath, "middleware") && strings.HasPrefix(name, "with"):
		return &localEvidenceGroup{ID: "middleware-composition", Name: "Middleware composition", Category: domain.CategoryMiddleware, Description: "Cross-cutting behavior is composed through dedicated middleware-style functions.", Rule: "Implement matching cross-cutting behavior through the demonstrated middleware composition boundary."}
	case strings.Contains(name, "transaction") || isTransactionSymbol(lowerPath, name):
		return &localEvidenceGroup{ID: "transaction-boundary", Name: "Transaction boundary", Category: domain.CategoryDatabase, Description: "Database transaction lifecycle is represented by dedicated source symbols.", Rule: "Keep multi-step persistence inside the demonstrated transaction boundary and preserve its error path."}
	default:
		return nil
	}
}

func isTestSourcePath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "_test.") || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, ".spec.ts") || strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".spec.js") || strings.HasSuffix(base, ".test.js") {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch part {
		case "test", "tests", "__tests__":
			return true
		}
	}
	return false
}

func isTransactionSymbol(path, name string) bool {
	if !strings.Contains(path, "database") && !strings.Contains(path, "storage") && !strings.Contains(path, "repository") && !strings.Contains(path, "/db/") && !strings.Contains(path, "/sql/") {
		return false
	}
	return strings.HasPrefix(name, "begin") || strings.HasPrefix(name, "commit") || strings.HasPrefix(name, "rollback")
}

func normalizeLocalPaths(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, path := range paths {
		path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if path == "" || path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, "../") || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func localID(prefix, value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(prefix + "-" + value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	id := strings.Trim(b.String(), "-")
	if len(id) <= 72 {
		return id
	}
	sum := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%.56s-%s", id, hex.EncodeToString(sum[:4]))
}

func (s *AnalyzerService) analyzeLocalProject(root, name, language string, focusPaths []string) *AnalyzeProjectResult {
	if strings.TrimSpace(language) == "" {
		language = "unknown"
	}
	paths := normalizeLocalPaths(utils.RelativePaths(root, focusPaths))
	if len(paths) == 0 {
		policy := fileanalysis.NewConfiguredSelectionPolicy(s.configRepo, root)
		entries, _ := os.ReadDir(root)
		for _, entry := range entries {
			if entry.IsDir() && !policy.IsExcluded(entry.Name()) {
				paths = append(paths, entry.Name())
			}
		}
	}
	modulePaths := map[string]bool{}
	for _, path := range paths {
		part := strings.Split(path, "/")[0]
		if part != "" && !strings.HasPrefix(part, ".") && (strings.Contains(path, "/") || filepath.Ext(part) == "") {
			modulePaths[part] = true
		}
	}
	modules := make([]domain.ModuleInfo, 0, len(modulePaths))
	for path := range modulePaths {
		modules = append(modules, domain.ModuleInfo{Name: path, DisplayName: path, Path: path, Description: "Top-level project module discovered from the repository layout."})
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	validations := localValidationCommands(root)
	return &AnalyzeProjectResult{
		Language:           language,
		Architecture:       "Repository structure inferred locally from source and build boundaries.",
		Structure:          focusedStructure(paths),
		KeyModules:         modules,
		ValidationCommands: validations,
		Summary:            fmt.Sprintf("%s project profile built from local repository facts.", name),
	}
}

func localValidationCommands(root string) []domain.ValidationCommand {
	type marker struct {
		File    string
		Command string
		Type    string
	}
	markers := []marker{
		{File: "go.mod", Command: "go test ./...", Type: "test"},
		{File: "Cargo.toml", Command: "cargo test", Type: "test"},
	}
	var commands []domain.ValidationCommand
	for _, item := range markers {
		if _, err := os.Stat(filepath.Join(root, item.File)); err == nil {
			commands = append(commands, domain.ValidationCommand{Command: item.Command, When: "after relevant source changes", Source: item.File, Type: item.Type, Evidence: []string{item.File}})
		}
	}
	return commands
}
