package domain

// RuntimeState 保存开始学习或生成后不可再变更的初始化决策
type RuntimeState struct {
	Mode            string `json:"mode"`
	ModeLocked      bool   `json:"mode_locked"`
	Learned         bool   `json:"learned"`
	SkillsGenerated bool   `json:"skills_generated"`
	UpdatedAt       string `json:"updated_at"`
}
