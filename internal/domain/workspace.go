package domain

// WorkspaceProject 描述工作区根目录下的一个子项目
type WorkspaceProject struct {
	ID             string   `json:"id" yaml:"id"`
	Path           string   `json:"path" yaml:"path"`
	Type           string   `json:"type" yaml:"type"`
	Language       string   `json:"language" yaml:"language"`
	Responsibility string   `json:"responsibility,omitempty" yaml:"responsibility,omitempty"`
	Frameworks     []string `json:"frameworks,omitempty" yaml:"frameworks,omitempty"`
}

// WorkspaceProfile 是持久化的工作区事实画像
type WorkspaceProfile struct {
	Name         string                `json:"name"`
	RootPath     string                `json:"root_path"`
	Summary      string                `json:"summary,omitempty"`
	Projects     []WorkspaceProject    `json:"projects"`
	Shared       []WorkspacePath       `json:"shared,omitempty"`
	Contracts    []WorkspacePath       `json:"contracts,omitempty"`
	Infra        []WorkspacePath       `json:"infra,omitempty"`
	Dependencies []WorkspaceDependency `json:"dependencies,omitempty"`
	ImpactRoutes []WorkspaceRoute      `json:"impact_routes,omitempty"`
	GeneratedAt  string                `json:"generated_at"`
}

// WorkspacePath 描述工作区中非子项目但有特殊职责的路径
type WorkspacePath struct {
	Path             string   `json:"path" yaml:"path"`
	Description      string   `json:"description,omitempty" yaml:"description,omitempty"`
	Consumers        []string `json:"consumers,omitempty" yaml:"consumers,omitempty"`
	Producers        []string `json:"producers,omitempty" yaml:"producers,omitempty"`
	AffectedProjects []string `json:"affected_projects,omitempty" yaml:"affected_projects,omitempty"`
}

// WorkspaceDependency 描述工作区子项目之间的依赖关系
type WorkspaceDependency struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// WorkspaceSpec 是用于渲染根 skills 的工作区级开发规范
type WorkspaceSpec struct {
	Name                   string                       `json:"name"`
	RootPath               string                       `json:"root_path"`
	Projects               []WorkspaceProject           `json:"projects"`
	Routing                []WorkspaceRoute             `json:"routing"`
	Rules                  []WorkspaceRule              `json:"rules"`
	ChangeOrder            []string                     `json:"change_order,omitempty"`
	ParallelAgentGuidance  []WorkspaceParallelGuidance  `json:"parallel_agent_guidance,omitempty"`
	LoadMultipleSkillsWhen []WorkspaceLoadMultipleSkill `json:"load_multiple_skills_when,omitempty"`
	GeneratedAt            string                       `json:"generated_at"`
}

// WorkspaceRoute 把路径映射到改动前应读取的子项目 skills
type WorkspaceRoute struct {
	PathPattern string   `json:"path_pattern"`
	ProjectIDs  []string `json:"project_ids"`
	Reason      string   `json:"reason"`
}

// WorkspaceRule 是一条跨项目规则
type WorkspaceRule struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	AppliesTo   []string `json:"applies_to,omitempty"`
}

// WorkspaceParallelGuidance 描述跨项目并发处理边界
type WorkspaceParallelGuidance struct {
	Scope     string `json:"scope"`
	Allowed   bool   `json:"allowed"`
	Condition string `json:"condition"`
}

// WorkspaceLoadMultipleSkill 描述需要同时读取多个子项目 skill 的场景
type WorkspaceLoadMultipleSkill struct {
	Condition  string   `json:"condition"`
	ProjectIDs []string `json:"project_ids"`
	Reason     string   `json:"reason"`
}
