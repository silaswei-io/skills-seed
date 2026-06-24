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
	Projects         int
	ChangedProjects  int
	WorkspaceChanged bool
	NoFileChanges    bool
}

// LearnCurrentResult 描述 learn current 的运行结果。
type LearnCurrentResult struct {
	Summary LearnCurrentSummary
}
