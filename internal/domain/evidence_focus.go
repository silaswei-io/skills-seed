package domain

// LearningAgenda 是一次学习会话的临时议程。
// 它只用于组织本轮对话要查看的证据，不属于最终模式库分类。
type LearningAgenda struct {
	Focuses []EvidenceFocus `json:"focuses,omitempty"`
}

// EvidenceFocus 表示一次学习会话中需要共同查看的一组证据。
type EvidenceFocus struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	RouteTerms   []string `json:"route_terms,omitempty"`
	EntryPaths   []string `json:"entry_paths,omitempty"`
	RelatedPaths []string `json:"related_paths,omitempty"`
	ScopeReason  string   `json:"scope_reason,omitempty"`
}
