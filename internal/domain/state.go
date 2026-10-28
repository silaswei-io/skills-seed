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
	NavigationUpdated bool // 本地导航变化不表示业务语义已学习。
	PendingFiles      int  // 本地候选中尚未完成语义判断的文件数，不等于已确认存在的业务问题数。
	ChangedFiles      int
	DeletedFiles      int
	SkippedFiles      int
	PatternsFound     int
	PatternsSaved     int
	PatternsRetired   int
	PatternsDropped   int
	DropReasons       []string
	Projects          int
	ChangedProjects   int
	WorkspaceChanged  bool
	NoFileChanges     bool
}

// LearnCurrentResult 描述 learn current 的运行结果。
type LearnCurrentResult struct {
	Summary LearnCurrentSummary
}
