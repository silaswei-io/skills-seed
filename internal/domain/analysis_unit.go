package domain

// AnalysisUnit 表示 learn current 中可独立完成的业务分析单元。
type AnalysisUnit struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	RouteTerms   []string `json:"route_terms,omitempty"`
	EntryPaths   []string `json:"entry_paths,omitempty"`
	RelatedPaths []string `json:"related_paths,omitempty"`
	ScopeReason  string   `json:"scope_reason,omitempty"`
}
