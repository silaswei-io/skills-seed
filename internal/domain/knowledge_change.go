package domain

// KnowledgeFocusAction 表示增量学习对会话证据焦点的判断。
type KnowledgeFocusAction string

const (
	// KnowledgeFocusExisting 表示变更属于已有证据焦点。
	KnowledgeFocusExisting KnowledgeFocusAction = "existing"
	// KnowledgeFocusExtend 表示变更扩展了已有证据焦点的路径或符号边界。
	KnowledgeFocusExtend KnowledgeFocusAction = "extend"
	// KnowledgeFocusNew 表示变更暴露了新的证据焦点。
	KnowledgeFocusNew KnowledgeFocusAction = "new"
	// KnowledgeFocusNoChange 表示变更没有改变证据焦点。
	KnowledgeFocusNoChange KnowledgeFocusAction = "no_change"
)

// KnowledgePatternAction 表示增量学习对模式库的判断。
type KnowledgePatternAction string

const (
	// KnowledgePatternAdd 表示新增一条模式候选。
	KnowledgePatternAdd KnowledgePatternAction = "add"
	// KnowledgePatternUpdate 表示候选用于更新已有模式。
	KnowledgePatternUpdate KnowledgePatternAction = "update"
	// KnowledgePatternReinforce 表示候选只增强已有模式证据。
	KnowledgePatternReinforce KnowledgePatternAction = "reinforce"
	// KnowledgePatternRetire 表示已有模式可能失效。
	KnowledgePatternRetire KnowledgePatternAction = "retire"
	// KnowledgePatternNoChange 表示本次变更没有可沉淀的模式变化。
	KnowledgePatternNoChange KnowledgePatternAction = "no_change"
)

// KnowledgeChange 是 learn current 增量分析的过程产物。
// 它描述 diff 对知识库造成的变化，后续再由协调层转换成模式入库候选。
type KnowledgeChange struct {
	FocusAction   KnowledgeFocusAction
	FocusID       string
	FocusName     string
	PatternAction KnowledgePatternAction
	PatternID     string
	Proposal      *Pattern
	Anchors       []PatternDiffAnchor
	Reason        string
}

// CarriesPattern 判断该知识变更是否携带可进入模式策展的候选。
func (c KnowledgeChange) CarriesPattern() bool {
	switch c.PatternAction {
	case KnowledgePatternAdd, KnowledgePatternUpdate, KnowledgePatternReinforce:
		return c.Proposal != nil
	default:
		return false
	}
}
