package domain

import "strings"

// DevelopmentFocus 是由证据焦点沉淀出的稳定开发导航信息。
// 它只用于生成 Skill 的需求路由，不替代模式本身的源码证据和适用边界。
type DevelopmentFocus struct {
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	RouteTerms []string `json:"route_terms,omitempty"`
	EntryPaths []string `json:"entry_paths,omitempty"`
}

// DevelopmentFocusFromEvidenceFocus 提取可以长期用于导航的最小焦点信息。
func DevelopmentFocusFromEvidenceFocus(focus EvidenceFocus) *DevelopmentFocus {
	id := strings.TrimSpace(focus.ID)
	name := strings.TrimSpace(focus.Name)
	if !focus.IsDevelopmentRouteable() || !validDevelopmentFocusID(id) || name == "" {
		return nil
	}
	return &DevelopmentFocus{
		ID:         id,
		Name:       name,
		RouteTerms: uniqueDevelopmentFocusValues(focus.RouteTerms),
		EntryPaths: uniqueDevelopmentFocusValues(focus.EntryPaths),
	}
}

func validDevelopmentFocusID(id string) bool {
	if id == "" || strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") {
		return false
	}
	for _, char := range id {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' {
			continue
		}
		return false
	}
	return true
}

// Clone 返回焦点的独立副本，避免模式归一化时共享切片或指针。
func (f *DevelopmentFocus) Clone() *DevelopmentFocus {
	if f == nil {
		return nil
	}
	return &DevelopmentFocus{
		ID:         strings.TrimSpace(f.ID),
		Name:       strings.TrimSpace(f.Name),
		RouteTerms: uniqueDevelopmentFocusValues(f.RouteTerms),
		EntryPaths: uniqueDevelopmentFocusValues(f.EntryPaths),
	}
}

func uniqueDevelopmentFocusValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
