package domain

// RuntimeState 保存开始学习或生成后不可再变更的初始化决策
type RuntimeState struct {
	Mode            string           `json:"mode"`
	ModeLocked      bool             `json:"mode_locked"`
	Learned         bool             `json:"learned"`
	SkillsGenerated bool             `json:"skills_generated"`
	SkillsDirty     SkillsDirtyState `json:"skills_dirty,omitempty"`
	UpdatedAt       string           `json:"updated_at"`
}

// SkillsDirtyState 记录需要重新生成 skills 的目标。
type SkillsDirtyState struct {
	Project   bool     `json:"project,omitempty"`
	Workspace bool     `json:"workspace,omitempty"`
	Projects  []string `json:"projects,omitempty"`
}

// SkillsDirtyTarget 描述本次需要标记或清理的 skills 目标。
type SkillsDirtyTarget struct {
	Project   bool
	Workspace bool
	Projects  []string
}

// LearnCurrentSummary 描述 learn current 的用户可读运行摘要。
type LearnCurrentSummary struct {
	ChangedFiles     int
	DeletedFiles     int
	SkippedFiles     int
	PatternsFound    int
	PatternsSaved    int
	Projects         int
	DirtyProjects    int
	WorkspaceChanged bool
	NoFileChanges    bool
}

// LearnCurrentResult 描述 learn current 本轮是否产生需要重新生成 skills 的变化。
type LearnCurrentResult struct {
	SkillsDirty SkillsDirtyTarget
	Summary     LearnCurrentSummary
}
