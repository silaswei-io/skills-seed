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
	ScopeReason   string
	Attributes    []string
	RiskSignals   []string
	PatternCount  int
}

func buildDevelopmentFocuses(patterns []domain.Pattern) []developmentFocusView {
	byID := make(map[string]*developmentFocusView)
	for _, pattern := range patterns {
		// 优先使用已审查焦点；缺失时从证据路径派生导航入口，避免生成侧丢失可路由信息。
		focus := pattern.DevelopmentFocus.Clone()
		derived := false
		if focus == nil {
			focus = deriveDevelopmentFocusFromPattern(pattern)
			derived = focus != nil
		}
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
				ReferencePath: developmentFocusReferencePath(focus.ID, pattern.Category, derived),
			}
			byID[focus.ID] = view
		}
		view.PatternCount++
		view.RouteTerms = mergeDevelopmentFocusTerms(view.RouteTerms, focus.RouteTerms)
		if view.PrimaryPath == "" {
			view.PrimaryPath = firstDevelopmentFocusPath(focus, pattern)
		}
		// 仅已审查业务焦点可指向 per-focus 业务页；派生焦点只挂分类页，避免死链。
		if !derived && pattern.Category == domain.CategoryBusiness {
			view.ReferencePath = developmentFocusReferencePath(focus.ID, pattern.Category, false)
		}
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

func developmentFocusReferencePath(focusID string, category domain.Category, derived bool) string {
	// 派生焦点没有对应的独立业务 reference 文件，只能落到分类页。
	if !derived && category == domain.CategoryBusiness && strings.TrimSpace(focusID) != "" {
		return "./references/patterns/business/" + focusID + ".md"
	}
	return "./references/patterns/" + string(category) + ".md"
}

func firstDevelopmentFocusPath(focus *domain.DevelopmentFocus, pattern domain.Pattern) string {
	if focus != nil {
		for _, path := range focus.EntryPaths {
			if path = strings.TrimSpace(path); path != "" {
				return path
			}
		}
	}
	for _, evidence := range pattern.EvidenceLocations {
		if path := strings.TrimSpace(evidence.Path); path != "" {
			return path
		}
		if path := strings.TrimSpace(evidence.DisplayLocation()); path != "" {
			return path
		}
	}
	return ""
}

// deriveDevelopmentFocusFromPattern 在缺少已审查焦点时，用模式身份与证据路径派生导航入口。
func deriveDevelopmentFocusFromPattern(pattern domain.Pattern) *domain.DevelopmentFocus {
	id := strings.TrimSpace(pattern.ID)
	name := strings.TrimSpace(pattern.Name)
	if id == "" && name == "" {
		return nil
	}
	if id == "" {
		id = slugDevelopmentFocusID(name)
	}
	if name == "" {
		name = id
	}
	entryPaths := make([]string, 0, len(pattern.EvidenceLocations))
	seen := make(map[string]struct{}, len(pattern.EvidenceLocations))
	for _, evidence := range pattern.EvidenceLocations {
		path := strings.TrimSpace(evidence.Path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		entryPaths = append(entryPaths, path)
	}
	if len(entryPaths) == 0 {
		return nil
	}
	return &domain.DevelopmentFocus{
		ID:         id,
		Name:       name,
		EntryPaths: entryPaths,
		RouteTerms: []string{name},
	}
}

func slugDevelopmentFocusID(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "pattern"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "pattern"
	}
	return out
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
