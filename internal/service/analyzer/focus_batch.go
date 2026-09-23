package analyzer

// FocusAnalysisMode 表示当前焦点分析使用的材料模式。
// full 与 delta 仍有不同结果形状（delta 产出 KnowledgeChange），但编排层应通过本枚举选择路径，
// 避免在多个调用点散落布尔分支，便于后续收敛为单一契约。
type FocusAnalysisMode string

const (
	// FocusAnalysisModeFull 使用完整源码样本与结构上下文分析。
	FocusAnalysisModeFull FocusAnalysisMode = "full"
	// FocusAnalysisModeDelta 使用 diff 锚定与相关既有知识分析。
	FocusAnalysisModeDelta FocusAnalysisMode = "delta"
)

// SelectFocusAnalysisMode 根据是否启用增量 diff 分析返回统一模式枚举。
func SelectFocusAnalysisMode(useDelta bool) FocusAnalysisMode {
	if useDelta {
		return FocusAnalysisModeDelta
	}
	return FocusAnalysisModeFull
}
