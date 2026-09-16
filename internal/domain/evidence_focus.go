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
	// EvidenceFocusPurposeDevelopment 表示可沉淀为长期开发导航的证据焦点。
	EvidenceFocusPurposeDevelopment = "development"
	// EvidenceFocusPurposeCoverage 表示仅为覆盖本轮输入而建立的临时焦点。
	EvidenceFocusPurposeCoverage = "coverage"

	EvidenceFocusDepthStandard = "standard"
	EvidenceFocusDepthCareful  = "careful"
	EvidenceFocusDepthCritical = "critical"
)

// IsDevelopmentRouteable 判断焦点是否可作为未来任务的长期导航入口。
// 未标记的旧状态保持可路由，兼容历史运行检查点。
func (f EvidenceFocus) IsDevelopmentRouteable() bool {
	return f.Purpose != EvidenceFocusPurposeCoverage
}

// EffectiveAnalysisDepth 返回焦点的有效分析深度；旧状态缺省为 standard。
func (f EvidenceFocus) EffectiveAnalysisDepth() string {
	switch f.AnalysisDepth {
	case EvidenceFocusDepthCareful, EvidenceFocusDepthCritical:
		return f.AnalysisDepth
	default:
		return EvidenceFocusDepthStandard
	}
}
