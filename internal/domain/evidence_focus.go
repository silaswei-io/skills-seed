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
	RouteTerms    []string `json:"route_terms,omitempty"`
	Attributes    []string `json:"attributes,omitempty"`
	RiskSignals   []string `json:"risk_signals,omitempty"`
	AnalysisDepth string   `json:"analysis_depth,omitempty"`
	EntryPaths    []string `json:"entry_paths,omitempty"`
	RelatedPaths  []string `json:"related_paths,omitempty"`
	ScopeReason   string   `json:"scope_reason,omitempty"`
}

const (
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
