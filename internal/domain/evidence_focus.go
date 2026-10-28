package domain

// LearningAgenda 是本轮学习的临时议程。
// 它只用于组织独立批次调用要查看的证据，不属于最终模式库分类。
type LearningAgenda struct {
	Focuses []EvidenceFocus `json:"focuses,omitempty"`
}

// EvidenceFocus 表示一次独立批次调用中需要共同查看的一组证据。
type EvidenceFocus struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Purpose       string   `json:"purpose,omitempty"`
	RouteTerms    []string `json:"route_terms,omitempty"`
	Attributes    []string `json:"attributes,omitempty"`
	RiskSignals   []string `json:"risk_signals,omitempty"`
	AnalysisDepth string   `json:"analysis_depth,omitempty"`
	// EntryPaths 是本焦点独占的学习责任，只有这些路径计入议程覆盖。
	EntryPaths []string `json:"entry_paths,omitempty"`
	// RelatedPaths 是可跨焦点共享的只读证据，不承担学习覆盖责任。
	RelatedPaths []string `json:"related_paths,omitempty"`
	ScopeReason  string   `json:"scope_reason,omitempty"`
}

const (
	// EvidenceFocusPurposeDevelopment 兼容旧学习状态中的开发批次标记。
	EvidenceFocusPurposeDevelopment = "development"
	// EvidenceFocusPurposeCoverage 表示仅为覆盖本轮输入而建立的临时焦点。
	EvidenceFocusPurposeCoverage = "coverage"

	EvidenceFocusDepthStandard = "standard"
	EvidenceFocusDepthCareful  = "careful"
	EvidenceFocusDepthCritical = "critical"
)

// EffectiveAnalysisDepth 返回焦点的有效分析深度；旧状态缺省为 standard。
func (f EvidenceFocus) EffectiveAnalysisDepth() string {
	switch f.AnalysisDepth {
	case EvidenceFocusDepthCareful, EvidenceFocusDepthCritical:
		return f.AnalysisDepth
	default:
		return EvidenceFocusDepthStandard
	}
}
