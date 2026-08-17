package generator

import (
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/domain"
)

type developmentFocusView struct {
	ID            string
	Title         string
	RouteTerms    []string
	PrimaryPath   string
	ReferencePath string
	PatternCount  int
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
				ID:            focus.ID,
				Title:         focus.Name,
				RouteTerms:    append([]string(nil), focus.RouteTerms...),
				PrimaryPath:   firstDevelopmentFocusPath(focus, pattern),
				ReferencePath: developmentFocusReferencePath(focus.ID, pattern.Category),
			}
			byID[focus.ID] = view
		}
		view.PatternCount++
		view.RouteTerms = mergeDevelopmentFocusTerms(view.RouteTerms, focus.RouteTerms)
		if view.PrimaryPath == "" {
			view.PrimaryPath = firstDevelopmentFocusPath(focus, pattern)
		}
		if pattern.Category == domain.CategoryBusiness {
			view.ReferencePath = developmentFocusReferencePath(focus.ID, pattern.Category)
		}
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

func firstDevelopmentFocusPath(focus *domain.DevelopmentFocus, pattern domain.Pattern) string {
	if focus != nil && len(focus.EntryPaths) > 0 {
		return focus.EntryPaths[0]
	}
	if len(pattern.EvidenceLocations) > 0 {
		return pattern.EvidenceLocations[0].DisplayLocation()
	}
	return ""
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
