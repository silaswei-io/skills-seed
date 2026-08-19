package generator

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type developmentFocusView struct {
	ID             string
	Title          string
	RouteTerms     []string
	EntryPaths     []string
	ReferencePaths []string
	ScopeReason    string
	Attributes     []string
	RiskSignals    []string
	PatternCount   int
}

func buildDevelopmentFocuses(patterns []domain.Pattern) []developmentFocusView {
	byID := make(map[string]*developmentFocusView)
	for _, pattern := range patterns {
		focus := pattern.DevelopmentFocus.Clone()
		if focus == nil {
			continue
		}
		view, ok := byID[focus.ID]
		if !ok {
			view = &developmentFocusView{
				ID:         focus.ID,
				Title:      focus.Name,
				RouteTerms: append([]string(nil), focus.RouteTerms...),
			}
			byID[focus.ID] = view
		}
		view.PatternCount++
		view.RouteTerms = mergeDevelopmentFocusTerms(view.RouteTerms, focus.RouteTerms)
		view.EntryPaths = mergeDevelopmentFocusPaths(view.EntryPaths, focus.EntryPaths)
		view.EntryPaths = mergeDevelopmentFocusPaths(view.EntryPaths, patternEvidencePaths(pattern))
		view.ReferencePaths = mergeDevelopmentFocusPaths(view.ReferencePaths, []string{developmentFocusReferencePath(focus.ID, pattern.Category)})
		if view.ScopeReason == "" {
			view.ScopeReason = focus.ScopeReason
		}
		view.Attributes = mergeDevelopmentFocusTerms(view.Attributes, focus.Attributes)
		view.RiskSignals = mergeDevelopmentFocusTerms(view.RiskSignals, focus.RiskSignals)
	}

	result := make([]developmentFocusView, 0, len(byID))
	for _, view := range byID {
		if len(view.RouteTerms) == 0 {
			view.RouteTerms = []string{view.Title}
		}
		result = append(result, *view)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].PatternCount != result[j].PatternCount {
			return result[i].PatternCount > result[j].PatternCount
		}
		return result[i].Title < result[j].Title
	})
	return result
}

func developmentFocusReferencePath(focusID string, category domain.Category) string {
	if category == domain.CategoryBusiness {
		return "./references/patterns/business/" + focusID + ".md"
	}
	return "./references/patterns/" + string(category) + ".md"
}

func patternEvidencePaths(pattern domain.Pattern) []string {
	paths := make([]string, 0, len(pattern.EvidenceLocations))
	for _, evidence := range pattern.EvidenceLocations {
		if path := strings.TrimSpace(evidence.DisplayLocation()); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func mergeDevelopmentFocusTerms(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	result := make([]string, 0, len(left)+len(right))
	for _, terms := range [][]string{left, right} {
		for _, term := range terms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			if _, exists := seen[term]; exists {
				continue
			}
			seen[term] = struct{}{}
			result = append(result, term)
		}
	}
	return result
}

func mergeDevelopmentFocusPaths(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	result := make([]string, 0, len(left)+len(right))
	for _, paths := range [][]string{left, right} {
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			result = append(result, path)
		}
	}
	sort.Strings(result)
	return result
}
