package domain

// RuntimeState 保存开始学习或生成后不可再变更的初始化决策
type RuntimeState struct {
	Mode            string `json:"mode"`
	ModeLocked      bool   `json:"mode_locked"`
	Learned         bool   `json:"learned"`
	SkillsGenerated bool   `json:"skills_generated"`
	UpdatedAt       string `json:"updated_at"`
}

// LearnCurrentSummary 描述 learn current 的用户可读运行摘要。
type LearnCurrentSummary struct {
	ChangedFiles     int
	DeletedFiles     int
	SkippedFiles     int
	PatternsFound    int
	PatternsSaved    int
	PatternsRetired  int
	PatternsDropped  int
	DropReasons      []string
	Projects         int
	ChangedProjects  int
	WorkspaceChanged bool
	NoFileChanges    bool
	// AgentCallTotal 是本轮结构化 Agent 调用总次数（可选度量）。
	AgentCallTotal int
	// SkippedStages 是本轮短路跳过的阶段（如 review_empty）。
	SkippedStages []string
	// AnalysisMode 是 full、delta、mixed 或 none。
	AnalysisMode string
	// Resumed 表示从检查点恢复。
	Resumed bool
	// WallMs 是墙钟耗时毫秒。
	WallMs int64
}

// LearnCurrentResult 描述 learn current 的运行结果。
type LearnCurrentResult struct {
	Summary LearnCurrentSummary
}
