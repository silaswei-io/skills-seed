package domain

// LearningEvidence 保存程序提供的源码和结构事实，不包含分析会话或模型推理。
// 分析与审查共享同一份材料，源码路径相对项目根目录，diff 路径指向运行产物。
type LearningEvidence struct {
	SourcePaths       []string `json:"source_paths,omitempty"`
	DiffPaths         []string `json:"diff_paths,omitempty"`
	StructuralContext string   `json:"structural_context,omitempty"`
}

// Clone 返回可独立持有的事实材料。
func (e LearningEvidence) Clone() LearningEvidence {
	e.SourcePaths = append([]string(nil), e.SourcePaths...)
	e.DiffPaths = append([]string(nil), e.DiffPaths...)
	return e
}
