package analyzer

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/utils"
)

type currentPatternSource struct {
	content   string
	lineCount int
	symbols   []symbolInfo
}

type currentPatternValidator struct {
	projectRoot string
	files       map[string]currentPatternSource
}

func validateCurrentPatterns(projectRoot string, patterns []domain.Pattern) []domain.Pattern {
	validator := currentPatternValidator{
		projectRoot: projectRoot,
		files:       make(map[string]currentPatternSource),
	}
	validated := make([]domain.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		if candidate, ok := validator.validate(pattern); ok {
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
	pattern.Frequency, pattern.Confidence = domain.PatternEvidenceQuality(locations)
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

	symbol, ok := findSymbol(location, source.symbols)
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
	}
	if entry := grammars.DetectLanguage(filepath.Base(path)); entry != nil {
		if tree, language, parseErr := parseTree(path, content); parseErr == nil {
			source.symbols = extractSymbols(tree.RootNode(), language, content, entry.Name)
			tree.Release()
		}
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

func findSymbol(location domain.PatternEvidenceLocation, symbols []symbolInfo) (symbolInfo, bool) {
	wanted := simpleSymbolName(location.Symbol)
	var match symbolInfo
	matched := false
	for _, symbol := range symbols {
		if symbol.Name != wanted || !compatibleSymbolKind(location.Kind, symbol.Kind) {
			continue
		}
		if !matched || location.Line > 0 && lineDistance(symbol.Line, location.Line) < lineDistance(match.Line, location.Line) {
			match = symbol
			matched = true
		}
	}
	return match, matched
}

func simpleSymbolName(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndex(value, "."); index >= 0 {
		value = value[index+1:]
	}
	if index := strings.Index(value, "("); index > 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func compatibleSymbolKind(requested, actual string) bool {
	requested = strings.ToLower(strings.TrimSpace(requested))
	actual = strings.ToLower(strings.TrimSpace(actual))
	if requested == "" || requested == actual {
		return true
	}
	if requested == "function" {
		return actual == "func"
	}
	if requested == "method" && actual == "func" {
		// 部分语法树把类成员方法与顶层函数统一标记为 func。
		return true
	}
	if requested != "type" {
		return false
	}
	switch actual {
	case "type", "class", "interface", "struct", "enum", "trait", "protocol", "module", "object":
		return true
	default:
		return false
	}
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

func lineDistance(left, right int) int {
	if left < right {
		return right - left
	}
	return left - right
}
